package bitcom

import (
	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/shruggr/casemod-indexer/lib"
)

var MAP = "1PuQa7K62MiKCtssSLKy1kh56WWU7MtUR5"
var B = "19HxigV4QyBv3tHpQVcUEQyq1pzZVdoAut"

type Bitcom struct {
	lib.IndexData
	Map    *Map
	B      *BFile
	Sigmas []*Sigma
}

func (b *Bitcom) Tag() string {
	return "bitcom"
}

func (b *Bitcom) Parse(txCtx *lib.IndexContext, vout uint32) lib.IndexData {
	s := *txCtx.Tx.Outputs[vout].LockingScript
	var opReturn int
	bitcom := &Bitcom{}
	for i := 0; i < len(s); {
		startI := i
		op, err := lib.ReadOp(s, &i)
		if err != nil {
			break
		}
		switch op.OpCode {
		case script.OpRETURN:
			if opReturn == 0 {
				opReturn = startI
			}
			ParseBitcom(txCtx, vout, &i, bitcom)
		case script.OpDATA1:
			if op.Data[0] == '|' && opReturn > 0 {
				ParseBitcom(txCtx, vout, &i, bitcom)
			}
		}
	}
	return bitcom
}

func ParseBitcom(txCtx *lib.IndexContext, vout uint32, idx *int, bitcom *Bitcom) {
	txo := txCtx.Txos[vout]
	startIdx := *idx
	op, err := lib.ReadOp(txo.Script, idx)
	if err != nil {
		return
	}

	switch string(op.Data) {
	case MAP:
		if bitcom.Map == nil {
			bitcom.Map = ParseMAP(txo.Script, idx)
		}
	case B:
		if bitcom.B != nil {
			bitcom.B = ParseB(txo.Script, idx)
		}
	case "SIGMA":
		sigma := ParseSigma(txCtx, vout, startIdx, idx)
		if sigma != nil {
			bitcom.Sigmas = append(bitcom.Sigmas, sigma)
		}
	default:
		*idx--
	}
}
