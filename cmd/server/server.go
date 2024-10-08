package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/GorillaPool/go-junglebus"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	_ "github.com/shruggr/casemod-indexer/cmd/server/docs"
	"github.com/shruggr/casemod-indexer/db"
	"github.com/shruggr/casemod-indexer/mod/bsv21"
	"github.com/shruggr/casemod-indexer/mod/ord"
	"github.com/shruggr/casemod-indexer/txostore"
	"github.com/shruggr/casemod-indexer/types"
)

var POSTGRES string
var CONCURRENCY int
var PORT int
var rdb *redis.Client
var cache *redis.Client
var jb *junglebus.Client

const INCLUDE_THREASHOLD = 10000000
const HOLDER_CACHE_TIME = 24 * time.Hour

var store = &txostore.Store{
	Indexers: []types.Indexer{
		&bsv21.Bsv21Indexer{
			InscrptionIndexer: &ord.InscriptionIndexer{},
		},
	},
}

func init() {
	wd, _ := os.Getwd()
	log.Println("CWD:", wd)
	godotenv.Load(fmt.Sprintf(`%s/../../.env`, wd))

	if POSTGRES == "" {
		POSTGRES = os.Getenv("POSTGRES_FULL")
	}

	log.Println("POSTGRES:", POSTGRES)
	var err error
	config, err := pgxpool.ParseConfig(POSTGRES)
	if err != nil {
		log.Panic(err)
	}
	config.MaxConnIdleTime = 15 * time.Second

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

	JUNGLEBUS := os.Getenv("JUNGLEBUS")
	if JUNGLEBUS == "" {
		JUNGLEBUS = "https://junglebus.gorillapool.io"
	}

	jb, err = junglebus.New(
		junglebus.WithHTTP(JUNGLEBUS),
	)
	if err != nil {
		log.Panicln(err.Error())
	}

	db.Initialize(rdb, cache, 8)
}

