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
)

var rdb *redis.Client

var cache *redis.Client

var VERBOSE int = 0
var PAGE_SIZE = uint(10000)

const REFRESH = 30 * time.Second

var ctx = context.Background()

func init() {
	wd, _ := os.Getwd()
	log.Println("CWD:", wd)
	godotenv.Load(fmt.Sprintf(`%s/../../.env`, wd))
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

	db.Initialize(rdb, cache, 10)
}

func main() {
	if err := syncBlocks(); err != nil {
		log.Panicln(err)
	}
}

func syncBlocks() (err error) {
	fromHeight := uint32(1)
	if blockIds, err := db.Txos.ZRangeArgsWithScores(ctx, redis.ZRangeArgs{
		Key:     db.BlockIdKey,
		ByScore: true,
		Rev:     true,
		Count:   1,
		Stop:    50000000,
	}).Result(); err != nil {
		log.Panicln(err)
	} else if len(blockIds) > 0 {
		fromHeight = uint32(blockIds[0].Score) - 5
	}

	for {
		height, err := db.SyncBlocks(ctx, fromHeight, PAGE_SIZE)
		if err != nil {
			return err
		}
		if height-fromHeight < uint32(PAGE_SIZE) {
			break
		}
		fromHeight = height
	}
	return nil
}
