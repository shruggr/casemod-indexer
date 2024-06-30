package bitcom

import (
	"crypto/sha256"

	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/shruggr/casemod-indexer/lib"
)

type BFile struct {
	lib.File
	lib.IndexData
}

func (b *BFile) Tag() string {
	return "b"
}

func (b *BFile) Parse(ic *lib.IndexContext, vout uint32) lib.IndexData {
	txo := ic.Txos[vout]
	if bitcom, ok := txo.Data["bitcom"].(*Bitcom); ok {
		return bitcom.B
	}
	return nil
}

func ParseB(s []byte, idx *int) (b *BFile) {
	b = &BFile{}
	for i := 0; i < 4; i++ {
		prevIdx := *idx
		op, err := lib.ReadOp(s, idx)
		if err != nil || op.OpCode == script.OpRETURN || (op.OpCode == 1 && op.Data[0] == '|') {
			*idx = prevIdx
			break
		}

		switch i {
		case 0:
			b.Content = op.Data
		case 1:
			b.Type = string(op.Data)
		case 2:
			b.Encoding = string(op.Data)
		case 3:
			b.Name = string(op.Data)
		}
	}
	hash := sha256.Sum256(b.Content)
	b.Size = uint32(len(b.Content))
	b.Hash = hash[:]
	return
}
