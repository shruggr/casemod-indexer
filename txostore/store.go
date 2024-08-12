package txostore

import (
	"bytes"
	"context"
	"encoding/hex"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/util"
	"github.com/redis/go-redis/v9"
	"github.com/shruggr/casemod-indexer/db"
	"github.com/shruggr/casemod-indexer/types"
	"github.com/vmihailenco/msgpack/v5"
)

type Store struct {
	Indexers   []types.Indexer
	indexerMap map[string]types.Indexer
	tags       []string
}

func (s *Store) IndexerMap() map[string]types.Indexer {
	if s.indexerMap == nil {
		s.indexerMap = make(map[string]types.Indexer, len(s.Indexers))
		for _, indexer := range s.Indexers {
			s.indexerMap[indexer.Tag()] = indexer
		}
	}
	return s.indexerMap
}

func (s *Store) Tags() []string {
	if s.tags == nil {
		s.tags = make([]string, len(s.Indexers))
		for i, indexer := range s.Indexers {
			s.tags[i] = indexer.Tag()
		}
	}
	return s.tags
}

type LoadTxoParams struct {
	Block  bool     `json:"block"`
	Spend  bool     `json:"spend"`
	Deps   bool     `json:"deps"`
	Events bool     `json:"events"`
	Obj    bool     `json:"data"`
	Tags   []string `json:"tags"`
}

func (l *LoadTxoParams) keys() []string {
	keys := make([]string, 0, 2+len(l.Tags)*3)
	keys = append(keys, string(db.OutputMember))
	if l.Spend {
		keys = append(keys, string(db.SpendMember))
	}
	for _, tag := range l.Tags {
		if l.Deps {
			keys = append(keys, db.DepMember(tag))
		}
		if l.Events {
			keys = append(keys, db.EventMember(tag))
		}
		if l.Obj {
			keys = append(keys, db.DataMember(tag))
		}
	}
	return keys
}

type SearchTxoParams struct {
	Tag    string         `json:"tag"`
	Id     string         `json:"id"`
	Value  string         `json:"value"`
	Owner  *types.PKHash  `json:"owner"`
	Spent  bool           `json:"spent"`
	Limit  uint32         `json:"limit"`
	Offset uint32         `json:"cursor"`
	Fields *LoadTxoParams `json:"fields"`
}

type TxoSearchResult struct {
	Txos   []*types.Txo `json:"txos"`
	Cursor uint64       `json:"cursor"`
}

func (s *Store) LoadTxosByTxid(ctx context.Context, txid string, req *LoadTxoParams) ([]*types.Txo, error) {
	iter := db.Txos.Scan(ctx, 0, db.TxoTxidKey(txid), 100).Iterator()
	txos := make([]*types.Txo, 0, 10)
	for iter.Next(ctx) {
		if err := iter.Err(); err != nil {
			return nil, err
		} else if outpoint, err := types.NewOutpointFromString(iter.Val()[4:]); err != nil {
			return nil, err
		} else if txo, err := s.LoadTxo(ctx, outpoint, req); err != nil {
			return nil, err
		} else {
			txos = append(txos, txo)
		}
	}
	return txos, nil
}

