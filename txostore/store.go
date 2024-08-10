package store

import (
	"bytes"
	"context"
	"encoding/hex"
	"log"
	"slices"
	"time"

	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/util"
	"github.com/redis/go-redis/v9"
	"github.com/shruggr/casemod-indexer/db"
	"github.com/shruggr/casemod-indexer/types"
	"github.com/vmihailenco/msgpack/v5"
)

type Store struct {
	Indexers []types.Indexer
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

			spend := &types.Txo{
				Outpoint: &types.Outpoint{
					Txid: input.SourceTXID,
					Vout: input.SourceTxOutIndex,
				},
			}
			if t, err := db.Rdb.HGet(ctx, "txo:"+spend.Outpoint.String(), "txo").Bytes(); err == redis.Nil {
				spend.Satoshis = *input.SourceTxSatoshis()
				spend.Script = *input.SourceTxScript()
			} else if err != nil {
				return err
			} else if err = msgpack.Unmarshal(t, spend); err != nil {
				return err
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
		txo := &types.Txo{
			Outpoint: &types.Outpoint{
				Txid: idxCtx.Txid,
				Vout: uint32(vout),
			},
			Data: make(map[string]*types.IndexData),
		}
		if t, err := db.Rdb.HGet(ctx, "txo:"+txo.Outpoint.String(), "txo").Bytes(); err == redis.Nil {
			txo.Satoshis = output.Satoshis
			txo.Script = *output.LockingScript
		} else if err != nil {
			return err
		} else if err = msgpack.Unmarshal(t, txo); err != nil {
			return err
		}
		// txo := (&types.Txo{}).FromRawTxo(rawTxo, s.Indexers)

		if txo.Owner != nil {
			if owner, err := types.NewPKHashFromScript(txo.Script); err == nil {
				txo.Owner = owner
			}
		}
		txo.Block = idxCtx.Block
		idxCtx.Txos = append(idxCtx.Txos, txo)
		for _, indexer := range s.Indexers {
			data := indexer.Parse(idxCtx, uint32(vout))
			if data != nil {
				txo.Data[indexer.Tag()] = data
			}
		}
	}
	for _, indexer := range s.Indexers {
		indexer.Save(idxCtx)
	}
	return nil
}

func NewIndexContext(ctx context.Context, tx *transaction.Transaction) *types.IndexContext {
	txid := tx.TxIDBytes()

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
		return nil, err
	} else if err = s.ParseOutputs(ctx, idxCtx); err != nil {
		return nil, err
	}
	if _, err = db.Rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		if err := s.PersistSpends(ctx, idxCtx, pipe); err != nil {
			return err
		} else if err := s.PersistTxos(ctx, idxCtx, pipe); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}
	return idxCtx, nil
}

func (s *Store) PersistSpends(ctx context.Context, idxCtx *types.IndexContext, pipe redis.Cmdable) (err error) {
	for _, spend := range idxCtx.Spends {
		if s, err := msgpack.Marshal(spend.Spend); err != nil {
			log.Println(spend.Outpoint.String(), err)
			return err
		} else if err := pipe.HSet(ctx, spend.Key(), db.SpendKey, s).Err(); err != nil {
			log.Println(spend.Outpoint.String(), err)
			return err
		}

		for _, indexer := range s.Indexers {
			tag := indexer.Tag()
			data := spend.Data[tag]
			if data == nil || len(data.Events) == 0 {
				continue
			}

			score := indexer.Score(idxCtx, spend.Outpoint.Vout)
			for _, e := range data.Events {
				member := spend.EventMember(e)
				pipe.ZAdd(ctx,
					spend.EventKey(tag, e),
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
						spend.OwnerKey(tag, e),
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
		for _, indexer := range s.Indexers {
			tag := indexer.Tag()
			data := txo.Data[tag]
			if data == nil {
				continue
			}
			if len(data.Deps) > 0 {
				deps := make([]byte, 0, 36*len(data.Deps))
				for _, dep := range data.Deps {
					deps = append(deps, dep.Bytes()...)
				}
				pipe.HSet(ctx, txo.Key(), db.DepKey(tag), deps)
			}

			if data.Item != nil {
				if data.Data, err = msgpack.Marshal(data.Item); err != nil {
					panic(err)
				}
			}

			if len(data.Events) == 0 {
				continue
			}

			score := indexer.Score(idxCtx, txo.Outpoint.Vout)
			for _, e := range data.Events {
				member := txo.EventMember(e)
				pipe.ZAdd(ctx,
					txo.EventKey(tag, e),
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
						txo.OwnerKey(tag, e),
						redis.Z{
							Score:  score,
							Member: member,
						},
					)
				}
			}
		}
		if t, err := msgpack.Marshal(txo); err != nil {
			log.Println(txo.Outpoint.String(), err)
			return err
		} else if err = pipe.HSet(ctx, txo.Key(), db.TxoKey, t).Err(); err != nil {
			log.Println(txo.Outpoint.String(), err)
			return err
		}
	}
	return nil
}
