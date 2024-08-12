package types

import (
	"bytes"

	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/bitcoin-sv/go-sdk/transaction"
)

type Output struct {
	Satoshis uint64 `json:"satoshis"`
	Script   []byte `json:"script"`
}

func (o *Output) Bytes() []byte {
	return (&transaction.TransactionOutput{
		Satoshis:      o.Satoshis,
		LockingScript: (*script.Script)(&o.Script),
	}).Bytes()
}

func NewOutputFromBytes(b []byte) *Output {
	to := transaction.TransactionOutput{}
	to.ReadFrom(bytes.NewReader(b))
	return &Output{
		Satoshis: to.Satoshis,
		Script:   *to.LockingScript,
	}
}

type Txo struct {
	Outpoint *Outpoint             `json:"outpoint"`
	Output   *Output               `json:"output"`
	Block    *Block                `json:"block"`
	Spend    *Spend                `json:"spend"`
	Data     map[string]*IndexData `json:"data"`
	Owner    *PKHash               `json:"owner"`
}

// func (t *Txo) EventKey(tag string, e *EventLog) string {
// 	return fmt.Sprintf("evt:%s:%s:%s", tag, e.Label, e.Value)
// }

// func (t *Txo) OwnerKey(tag string, e *EventLog) string {
// 	return fmt.Sprintf("oev:%s:%s:%s:%s", t.Owner, tag, e.Label, e.Value)
// }

func (t *Txo) Score() float64 {
	if t.Spend != nil {
		return -1 * BlockScore(t.Spend.Block)
	} else {
		return BlockScore(t.Block)
	}
}
