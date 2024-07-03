package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/shruggr/casemod-indexer/db"
	"github.com/shruggr/casemod-indexer/mod/bsv21"
	"github.com/shruggr/casemod-indexer/mod/ord"
	store "github.com/shruggr/casemod-indexer/txostore"
	"github.com/shruggr/casemod-indexer/types"
)

var rdb *redis.Client
var cache *redis.Client

var INDEXER string = "bsv21"
var TOPIC string
var VERBOSE int = 0
var FROM_HEIGHT uint
var PAGE_SIZE = int64(100)
var ctx = context.Background()

const REFRESH = 15 * time.Second

func init() {
	wd, _ := os.Getwd()
	log.Println("CWD:", wd)
	godotenv.Load(fmt.Sprintf(`%s/../../.env`, wd))

	flag.IntVar(&VERBOSE, "v", 0, "Verbose")
	flag.Parse()

	rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDISDB"),
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	cache = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDISCAHCE"),
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	db.Initialize(rdb, cache)
}

func main() {
	// go listener.Start(ctx, INDEXER, TOPIC, FROM_HEIGHT, VERBOSE)
	prog, err := rdb.HGet(ctx, "idx:prog", INDEXER).Result()
	if err != nil && err != redis.Nil {
		panic(err)
	}
	if prog == "" {
		prog = "-"
	}
	indexers := []types.Indexer{
		&ord.InscriptionIndexer{},
		&bsv21.Bsv21Indexer{},
	}

	var prevProg string
	for {
		if prevProg == prog {
			log.Println("Waiting for new txns")
			time.Sleep(REFRESH)
			continue
		}
		prevProg = prog
		if VERBOSE > 0 {
			log.Println("Progress", prog)
		}
		if stream, err := rdb.XRangeN(ctx, "idx:log:"+INDEXER, prog, "+", PAGE_SIZE).Result(); err != nil {
			panic(err)
		} else {
			for _, msg := range stream {
				prog = msg.ID
				txid := msg.Values["txn"].(string)
				log.Println("Indexing", txid)
				if tx, err := db.LoadTx(ctx, txid); err != nil {
					panic(err)
				} else if tx == nil {
					log.Panicln("Missing tx", txid)
				} else if _, err := store.Ingest(ctx, tx, indexers); err != nil {
					panic(err)
				}
			}
			if err = rdb.HSet(ctx, "idx:prog", INDEXER, prog).Err(); err != nil {
				panic(err)
			}
		}
	}
}
