package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/shruggr/casemod-indexer/db"
	"github.com/shruggr/casemod-indexer/listener"
	"github.com/shruggr/casemod-indexer/mod/bsv21"
	"github.com/shruggr/casemod-indexer/mod/ord"
	"github.com/shruggr/casemod-indexer/txostore"
	"github.com/shruggr/casemod-indexer/types"
)

const CONCURRENCY = 8

var PAGE_SIZE = int64(250)

var rdb *redis.Client

var cache *redis.Client

var INDEXER string = "bsv21"
var TOPIC string
var VERBOSE int = 1
var FROM_HEIGHT uint
var ctx = context.Background()

const REFRESH = 15 * time.Second

func init() {
	wd, _ := os.Getwd()
	log.Println("CWD:", wd)
	godotenv.Load(fmt.Sprintf(`%s/../../.env`, wd))

	flag.StringVar(&TOPIC, "t", "", "Junglebus SuscriptionID")
	flag.UintVar(&FROM_HEIGHT, "s", uint(811302), "Start from block")
	flag.IntVar(&VERBOSE, "v", 0, "Verbose")
	flag.Parse()

	if opt, err := redis.ParseURL(os.Getenv("REDISDB")); err != nil {
		panic(err)
	} else {
		rdb = redis.NewClient(opt)
	}

	if opt, err := redis.ParseURL(os.Getenv("REDISCACHE")); err != nil {
		panic(err)
	} else {
		cache = redis.NewClient(opt)
	}

	db.Initialize(rdb, cache, 8)
}

var prevProgress string
var prevScore atomic.Value
var store = &txostore.Store{
	Indexers: []types.Indexer{
		&bsv21.Bsv21Indexer{
			InscrptionIndexer: &ord.InscriptionIndexer{},
		},
	},
}

func main() {
	progress, err := rdb.HGet(ctx, db.ProgressKey, INDEXER).Result()
	if err != nil && err != redis.Nil {
		panic(err)
	}
	if progress == "" {
		progress = "-"
		prevScore.Store(0.0)
	} else {
		prevScore.Store(logIdToScore(progress) - 1)
	}

	go listener.Start(context.Background(),
		INDEXER,
		TOPIC,
		FROM_HEIGHT,
		VERBOSE,
	)
	go processLogs(progress)

	go processQueue()
	<-make(chan struct{})
}

func processLogs(progress string) {
	limiter := make(chan struct{}, CONCURRENCY)
	var wg sync.WaitGroup
	for {
		log.Println("Progress", progress)
		if stream, err := rdb.XRangeN(ctx, db.LogKey(INDEXER), progress, "+", PAGE_SIZE).Result(); err != nil {
			panic(err)
		} else {
			start := time.Now()
			for _, msg := range stream {
				wg.Add(1)
				limiter <- struct{}{}
				progress = msg.ID
				go func(msg redis.XMessage) {
					score := logIdToScore(msg.ID)
					txid := msg.Values["txn"].(string)
					if VERBOSE > 0 {
						log.Println("Parsing", txid)
					}
					var tx *transaction.Transaction
					if tx, err = db.LoadTx(ctx, txid); err != nil {
						log.Panicln(txid, err)
					}
					idxCtx := txostore.NewIndexContext(ctx, tx)
					if err = store.ParseOutputs(ctx, idxCtx); err != nil {
						log.Panicln(txid, err)
					} else {
						ids := make(map[string]struct{})
						for _, txo := range idxCtx.Txos {
							if item, ok := txo.Data["bsv21"]; !ok {
								continue
							} else if bsv21, ok := item.Obj.(*bsv21.Bsv21); ok {
								queueKey := db.QueueKey(INDEXER) + bsv21.Id.String()
								if err = db.Txos.ZAdd(ctx, queueKey, redis.Z{
									Score:  score,
									Member: txid,
								}).Err(); err != nil {
									panic(err)
								}
								ids[queueKey] = struct{}{}
							}
						}
					}
					<-limiter
					wg.Done()
				}(msg)
			}
			wg.Wait()
			elapsed := time.Since(start)
			log.Println("Processed", len(stream), "in", elapsed, "(", float64(len(stream))/elapsed.Seconds(), "tx/s )")
			if err = rdb.HSet(ctx, db.ProgressKey, INDEXER, progress).Err(); err != nil {
				panic(err)
			}

			if prevProgress == progress {
				log.Println("Waiting for new txns")
				time.Sleep(REFRESH)
			} else {
				prevScore.Store(logIdToScore(progress))
			}
			prevProgress = progress
		}
	}
}

func processQueue() {
	limiter := make(chan struct{}, CONCURRENCY)
	var wg sync.WaitGroup
	for {
		queueKey := db.QueueKey(INDEXER)
		iter := db.Txos.Scan(ctx, 0, queueKey+"*", 1000).Iterator()
		count := 0
		for iter.Next(ctx) {
			count++
			wg.Add(1)
			limiter <- struct{}{}
			tokenId := strings.TrimPrefix(iter.Val(), queueKey)
			go func(tokenId string) {
				defer func() {
					<-limiter
					wg.Done()
				}()
				if VERBOSE > 0 {
					log.Println("Processing", tokenId)
				}
				processToken(tokenId)
				<-limiter
				wg.Done()
			}(tokenId)
		}
		wg.Wait()
		if count == 0 {
			time.Sleep(60 * time.Second)
		}
	}
}

func processToken(tokenId string) {
	queueKey := db.QueueKey(INDEXER) + tokenId
	if txids, err := db.Txos.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     queueKey,
		Start:   0,
		Stop:    prevScore.Load().(float64),
		ByScore: true,
		Count:   100,
	}).Result(); err != nil {
		panic(err)
	} else {
		pipe := db.Txos.Pipeline()
		for _, txid := range txids {
			if VERBOSE > 0 {
				log.Println("Processing", tokenId, txid)
			}
			if tx, err := db.LoadTxAndProof(ctx, txid); err != nil {
				panic(err)
			} else if _, err := store.Ingest(ctx, tx); err != nil {
				panic(err)
			} else if err := pipe.ZRem(ctx, queueKey, txid).Err(); err != nil {
				panic(err)
			}
		}
		if _, err := pipe.Exec(ctx); err != nil {
			panic(err)
		}
	}
}

func logIdToScore(logId string) float64 {
	idx := []byte("000000000")
	parts := strings.Split(logId, "-")
	copy(idx[9-len(parts[1]):], parts[1])
	score, err := strconv.ParseFloat(fmt.Sprintf("%s.%s", parts[0], idx), 64)
	if err != nil {
		panic(err)
	}
	return score
}
