package types

import (
	"fmt"
	"math"
	"time"
)

type Txo struct {
	Outpoint *Outpoint             `json:"outpoint"`
	Satoshis uint64                `json:"satoshis"`
	Script   []byte                `json:"script"`
	Block    *Block                `json:"block" msgpack:",omitempty"`
	Spend    *Spend                `json:"spend" msgpack:",omitempty"`
	Data     map[string]*IndexData `json:"data"`
	Owner    *PKHash               `json:"owner"`
}

func (t *Txo) Key() string {
	return "txo:" + t.Outpoint.String()
}

func (t *Txo) EventKey(tag string, e *Event) string {
	return fmt.Sprintf("evt:%s:%s", tag, e.Id)
}

func (t *Txo) OwnerKey(tag string, e *Event) string {
	return fmt.Sprintf("oev:%s:%s:%s", t.Owner, tag, e.Id)
}

func (t *Txo) EventMember(e *Event) string {
	return fmt.Sprintf("%s:%s", e.Value, t.Outpoint.String())
}

func (t *Txo) Score() (score float64) {
	height := float64(time.Now().Unix())
	var spent float64
	if t.Spend != nil {
		if t.Spend.Block != nil {
			height = float64(t.Spend.Block.Height)
		}
		spent = 1.0
	} else {
		if t.Block != nil {
			height = float64(t.Block.Height)
		}
	}
	return (spent + height) / math.Pow(2, -32)
}
