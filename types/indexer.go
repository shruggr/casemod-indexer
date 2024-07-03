package types

import (
	"fmt"

	"github.com/bitcoin-sv/go-sdk/transaction"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type IndexContext struct {
	Txid   []byte `json:"txid"`
	Rawtx  []byte `json:"rawtx"`
	Tx     *transaction.Transaction
	Block  *Block `json:"block"`
	Spends []*Txo `json:"spends"`
	Txos   []*Txo `json:"txos"`
}

type IndexData struct {
	RawData
	Item protoreflect.ProtoMessage
}

type Indexer interface {
	Tag() string
	Parse(*IndexContext, uint32) *IndexData
	Save(*IndexContext)
	UnmarshalData(raw []byte) (protoreflect.ProtoMessage, error)
}

func (e *Event) EventKey(tag string, txid string, vout uint32, satoshis uint64) string {
	return fmt.Sprintf("%s:%s:%s:%s:%d:%d", tag, e.Id, e.Value, txid, vout, satoshis)
}

func (e *Event) OwnerKey(owner string, tag string, txid string, vout uint32, satoshis uint64) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%d:%d", owner, tag, e.Id, e.Value, txid, vout, satoshis)
}
