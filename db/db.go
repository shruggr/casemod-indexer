package db

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/GorillaPool/go-junglebus"
	"github.com/bitcoin-sv/go-sdk/transaction"

	"github.com/redis/go-redis/v9"
)

var TRIGGER = uint32(783968)

var Rdb *redis.Client

var Cache *redis.Client
var JB *junglebus.Client

var JUNGLEBUS string

func Initialize(rdb *redis.Client, cache *redis.Client) (err error) {
	Rdb = rdb
	Cache = cache

	JUNGLEBUS = os.Getenv("JUNGLEBUS")
	log.Println("JUNGLEBUS", JUNGLEBUS)
	if JUNGLEBUS != "" {
		JB, err = junglebus.New(
			junglebus.WithHTTP(JUNGLEBUS),
		)
		if err != nil {
			return
		}
	}
	return
}

func LoadTx(ctx context.Context, txid string) (*transaction.Transaction, error) {
	if rawtx, err := LoadRawtx(ctx, txid); err != nil {
		return nil, err
	} else if len(rawtx) == 0 {
		return nil, fmt.Errorf("missing-txn %s", txid)
	} else if tx, err := transaction.NewTransactionFromBytes(rawtx); err != nil {
		return nil, err
	} else if tx.MerklePath, err = LoadProof(ctx, txid); err != nil {
		return nil, err
	} else {
		return tx, nil
	}
}

func LoadRawtx(ctx context.Context, txid string) (rawtx []byte, err error) {
	rawtx, _ = Cache.HGet(ctx, RawtxKey, txid).Bytes()

	if len(rawtx) > 0 {
		return rawtx, nil
	}

	if len(rawtx) == 0 && JUNGLEBUS != "" {
		url := fmt.Sprintf("%s/v1/transaction/get/%s/bin", JUNGLEBUS, txid)
		// fmt.Println("JB", url)
		if resp, err := http.Get(url); err != nil {
			log.Println("JB GetRawTransaction", err)
		} else if resp.StatusCode > 200 {
			return rawtx, fmt.Errorf("JB GetRawTransaction %d %s", resp.StatusCode, txid)
		} else {
			rawtx, _ = io.ReadAll(resp.Body)
		}
	}

	if len(rawtx) == 0 {
		err = fmt.Errorf("LoadRawtx: missing-txn %s", txid)
		return
	}

	Cache.HSet(ctx, RawtxKey, txid, rawtx).Err()
	return
}

func LoadProof(ctx context.Context, txid string) (*transaction.MerklePath, error) {
	proof, _ := Cache.HGet(ctx, ProofKey, txid).Bytes()
	if len(proof) > 0 {
		return transaction.NewMerklePathFromBinary(proof)
	} else if JUNGLEBUS != "" {
		url := fmt.Sprintf("%s/v1/transaction/proof/%s", JUNGLEBUS, txid)
		// log.Println("JB", url)
		if resp, err := http.Get(url); err != nil {
			log.Println("JB GetProof", err)
		} else if resp.StatusCode == 200 {
			proof, _ = io.ReadAll(resp.Body)
			if err = Cache.HSet(ctx, ProofKey, txid, proof).Err(); err != nil {
				return nil, err
			}
			return transaction.NewMerklePathFromBinary(proof)
		}
	}
	return nil, nil
}

type TxoSearch struct {
	Indexer *string `json:"indexer"`
	Tag     *string `json:"tag"`
	Id      *string `json:"id"`
	Value   *string `json:"value"`
	Owner   *string `json:"owner"`
	Spent   *bool   `json:"spent"`
	Cursor  uint64  `json:"cursor"`
}

func SyncBlocks(ctx context.Context, fromHeight uint32, pageSize uint) (uint32, error) {
	log.Println("Syncing from", fromHeight)
	height := fromHeight
	blocks, err := JB.GetBlockHeaders(ctx, strconv.FormatUint(uint64(fromHeight), 10), pageSize)
	if err != nil {
		log.Panicln(err)
	}
	if _, err := Rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, block := range blocks {
			if blockData, err := json.Marshal(block); err != nil {
				return err
			} else if err := pipe.HSet(ctx, BlockKey, BlockHeightKey(block.Height), blockData).Err(); err != nil {
				return err
			} else if err := pipe.ZAdd(ctx, BlockIdKey, redis.Z{
				Score:  float64(block.Height),
				Member: block.Hash,
			}).Err(); err != nil {
				return err
			}
			height = block.Height + 1
		}
		return nil
	}); err != nil {
		return fromHeight, err
	}
	return height, nil
}

// func (search *TxoSearch) Search(ctx context.Context) ([]*Outpoint, error) {
// 	var key string = "events"
// 	var pattern string
// 	if search.Owner != nil {
// 		key = "oevents"
// 		pattern = *search.Owner + ":"
// 	}
// 	if search.Indexer != nil {
// 		pattern = pattern + *search.Indexer + ":"
// 		if search.Tag != nil {
// 			pattern = pattern + *search.Tag
// 			if search.Id != nil {
// 				pattern = pattern + ":" + *search.Id
// 				if search.Value != nil {
// 					pattern = pattern + ":" + *search.Value
// 				}
// 			}
// 		}
// 	}
// 	start := float64(0)
// 	end := float64(2)
// 	if search.Spent != nil && *search.Spent {
// 		start = 1
// 	} else if search.Spent != nil && !*search.Spent {
// 		end = 1
// 	}
// 	Rdb.ZRangeByScore(ctx, key, &redis.ZRangeBy{})
// 	// if keys, _, err := Rdb.ZScan(ctx, key, start, pattern+":*", 100).Result(); err != nil {

// 	// }
// }
