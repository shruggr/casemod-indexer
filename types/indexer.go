package types

import (
	"encoding/json"

	"github.com/bitcoin-sv/go-sdk/transaction"
)

type IndexContext struct {
	Txid   []byte
	Rawtx  []byte
	Tx     *transaction.Transaction
	Block  *Block
	Spends []*Txo
	Txos   []*Txo
}

type EventLog struct {
	Label string
	Value string
}

type IndexData struct {
	Tag    string
	Data   []byte
	Events []*EventLog
	Deps   []*Outpoint
	Obj    interface{}
}

func (id *IndexData) MarshalJSON() (bytes []byte, err error) {
	return json.Marshal(id.Obj)
}

type Indexer interface {
	Tag() string
	Parse(*IndexContext, uint32) *IndexData
	Save(*IndexContext)
	Score(txo *Txo) float64
	UnmarshalData([]byte) (any, error)
}

type BaseIndexer struct{}

func (b *BaseIndexer) Score(txo *Txo) float64 {
	return txo.Score()
}
