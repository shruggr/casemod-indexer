package types

import (
	"github.com/bitcoin-sv/go-sdk/transaction"
)

type IndexContext struct {
	Txid   []byte `json:"txid"`
	Rawtx  []byte `json:"rawtx"`
	Tx     *transaction.Transaction
	Block  *Block `json:"block"`
	Spends []*Txo `json:"spends"`
	Txos   []*Txo `json:"txos"`
}

type Event struct {
	Id    string `json:"id"`
	Value string `json:"value"`
}

type IndexData struct {
	Tag    string      `json:"tag"`
	Data   []byte      `json:"data"`
	Events []*Event    `json:"events"`
	Deps   []*Outpoint `json:"deps"`
	Item   interface{} `json:"item" msgpack:"-"`
}

type Indexer interface {
	Tag() string
	Parse(*IndexContext, uint32) *IndexData
	Save(*IndexContext)
	Score(*IndexContext, uint32) float64
}

type BaseIndexer struct{}

func (b *BaseIndexer) Score(idxCtx *IndexContext, vout uint32) float64 {
	txo := idxCtx.Txos[vout]
	return txo.Score()
}
