package txostore

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/util"
	"github.com/redis/go-redis/v9"
	"github.com/shruggr/casemod-indexer/lib"
	"github.com/shruggr/casemod-indexer/types"
)

func Parse(ctx context.Context, tx *transaction.Transaction, indexers []lib.Indexer) (*types.IndexContext, error) {
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

	if !tx.IsCoinbase() {
		for vin, input := range tx.Inputs {
			if input.SourceTransaction == nil {
				if input.SourceTransaction, err = lib.LoadTx(hex.EncodeToString(input.SourceTXID)); err != nil {
					return nil, err
				}
			} else if _, err = Ingest(ctx, input.SourceTransaction, indexers); err != nil {
				return nil, err
			}
			outpoint := lib.NewOutpoint(input.SourceTXID, input.SourceTxOutIndex)
			spend := &lib.Txo{
				Outpoint: outpoint,
				Satoshis: *input.PreviousTxSatoshis(),
				Script:   *input.PreviousTxScript(),
			}
			if t, err := lib.Rdb.JSONGet(ctx, outpoint.String(), "$").Result(); err != nil && err != redis.Nil {
				return nil, err
			} else if err = json.Unmarshal([]byte(t), spend); err != nil {
				return nil, err
			}
			spend.Spend = &lib.Spend{
				Txid:  txid,
				Vin:   uint32(vin),
				Block: block,
			}
			idxCtx.Spends = append(idxCtx.Spends, spend)
		}
	}

	for vout, output := range tx.Outputs {
		outpoint := lib.NewOutpoint(txid, uint32(vout))
		txo := &lib.Txo{
			Outpoint: outpoint,
		}
		if t, err := lib.Rdb.JSONGet(ctx, outpoint.String(), "$").Result(); err == redis.Nil {
			txo.Satoshis = output.Satoshis
			txo.Script = *output.LockingScript
		} else if err != nil {
			return nil, err
		} else if err = json.Unmarshal([]byte(t), txo); err != nil {
			return nil, err
		}
		if txo.Owner != nil {
			txo.Owner, _ = lib.NewPKHashFromScript(txo.Script)
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

func Ingest(ctx context.Context, tx *transaction.Transaction, indexers []lib.Indexer) (*lib.IndexContext, error) {
	idxCtx, err := Parse(ctx, tx, indexers)
	if err != nil {
		return nil, err
	}
	// for _, spend := range idxCtx.Spends {

	for vout, txo := range idxCtx.Txos {
		if err = lib.Rdb.JSONSet(ctx, txo.ID(), "$", txo).Err(); err != nil {
			return nil, err
		}
		var score float64
		if txo.Spend != nil {
			score = 1 + float64(txo.Spend.Block.Height)*math.Pow(2, -32)

		} else {
			score = 0 + float64(txo.Spend.Block.Height)/math.Pow(2, -32)
		}
		for _, indexer := range indexers {
			data := txo.Data[indexer.Tag()]

			for _, e := range data.Events {
				val := fmt.Sprintf("%s:%s:%s:%x:%d", indexer.Tag(), e.Id, e.Value, idxCtx.Txid, vout)
				if err = lib.Rdb.ZAdd(ctx, "events", redis.Z{
					Score:  score,
					Member: val,
				}).Err(); err != nil {
					return nil, err
				}

			}
		}
	}
	return idxCtx, nil
}
