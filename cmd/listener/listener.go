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
	"github.com/shruggr/casemod-indexer/listener"
)

var rdb *redis.Client

// var cache *redis.Client

var INDEXER string
var TOPIC string
var VERBOSE int = 0
var FROM_HEIGHT uint
var PAGE_SIZE = int64(100)

const REFRESH = 15 * time.Second

func init() {
	wd, _ := os.Getwd()
	log.Println("CWD:", wd)
	godotenv.Load(fmt.Sprintf(`%s/../../.env`, wd))

	flag.StringVar(&INDEXER, "id", "", "Indexer key")
	flag.StringVar(&TOPIC, "t", "", "Junglebus SuscriptionID")
	flag.UintVar(&FROM_HEIGHT, "s", uint(db.TRIGGER), "Start from block")
	flag.IntVar(&VERBOSE, "v", 0, "Verbose")
	flag.Parse()

	if opt, err := redis.ParseURL(os.Getenv("REDISDB")); err != nil {
		panic(err)
	} else {
		rdb = redis.NewClient(opt)
	}

	// if opt, err := redis.ParseURL(os.Getenv("REDISCACHE")); err != nil {
	// 	panic(err)
	// } else {
	// 	cache = redis.NewClient(opt)
	// }

	db.Initialize(rdb)
}

func main() {
	listener.Start(context.Background(),
		INDEXER,
		TOPIC,
		FROM_HEIGHT,
		VERBOSE,
	)
}