func (s *Store) LoadTxo(ctx context.Context, outpoint *types.Outpoint, params *LoadTxoParams) (txo *types.Txo, err error) {
	txo = &types.Txo{
		Outpoint: outpoint,
		Data:     make(map[string]*types.IndexData),
	}

	if params == nil || params.Block {
		txo.Block = db.LoadTxBlock(ctx, outpoint.Txid.String())
	}

	txoMap := make(map[string][]byte)
	if params == nil {
		if err := db.Txos.HGetAll(ctx, db.TxoKey(outpoint)).Scan(&txoMap); err == redis.Nil || len(txoMap) == 0 {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
	} else {
		keys := params.keys()
		if len(keys) == 0 {
			return txo, nil
		} else if err := db.Txos.HMGet(ctx, db.TxoKey(outpoint), keys...).Scan(&txoMap); err == redis.Nil || len(txoMap) == 0 {
			return txo, nil
		} else if err != nil {
			return nil, err
		}
	}

	if output := txoMap[db.OutputMember]; len(output) == 0 {
		return nil, nil
	} else {
		txo.Output = types.NewOutputFromBytes(output)
	}
	for member, data := range txoMap {
		switch member {
		case string(db.OutputMember):
		case string(db.SpendMember):
			if err := msgpack.Unmarshal(data, &txo.Spend); err != nil {
				log.Panic(err)
			}
		default:
			parts := strings.Split(member, ":")
			tag := parts[0]
			idxData := txo.Data[tag]
			if idxData == nil {
				idxData = &types.IndexData{}
				txo.Data[tag] = idxData
			}
			switch parts[1] {
			case db.DepSuffix:
				if err := msgpack.Unmarshal(data, &idxData.Deps); err != nil {
					log.Panic(err)
				}
			case db.EventSuffix:
				if err := msgpack.Unmarshal(data, &idxData.Events); err != nil {
					log.Panic(err)
				}
			case db.DataSuffix:
				idxData.Data = data
				indexer := s.IndexerMap()[tag]
				if indexer != nil {
					if idxData.Obj, err = indexer.UnmarshalData(data); err != nil {
						log.Panic(err)
					}
				}
			}
		}
	}

	return txo, nil
}

func (s *Store) SearchTxos(ctx context.Context, params SearchTxoParams) ([]*types.Txo, error) {
	var key string
	if params.Owner == nil {
		key = db.EventKey(params.Tag, &types.EventLog{
			Label: params.Id,
			Value: params.Value,
		})
	} else {
		key = db.OwnerEventKey(params.Owner.String(), params.Tag, &types.EventLog{
			Label: params.Id,
			Value: params.Value,
		})
	}
	start := 0.0
	stop := 1.0
	if params.Spent {
		start = 1
		stop = 2
	}

	if outpoints, err := db.Txos.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     key,
		ByScore: true,
		Start:   start,
		Stop:    stop,
		Count:   int64(params.Limit),
		Offset:  int64(params.Offset),
	}).Result(); err != nil {
		return nil, err
	} else {
		txos := make([]*types.Txo, 0, len(outpoints))
		for _, op := range outpoints {
			if outpoint, err := types.NewOutpointFromString(op); err != nil {
				log.Panicf("Invalid outpoint %s: %s", outpoint, err)
			} else if txo, err := s.LoadTxo(ctx, outpoint, params.Fields); err != nil {
				return nil, err
			} else {
				txos = append(txos, txo)
			}
		}
		return txos, nil
	}
}

func NewIndexContext(ctx context.Context, tx *transaction.Transaction) *types.IndexContext {
	txid := tx.TxIDBytes()

	log.Println("NewIndexContext", hex.EncodeToString(txid))
	block := &types.Block{
		Height: uint32(time.Now().Unix()),
	}
	if tx.MerklePath != nil {
		block.Height = tx.MerklePath.BlockHeight
		// TODO: populate block hash
		idx := slices.IndexFunc(tx.MerklePath.Path[0], func(pe *transaction.PathElement) bool {
			return bytes.Equal(pe.Hash, util.ReverseBytes(txid))
		})
		if idx >= 0 {
			block.Idx = tx.MerklePath.Path[0][idx].Offset
		}
	}

	return &types.IndexContext{
		Rawtx: tx.Bytes(),
		Tx:    tx,
		Txid:  txid,
		Block: block,
		Txos:  make([]*types.Txo, 0, len(tx.Outputs)),
	}
}

func (s *Store) Ingest(ctx context.Context, tx *transaction.Transaction) (idxCtx *types.IndexContext, err error) {
	idxCtx = NewIndexContext(ctx, tx)
	if err = s.PopulateInputs(ctx, idxCtx); err != nil {
		log.Println("PopulateInputs", err)
		return nil, err
	} else if err = s.ParseOutputs(ctx, idxCtx); err != nil {
		log.Println("ParseOutputs", err)
		return nil, err
	}

	if _, err = db.Txos.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		if err := s.PersistSpends(ctx, idxCtx, pipe); err != nil {
			log.Println("PersistSpends", err)
			return err
		} else if err := s.PersistTxos(ctx, idxCtx, pipe); err != nil {
			log.Println("PersistTxos", err)
			return err
		}

		return nil
	}); err != nil {
		log.Println("Pipelined", err)
		return nil, err
	}
	return idxCtx, nil
}

func (s *Store) PopulateInputs(ctx context.Context, idxCtx *types.IndexContext) (err error) {
	if !idxCtx.Tx.IsCoinbase() {
		for vin, input := range idxCtx.Tx.Inputs {
			if input.SourceTransaction == nil {
				if input.SourceTransaction, err = db.LoadTx(ctx, hex.EncodeToString(input.SourceTXID)); err != nil {
					return err
				}
			} else if _, err = s.Ingest(ctx, input.SourceTransaction); err != nil {
				return err
			}

			outpoint := &types.Outpoint{
				Txid: input.SourceTXID,
				Vout: input.SourceTxOutIndex,
			}
			var spend *types.Txo
			if spend, err = s.LoadTxo(ctx, outpoint, &LoadTxoParams{
				Block:  true,
				Events: true,
				Obj:    true,
				Tags:   s.Tags(),
			}); err != nil {
				return err
			} else if spend == nil {
				spend = &types.Txo{
					Outpoint: outpoint,
					Output: &types.Output{
						Satoshis: *input.SourceTxSatoshis(),
						Script:   *input.SourceTxScript(),
					},
				}
			}
			spend.Spend = &types.Spend{
				Txid:  idxCtx.Txid,
				Vin:   uint32(vin),
				Block: idxCtx.Block,
			}
			idxCtx.Spends = append(idxCtx.Spends, spend)
		}
	}
	return nil
}

