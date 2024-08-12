package types

import (
	"math"
	"time"

	"github.com/bitcoin-sv/go-sdk/util"
)

type Block struct {
	Height uint32          `json:"height,omitempty"`
	Idx    uint64          `json:"idx,omitempty"`
	Hash   util.ByteString `json:"hash,omitempty"`
}

// Negative numbers are spent, positive numbers are unspent
// Mempool transactions are < 0x1FFFFF00000000 if spent and > 0x1FFFFF00000000 if unspent
// Mined transactions are block height << 31 + block index
func BlockScore(b *Block) float64 {
	if b == nil {
		return 0x1FFFFF + float64(time.Now().Unix())*math.Pow(2, -31)
	}
	return float64(b.Height) + float64(b.Idx)*math.Pow(2, -31)
}

func ParseBlockScore(score float64) *Block {
	score = math.Abs(score)
	if score > 0x1FFFFF {
		return nil
	}
	height := uint32(score)
	return &Block{
		Height: height,
		Idx:    uint64((score - float64(height)) * math.Pow(2, 31)),
	}
}

type Spend struct {
	Txid  util.ByteString `json:"txid"`
	Vin   uint32          `json:"vin"`
	Block *Block          `json:"block"`
}
