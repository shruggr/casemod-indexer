package lib

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/redis/go-redis/v9"
)

type IndexContext struct {
	Tx      *transaction.Transaction `json:"-"`
	Txid    ByteString               `json:"txid"`
	BlockId *Block                   `json:"block"`
	Txos    []*Txo                   `json:"txos"`
	Spends  []*Txo                   `json:"spends"`
}

func (ctx *IndexContext) SaveTxos(cmdable redis.Cmdable) {
	for _, txo := range ctx.Txos {
		txo.Save(ctx, cmdable)
	}
}

func (ctx *IndexContext) SaveSpends(cmdable redis.Cmdable) {
	scoreHeight := ctx.Height
	if scoreHeight == 0 {
		scoreHeight = uint32(time.Now().Unix())
	}
	spentScore, err := strconv.ParseFloat(fmt.Sprintf("1.%010d", scoreHeight), 64)
	if err != nil {
		panic(err)
	}
	for _, spend := range ctx.Spends {
		spend.SetSpend(ctx, cmdable, spentScore)
	}
}