func (s *Store) ParseOutputs(ctx context.Context, idxCtx *types.IndexContext) (err error) {
	for vout, output := range idxCtx.Tx.Outputs {
		var txo *types.Txo
		outpoint := &types.Outpoint{
			Txid: idxCtx.Txid,
			Vout: uint32(vout),
		}
		if txo, err = s.LoadTxo(ctx, outpoint, &LoadTxoParams{
			Spend: true,
		}); err != nil {
			return err
		}
		txo.Output = &types.Output{
			Satoshis: output.Satoshis,
			Script:   *output.LockingScript,
		}
		txo.Block = idxCtx.Block
		txo.Owner, _ = types.NewPKHashFromScript(txo.Output.Script)
		idxCtx.Txos = append(idxCtx.Txos, txo)
		for _, indexer := range s.Indexers {
			if data := indexer.Parse(idxCtx, uint32(vout)); data != nil {
				txo.Data[indexer.Tag()] = data
			}
		}
	}
	for _, indexer := range s.Indexers {
		indexer.Save(idxCtx)
	}
	return nil
}

func (s *Store) PersistSpends(ctx context.Context, idxCtx *types.IndexContext, pipe redis.Cmdable) (err error) {
	for _, spend := range idxCtx.Spends {
		if s, err := msgpack.Marshal(spend.Spend); err != nil {
			log.Println(spend.Outpoint.String(), err)
			return err
		} else if err := pipe.HSet(ctx, db.TxoKey(spend.Outpoint), db.SpendMember, s).Err(); err != nil {
			log.Println(spend.Outpoint.String(), err)
			return err
		}

		for tag, data := range spend.Data {
			indexer := s.IndexerMap()[tag]
			if indexer == nil {
				continue
			}

			score := indexer.Score(spend)
			for _, e := range data.Events {
				member := spend.Outpoint.String()
				pipe.ZAdd(ctx,
					db.TxoEventKey(tag, e),
					redis.Z{
						Score:  score,
						Member: member,
					},
				)
				if spend.Owner != nil {
					pipe.ZAdd(ctx,
						db.OwnerKey(spend.Owner),
						redis.Z{
							Score:  score,
							Member: member,
						},
					)
					pipe.ZAdd(ctx,
						db.TxoOwnerKey(spend.Owner, tag, e),
						redis.Z{
							Score:  score,
							Member: member,
						},
					)
				}
			}
		}
	}
	return nil
}

func (s *Store) PersistTxos(ctx context.Context, idxCtx *types.IndexContext, pipe redis.Pipeliner) (err error) {
	for _, txo := range idxCtx.Txos {
		txoData := make(map[string]interface{}, 10)
		txoData[db.OutputMember] = txo.Output.Bytes()
		for _, indexer := range s.Indexers {
			tag := indexer.Tag()
			idxData := txo.Data[tag]
			if idxData == nil {
				continue
			}
			if len(idxData.Deps) > 0 {
				txoData[db.DepMember(tag)] = idxData.Deps
			}

			if idxData.Obj != nil {
				txoData[db.DataMember(tag)] = idxData.Obj
			}

			if len(idxData.Events) > 0 {
				txoData[db.EventMember(tag)] = idxData.Events
				score := indexer.Score(txo)
				for _, e := range idxData.Events {
					member := txo.Outpoint.String()
					pipe.ZAdd(ctx,
						db.TxoEventKey(tag, e),
						redis.Z{
							Score:  score,
							Member: member,
						},
					)
					if txo.Owner != nil {
						pipe.ZAdd(ctx,
							db.OwnerKey(txo.Owner),
							redis.Z{
								Score:  score,
								Member: member,
							},
						)
						pipe.ZAdd(ctx,
							db.TxoOwnerKey(txo.Owner, tag, e),
							redis.Z{
								Score:  score,
								Member: member,
							},
						)
					}
				}
			}
		}
		if len(txo.Data) == 0 {
			continue
		}
		txoMap := make(map[string]interface{}, len(txoData))
		for k, v := range txoData {
			switch k {
			case db.OutputMember:
				txoMap[k] = v
			default:
				if data, err := msgpack.Marshal(v); err != nil {
					log.Panicln(txo.Outpoint.String(), err)
				} else {
					txoMap[k] = data
				}
			}
		}

		if err = pipe.HMSet(ctx, db.TxoKey(txo.Outpoint), txoMap).Err(); err != nil {
			log.Println(txo.Outpoint.String(), err)
			return err
		}
	}
	return nil
}
