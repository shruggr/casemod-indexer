package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/GorillaPool/go-junglebus"
	"github.com/GorillaPool/go-junglebus/models"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/shruggr/casemod-indexer/db"
)

var rdb *redis.Client
var cache *redis.Client

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

	if opt, err := redis.ParseURL(os.Getenv("REDISCACHE")); err != nil {
		panic(err)
	} else {
		cache = redis.NewClient(opt)
	}

	db.Initialize(rdb, cache)
}

func main() {
	Start(context.Background(), INDEXER, TOPIC, FROM_HEIGHT, VERBOSE)
}

func Start(ctx context.Context, indexer string, topic string, progress uint, verbose int) {
	var tip *models.BlockHeader
	var lastBlock uint32
	var lastIdx uint64
	tip, err := db.JB.GetChaintip(ctx)
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			time.Sleep(REFRESH)
			if tip, err = db.JB.GetChaintip(ctx); err != nil {
				log.Println("GetChaintip", err)
			}
		}
	}()

	if indexer != "" {
		if logs, err := db.Rdb.XRevRangeN(ctx, "idx:log:"+indexer, "+", "-", 1).Result(); err != nil {
			log.Panic(err)
		} else if len(logs) > 0 {
			parts := strings.Split(logs[0].ID, "-")
			if height, err := strconv.ParseUint(parts[0], 10, 32); err == nil && height > uint64(progress) {
				progress = uint(height)
				lastBlock = uint32(progress)
			} else if idx, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
				lastIdx = idx
			}
		}
	}

	var txCount int
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			if txCount > 0 {
				log.Printf("Blk %d I %d - %d txs %d/s\n", lastBlock, lastIdx, txCount, txCount/10)
			}
			txCount = 0
		}
	}()

	var sub *junglebus.Subscription
	var eventHandler junglebus.EventHandler
	queueKey := fmt.Sprintf("queue:%s", indexer)
	eventHandler = junglebus.EventHandler{
		OnStatus: func(status *models.ControlResponse) {
			if verbose > 0 {
				log.Printf("[STATUS]: %d %v\n", status.StatusCode, status.Message)
			}
			if status.StatusCode == 200 {
				progress = uint(status.Block) + 1
				if progress > uint(tip.Height-5) {
					sub.Unsubscribe()
					ticker := time.NewTicker(REFRESH)
					for range ticker.C {
						if progress <= uint(tip.Height-5) {
							if sub, err = db.JB.Subscribe(
								context.Background(),
								topic,
								uint64(progress),
								eventHandler,
							); err != nil {
								panic(err)
							}
							break
						}
					}
				}
			}
			if status.StatusCode == 999 {
				log.Println(status.Message)
				log.Println("Unsubscribing...")
				sub.Unsubscribe()
				os.Exit(0)
				return
			}
		},
		OnTransaction: func(txn *models.TransactionResponse) {
			if verbose > 0 {
				log.Printf("[TX]: %d %s\n", len(txn.Transaction), txn.Id)
			}
			scoreStr := fmt.Sprintf("%07d.%11d", txn.BlockHeight, txn.BlockIndex)
			if score, err := strconv.ParseFloat(scoreStr, 64); err != nil {
				log.Panicln(err)
			} else if err := rdb.ZAdd(ctx, queueKey, redis.Z{
				Member: txn.Id,
				Score:  score,
			}).Err(); err != nil {
				log.Panicln(err)
			}
			lastBlock = txn.BlockHeight
			lastIdx = txn.BlockIndex
			txCount++
		},
		OnMempool: func(txn *models.TransactionResponse) {
			if verbose > 0 {
				log.Printf("[MEM]: %d %s\n", len(txn.Transaction), txn.Id)
			}
			if err := rdb.ZAdd(ctx, queueKey, redis.Z{
				Member: txn.Id,
				Score:  float64(time.Now().Unix()),
			}).Err(); err != nil {
				log.Panicln(err)
			}
		},
		OnError: func(err error) {
			log.Panicf("[ERROR]: %v\n", err)
		},
	}

	log.Println("Subscribing to Junglebus from block", progress)
	if sub, err = db.JB.SubscribeWithQueue(
		context.Background(),
		topic,
		uint64(progress),
		0,
		eventHandler,
		&junglebus.SubscribeOptions{
			QueueSize: 10000,
			LiteMode:  true,
		},
	); err != nil {
		panic(err)
	}
	defer func() {
		sub.Unsubscribe()
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Printf("Caught signal")
		fmt.Println("Unsubscribing and exiting...")
		sub.Unsubscribe()
		os.Exit(0)
	}()

	<-make(chan struct{})

}
