package store

import (
	"bytes"
	"context"
	"encoding/hex"
	"math"
	"slices"
	"time"

	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/util"
	"github.com/redis/go-redis/v9"
	"github.com/shruggr/casemod-indexer/db"
	"github.com/shruggr/casemod-indexer/lib"
	"github.com/shruggr/casemod-indexer/types"
	"google.golang.org/protobuf/proto"
)

type Store struct {
	Indexers []types.Indexer
}

func (s *Store) Parse(ctx context.Context, tx *transaction.Transaction) (*types.IndexContext, error) {
	var err error
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

	idxCtx := &types.IndexContext{
		Rawtx: tx.Bytes(),
		Tx:    tx,
		Txid:  txid,
		Block: block,
		Txos:  make([]*types.Txo, 0, len(tx.Outputs)),
	}

	if !tx.IsCoinbase() {
		for vin, input := range tx.Inputs {
			if input.SourceTransaction == nil {
				if input.SourceTransaction, err = db.LoadTx(ctx, hex.EncodeToString(input.SourceTXID)); err != nil {
					return nil, err
				}
			} else if _, err = s.Ingest(ctx, input.SourceTransaction); err != nil {
				return nil, err
			}

			outpoint := &types.Outpoint{
				Txid: input.SourceTXID,
				Vout: input.SourceTxOutIndex,
			}
			rawTxo := &types.RawTxo{}
			if t, err := db.Rdb.HGet(ctx, "to:"+outpoint.JsonString(), "txo").Bytes(); err == redis.Nil {
				rawTxo.Outpoint = outpoint
				rawTxo.Satoshis = *input.PreviousTxSatoshis()
				rawTxo.Script = *input.PreviousTxScript()
			} else if err != nil {
				return nil, err
			} else if err = proto.Unmarshal(t, rawTxo); err != nil {
				return nil, err
			}
			spend := (&types.Txo{}).FromRawTxo(rawTxo, s.Indexers)
			spend.Spend = &types.Spend{
				Txid:  txid,
				Vin:   uint32(vin),
				Block: block,
			}
			idxCtx.Spends = append(idxCtx.Spends, spend)
		}
	}

	for vout, output := range tx.Outputs {
		outpoint := &types.Outpoint{
			Txid: txid,
			Vout: uint32(vout),
		}
		// txo := &types.Txo{
		// 	Data: make(map[string]*types.IndexData),
		// }
		rawTxo := &types.RawTxo{}
		if t, err := db.Rdb.HGet(ctx, "to:"+outpoint.JsonString(), "txo").Bytes(); err == redis.Nil {
			rawTxo.Outpoint = outpoint
			rawTxo.Satoshis = output.Satoshis
			rawTxo.Script = *output.LockingScript
		} else if err != nil {
			return nil, err
		} else if err = proto.Unmarshal(t, rawTxo); err != nil {
			return nil, err
		}
		txo := (&types.Txo{}).FromRawTxo(rawTxo, s.Indexers)

		if txo.Owner != "" {
			if owner, err := lib.NewPKHashFromScript(txo.Script); err == nil {
				txo.Owner, _ = owner.Address()
			}
		}
		txo.Block = block
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

	for _, txo := range idxCtx.Txos {
		txo.RawData = make(map[string]*types.RawData)
		for tag, data := range txo.Data {
			if data.Item == nil {
				continue
			}
			if data.RawData.Data, err = proto.Marshal(data.Item); err != nil {
				panic(err)
			}
			txo.RawData[tag] = &data.RawData
		}
	}

	return idxCtx, nil
}

func (s *Store) Ingest(ctx context.Context, tx *transaction.Transaction) (*types.IndexContext, error) {
	idxCtx, err := s.Parse(ctx, tx)
	if err != nil {
		return nil, err
	}
	if _, err = db.Rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, spend := range idxCtx.Spends {
			if s, err := proto.Marshal(spend.Spend); err != nil {
				panic(err)
			} else if err := pipe.HSet(ctx, "to:"+spend.Outpoint.JsonString(), "spend", s).Err(); err != nil {
				panic(err)
			}
			for tag, data := range spend.Data {
				for _, e := range data.Events {
					txidHex := hex.EncodeToString(idxCtx.Txid)
					pipe.ZAdd(ctx, "event", redis.Z{
						Score:  1 + float64(idxCtx.Block.Height)*math.Pow(2, -32),
						Member: e.EventKey(tag, txidHex, spend.Outpoint.Vout, spend.Satoshis),
					})
					if spend.Owner != "" {
						pipe.ZAdd(ctx, "oevent", redis.Z{
							Score:  1 + float64(idxCtx.Block.Height)*math.Pow(2, -32),
							Member: e.OwnerKey(spend.Owner, tag, txidHex, spend.Outpoint.Vout, spend.Satoshis),
						})
					}
				}
			}
		}

		for vout, txo := range idxCtx.Txos {
			var score float64
			if txo.Spend != nil {
				score = 1 + float64(idxCtx.Block.Height)*math.Pow(2, -32)
			} else {
				score = 0 + float64(idxCtx.Block.Height)/math.Pow(2, -32)
			}

			for _, event := range txo.Events {
				pipe.ZRem(ctx, "events", event)
			}
			for tag, data := range txo.Data {
				for _, e := range data.Events {
					pipe.ZAdd(ctx, "events", redis.Z{
						Score:  score,
						Member: e.EventKey(tag, hex.EncodeToString(idxCtx.Txid), uint32(vout), txo.Satoshis),
					})
				}
			}
			if t, err := proto.Marshal(txo); err != nil {
				return err
			} else {
				pipe.HSet(ctx, "to:"+txo.Outpoint.JsonString(), "txo", t).Err()
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return idxCtx, nil
}
