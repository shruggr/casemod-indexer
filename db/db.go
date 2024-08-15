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
	"github.com/shruggr/casemod-indexer/types"

	"github.com/redis/go-redis/v9"
)

// var TRIGGER = uint32(783968)

var Txos *redis.Client
var Blockchain *redis.Client
var JB *junglebus.Client

var JUNGLEBUS string
var reqLimiter chan struct{}

func Initialize(txoDb *redis.Client, blockchainDb *redis.Client, concurrentRequests uint8) (err error) {
	Txos = txoDb
	Blockchain = blockchainDb
	reqLimiter = make(chan struct{}, concurrentRequests)

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

func SaveRawtx(ctx context.Context, txid string, rawtx []byte) {
	if err := Blockchain.HSet(ctx, RawtxKey, txid, rawtx).Err(); err != nil {
		log.Panicf("SaveTx %s %s", txid, err)
	}
}

func SaveTx(ctx context.Context, tx *transaction.Transaction) {
	txid := tx.TxID()
	SaveRawtx(ctx, txid, tx.Bytes())
	if tx.MerklePath != nil {
		if err := Blockchain.HSet(ctx, ProofKey, txid, tx.MerklePath.Bytes()).Err(); err != nil {
			log.Panicln("SaveProof", txid, err)
		}
	}
}

func LoadTx(ctx context.Context, txid string) (tx *transaction.Transaction, err error) {
	if rawtx, err := Blockchain.HGet(ctx, RawtxKey, txid).Bytes(); err != nil && err != redis.Nil {
		return nil, err
	} else if len(rawtx) > 0 {
		if tx, err = transaction.NewTransactionFromBytes(rawtx); err != nil {
			log.Panicln("NewTransactionFromBytes", txid, err)
		}
	}
	return LoadTxRemote(ctx, txid)
}

func LoadTxBlock(ctx context.Context, txid string) *types.Block {
	if score, err := Txos.ZScore(ctx, TxStatusKey, txid).Result(); err != nil && err != redis.Nil {
		log.Panicln("LoadTxBlock", txid, err)
		return nil
	} else if err == redis.Nil {
		return nil
	} else {
		return types.ParseBlockScore(score)
	}
}

func LoadTxRemote(ctx context.Context, txid string) (*transaction.Transaction, error) {
	url := fmt.Sprintf("%s/v1/transaction/get/%s/bin", JUNGLEBUS, txid)
	reqLimiter <- struct{}{}
	resp, err := http.Get(url)
	<-reqLimiter
	if err != nil {
		log.Println("JB GetRawTransaction", err)
		return nil, err
	} else if resp.StatusCode > 200 {
		return nil, fmt.Errorf("JB GetRawTransaction %d %s", resp.StatusCode, txid)
	} else if rawtx, err := io.ReadAll(resp.Body); err != nil {
		log.Println("JB ReadRawTransaction", err)
		return nil, err
	} else if tx, err := transaction.NewTransactionFromBytes(rawtx); err != nil {
		return nil, err
	} else {
		SaveTx(ctx, tx)
		return tx, nil
	}
}

func LoadTxAndProof(ctx context.Context, txid string) (*transaction.Transaction, error) {
	if tx, err := LoadTx(ctx, txid); err != nil {
		return nil, err
	} else if tx.MerklePath, err = LoadProof(ctx, txid); err != nil {
		return nil, err
	} else {
		return tx, nil
	}
}

func LoadProof(ctx context.Context, txid string) (*transaction.MerklePath, error) {
	log.Println("LoadProof", txid)
	if proof, err := Blockchain.HGet(ctx, ProofKey, txid).Bytes(); err != nil && err != redis.Nil {
		return nil, err
	} else if len(proof) > 0 {
		return transaction.NewMerklePathFromBinary(proof)
	} else if JUNGLEBUS != "" {
		url := fmt.Sprintf("%s/v1/transaction/proof/%s/bin", JUNGLEBUS, txid)
		// log.Println("JB", url)
		reqLimiter <- struct{}{}
		resp, err := http.Get(url)
		<-reqLimiter
		if err != nil {
			log.Println("JB GetProof", err)
		} else if resp.StatusCode == 200 {
			proof, _ = io.ReadAll(resp.Body)
			if err = Blockchain.HSet(ctx, ProofKey, txid, proof).Err(); err != nil {
				return nil, err
			}
			return transaction.NewMerklePathFromBinary(proof)
		}
	}
	return nil, nil
}

func SyncBlocks(ctx context.Context, fromHeight uint32, pageSize uint) (uint32, error) {
	log.Println("Syncing from", fromHeight)
	height := fromHeight
	blocks, err := JB.GetBlockHeaders(ctx, strconv.FormatUint(uint64(fromHeight), 10), pageSize)
	if err != nil {
		log.Panicln(err)
	}
	if _, err := Txos.Pipelined(ctx, func(pipe redis.Pipeliner) error {
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