// @title BSV21 API
// @version 1.0
// @description This is a sample server server.
// @schemes http
func main() {
	// flag.IntVar(&CONCURRENCY, "c", 64, "Concurrency Limit")
	// flag.IntVar(&PORT, "p", 8082, "Port to listen on")
	// flag.Parse()
	PORT := os.Getenv("PORT")
	if PORT == "" {
		PORT = "8082"
	}

	app := fiber.New()
	app.Use(recover.New())
	app.Use(logger.New())

	app.Get("/", HealthCheck)
	app.Get("/swagger/*", swagger.HandlerDefault) // default

	app.Get("/yo", func(c *fiber.Ctx) error {
		return c.SendString("Yo!")
	})

	app.Get("/v1/txns/:txid", func(c *fiber.Ctx) error {
		if tx, err := db.LoadTx(c.Context(), c.Params("txid")); err != nil {
			return err
		} else {
			log.Println("TX", tx.TxID())
			return c.SendString(hex.EncodeToString(tx.Bytes()))
		}
	})

	app.Get("/v1/txos/outpoint/:outpoint", func(c *fiber.Ctx) error {
		if outpoint, err := types.NewOutpointFromString(c.Params("outpoint")); err != nil {
			return &fiber.Error{
				Code:    fiber.StatusBadRequest,
				Message: err.Error(),
			}
		} else if txo, err := store.LoadTxo(c.Context(), outpoint, nil); err != nil {
			return err
		} else if txo == nil {
			return &fiber.Error{
				Code:    fiber.StatusNotFound,
				Message: "Not Found",
			}
		} else {
			return c.JSON(txo)
		}
	})

	app.Post("/v1/txos/outpoint/:outpoint", func(c *fiber.Ctx) error {
		params := &txostore.LoadTxoParams{}
		if outpoint, err := types.NewOutpointFromString(c.Params("outpoint")); err != nil {
			return &fiber.Error{
				Code:    fiber.StatusBadRequest,
				Message: err.Error(),
			}
		} else if err = c.BodyParser(params); err != nil {
			return &fiber.Error{
				Code:    fiber.StatusBadRequest,
				Message: err.Error(),
			}
		} else if txo, err := store.LoadTxo(c.Context(), outpoint, params); err != nil {
			return err
		} else {
			return c.JSON(txo)
		}
	})

	app.Get("/v1/txos/txid/:txid", func(c *fiber.Ctx) error {
		if txos, err := store.LoadTxosByTxid(c.Context(), c.Params("txid"), nil); err != nil {
			return err
		} else {
			return c.JSON(txos)
		}
	})

	app.Post("/v1/txos/txid/:txid", func(c *fiber.Ctx) error {
		params := &txostore.LoadTxoParams{}
		if err := c.BodyParser(params); err != nil {
			return &fiber.Error{
				Code:    fiber.StatusBadRequest,
				Message: err.Error(),
			}
		} else if txos, err := store.LoadTxosByTxid(c.Context(), c.Params("txid"), nil); err != nil {
			return err
		} else {
			return c.JSON(txos)
		}
	})

	app.Post("/v1/txos/search", func(c *fiber.Ctx) error {
		var search txostore.SearchTxoParams
		if err := c.BodyParser(&search); err != nil {
			return &fiber.Error{
				Code:    fiber.StatusBadRequest,
				Message: err.Error(),
			}
		}
		if txos, err := store.SearchTxos(c.Context(), search); err != nil {
			return err
		} else {
			return c.JSON(txos)
		}
	})
	// app.Post("/v1/search", func(c *fiber.Ctx) error {
	// 	var search TxoSearch
	// 	if err := c.BodyParser(&search); err != nil {
	// 		return &fiber.Error{
	// 			Code:    fiber.StatusBadRequest,
	// 			Message: err.Error(),
	// 		}
	// 	}
	// 	var table string = "events"
	// 	if search.owner != "" {
	// 		table = "events:owner"
	// 		return &fiber.Error{
	// 			Code:    fiber.StatusBadRequest,
	// 			Message: "Invalid Parameters",
	// 		}
	// 	}
	// 	key := search.Indexer + ": "

	// 	if keys, _, err := rdb.ZScan(c.Context(), "events", 0, indexer+":*", 100).Result(); err != nil {
	// 		return &fiber.Error{
	// 			Code:    fiber.StatusInternalServerError,
	// 			Message: err.Error(),
	// 		}
	// 	} else {
	// 		return c.JSON(keys)
	// 	}
	// })

	// app.Get("/v1/search/:indexer/:tag", func(c *fiber.Ctx) error {
	// 	indexer := c.Params("indexer")
	// 	tag := c.Params("tag")
	// 	if indexer == "" {
	// 		return &fiber.Error{
	// 			Code:    fiber.StatusBadRequest,
	// 			Message: "Invalid Parameters",
	// 		}
	// 	}
	// 	if keys, _, err := rdb.ZScan(c.Context(), "events", 0, fmt.Sprintf("%s:%s:*", indexer, tag), 100).Result(); err != nil {
	// 		return &fiber.Error{
	// 			Code:    fiber.StatusInternalServerError,
	// 			Message: err.Error(),
	// 		}
	// 	} else {
	// 		return c.JSON(keys)
	// 	}
	// })

	// app.Get("/v1/flushall", func(c *fiber.Ctx) error {
	// 	if err := rdb.Del(c.Context(), "idx:prog").Err(); err != nil {
	// 		return &fiber.Error{
	// 			Code:    fiber.StatusInternalServerError,
	// 			Message: err.Error(),
	// 		}
	// 	}
	// 	iter := rdb.Scan(c.Context(), 0, "txo:*", 100).Iterator()
	// 	for iter.Next(c.Context()) {
	// 		if err := rdb.Del(c.Context(), iter.Val()).Err(); err != nil {
	// 			return &fiber.Error{
	// 				Code:    fiber.StatusInternalServerError,
	// 				Message: err.Error(),
	// 			}
	// 		}
	// 	}

	// 	return c.SendStatus(fiber.StatusOK)
	// })

	// app.Get("/v1/:indexer/flush", func(c *fiber.Ctx) error {
	// 	indexer := c.Params("indexer")
	// 	if indexer == "" {
	// 		return &fiber.Error{
	// 			Code:    fiber.StatusBadRequest,
	// 			Message: "Invalid Parameters",
	// 		}
	// 	}
	// 	if err := rdb.HDel(c.Context(), "idx:prog", indexer).Err(); err != nil {
	// 		return &fiber.Error{
	// 			Code:    fiber.StatusInternalServerError,
	// 			Message: err.Error(),
	// 		}
	// 	} else {
	// 		return c.SendStatus(fiber.StatusOK)
	// 	}
	// })

	log.Println("Listening on", PORT)
	app.Listen(fmt.Sprintf(":%s", PORT))
}

// HealthCheck godoc
// @Summary Show the status of server.
// @Description get the status of server.
// @Tags root
// @Accept */*
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router / [get]
func HealthCheck(c *fiber.Ctx) error {
	res := map[string]interface{}{
		"data": "Server is up and running",
	}

	if err := c.JSON(res); err != nil {
		return err
	}

	return nil
}
