package types

import "github.com/bitcoin-sv/go-sdk/util"

type Block struct {
	Height uint32          `json:"height,omitempty"`
	Idx    uint64          `json:"idx,omitempty"`
	Hash   util.ByteString `json:"hash,omitempty"`
}

type Spend struct {
	Txid  util.ByteString `json:"txid"`
	Vin   uint32          `json:"vin"`
	Block *Block          `json:"block"`
}
