package store

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/util"
	"github.com/redis/go-redis/v9"
	"github.com/shruggr/casemod-indexer/db"
	"github.com/shruggr/casemod-indexer/lib"
	"github.com/shruggr/casemod-indexer/mod"
	"github.com/shruggr/casemod-indexer/types"
	"google.golang.org/protobuf/proto"
)

func Parse(ctx context.Context, tx *transaction.Transaction, indexers []mod.Indexer) (*types.IndexContext, error) {
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
		Rawtx:  tx.Bytes(),
		Txid:   txid,
		Block:  block,
		Spends: make([]*types.Txo, 0, len(tx.Inputs)),
		Txos:   make([]*types.Txo, 0, len(tx.Outputs)),
	}

	for _, input := range tx.Inputs {
		if input.SourceTransaction == nil {
			if input.SourceTransaction, err = db.LoadTx(hex.EncodeToString(input.SourceTXID)); err != nil {
				return nil, err
			}
		} else if _, err = Ingest(ctx, input.SourceTransaction, indexers); err != nil {
			return nil, err
		}

	}

	if !tx.IsCoinbase() {
		for vin, input := range tx.Inputs {
			outpoint := &types.Outpoint{
				Txid: input.SourceTXID,
				Vout: input.SourceTxOutIndex,
			}
			spend := &types.Txo{}
			if t, err := db.Rdb.HGet(ctx, "o"+outpoint.JsonString(), "txo").Bytes(); err == redis.Nil {
				spend = &types.Txo{
					Outpoint: outpoint,
					Satoshis: *input.PreviousTxSatoshis(),
					Script:   *input.PreviousTxScript(),
				}
			} else if err != nil {
				return nil, err
			} else if err = proto.Unmarshal(t, spend); err != nil {
				return nil, err
			}
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
		txo := &types.Txo{}
		if t, err := db.Rdb.HGet(ctx, "o"+outpoint.JsonString(), "txo").Bytes(); err == redis.Nil {
			txo = &types.Txo{
				Outpoint: outpoint,
				Satoshis: output.Satoshis,
				Script:   *output.LockingScript,
			}
		} else if err != nil {
			return nil, err
		} else if err = proto.Unmarshal(t, txo); err != nil {
			return nil, err
		}

		if txo.Owner != nil {
			owner, _ := lib.NewPKHashFromScript(txo.Script)
			txo.Owner = *owner
		}
		txo.Block = block
		idxCtx.Txos = append(idxCtx.Txos, txo)
		for _, indexer := range indexers {
			data := indexer.Parse(idxCtx, uint32(vout))
			if data != nil {
				txo.Data[indexer.Tag()] = data
			}
		}
	}
	for _, indexer := range indexers {
		indexer.Save(idxCtx)
	}

	return idxCtx, nil
}

func Ingest(ctx context.Context, tx *transaction.Transaction, indexers []mod.Indexer) (*types.IndexContext, error) {
	idxCtx, err := Parse(ctx, tx, indexers)
	if err != nil {
		return nil, err
	}
	if _, err = db.Rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, spend := range idxCtx.Spends {
			if s, err := proto.Marshal(spend.Spend); err != nil {
				return err
			} else {
				pipe.HSet(ctx, "o"+spend.Outpoint.JsonString(), "spend", s)
			}
		}

		for vout, txo := range idxCtx.Txos {
			var score float64
			if txo.Spend != nil {
				score = 1 + float64(txo.Spend.Block.Height)*math.Pow(2, -32)

			} else {
				score = 0 + float64(txo.Spend.Block.Height)/math.Pow(2, -32)
			}

			for _, event := range txo.Events {
				pipe.ZRem(ctx, "events", event)
			}
			for _, data := range txo.Data {
				for _, e := range data.Events {
					event := fmt.Sprintf("%s:%s:%s:%x:%d", data.Tag, e.Id, e.Value, idxCtx.Txid, vout)
					pipe.ZAdd(ctx, "events", redis.Z{
						Score:  score,
						Member: event,
					})
				}
			}
			if t, err := proto.Marshal(txo); err != nil {
				return err
			} else {
				pipe.HSet(ctx, "o"+txo.Outpoint.JsonString(), "txo", t).Err()
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return idxCtx, nil

}
