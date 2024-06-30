package db

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/GorillaPool/go-junglebus"
	"github.com/bitcoin-sv/go-sdk/transaction"

	"github.com/ordishs/go-bitcoin"
	"github.com/redis/go-redis/v9"
)

var TRIGGER = uint32(783968)

var Rdb *redis.Client
var Cache *redis.Client
var JB *junglebus.Client
var bit *bitcoin.Bitcoind
var ctx = context.Background()

var JUNGLEBUS string

func Initialize(rdb *redis.Client, cache *redis.Client) (err error) {
	// Db = postgres
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

	if os.Getenv("BITCOIN_HOST") != "" {
		port, _ := strconv.ParseInt(os.Getenv("BITCOIN_PORT"), 10, 32)
		bit, err = bitcoin.New(os.Getenv("BITCOIN_HOST"), int(port), os.Getenv("BITCOIN_USER"), os.Getenv("BITCOIN_PASS"), false)
		if err != nil {
			log.Panic(err)
		}
	}

	return
}

func LoadTx(txid string) (*transaction.Transaction, error) {
	if rawtx, err := LoadRawtx(txid); err != nil {
		return nil, err
	} else if len(rawtx) == 0 {
		return nil, fmt.Errorf("missing-txn %s", txid)
	} else if tx, err := transaction.NewTransactionFromBytes(rawtx); err != nil {
		return nil, err
	} else if tx.MerklePath, err = LoadProof(txid); err != nil {
		return nil, err
	} else {
		return tx, nil
	}
}

func LoadRawtx(txid string) (rawtx []byte, err error) {
	rawtx, _ = Cache.HGet(ctx, "tx", txid).Bytes()

	if len(rawtx) > 0 {
		return rawtx, nil
	} else {
		rawtx = []byte{}

	}

	if len(rawtx) == 0 && JUNGLEBUS != "" {
		if resp, err := http.Get(fmt.Sprintf("%s/v1/tx/%s/bin", JUNGLEBUS, txid)); err != nil {
			log.Println("JB GetRawTransaction", err)
		} else if resp.StatusCode == 200 {
			rawtx, _ = io.ReadAll(resp.Body)
		}
	}

	if len(rawtx) == 0 && bit != nil {
		// log.Println("Requesting tx from node", txid)
		if r, err := bit.GetRawTransactionRest(txid); err == nil {
			rawtx, _ = io.ReadAll(r)
		}
	}

	if len(rawtx) == 0 {
		err = fmt.Errorf("LoadRawtx: missing-txn %s", txid)
		return
	}

	Cache.HSet(ctx, "tx", txid, rawtx).Err()
	return
}

func LoadProof(txid string) (*transaction.MerklePath, error) {
	proof, _ := Cache.HGet(ctx, "proof", txid).Bytes()
	if len(proof) > 0 {
		return transaction.NewMerklePathFromBinary(proof)
	} else if JUNGLEBUS != "" {
		if resp, err := http.Get(fmt.Sprintf("%s/v1/tx/%s/proof", JUNGLEBUS, txid)); err != nil {
			log.Println("JB GetProof", err)
		} else if resp.StatusCode == 200 {
			proof, _ = io.ReadAll(resp.Body)
			return transaction.NewMerklePathFromBinary(proof)
		}
	}
	return nil, nil
}

// func LoadTxOut(outpoint *Outpoint) (txout *bt.Output, err error) {
// 	txo, err := JB.GetTxo(ctx, hex.EncodeToString(outpoint.Txid()), outpoint.Vout())
// 	if err != nil {
// 		return
// 	}
// 	reader := bytes.NewReader(txo)
// 	txout = &bt.Output{}
// 	_, err = txout.ReadFrom(reader)
// 	return
// }

// func GetSpend(outpoint *Outpoint) (spend []byte, err error) {
// 	return JB.GetSpend(ctx, hex.EncodeToString(outpoint.Txid()), outpoint.Vout())
// }

// func GetChaintip() (*models.BlockHeader, error) {
// 	return JB.GetChaintip(ctx)

// }
