package bitcom

import (
	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/shruggr/casemod-indexer/lib"
)

var MAP = "1PuQa7K62MiKCtssSLKy1kh56WWU7MtUR5"
var B = "19HxigV4QyBv3tHpQVcUEQyq1pzZVdoAut"

func ParseScript(txo *lib.Txo) {
	vout := txo.Outpoint.Vout()
	script := *txo.Tx.Outputs[vout].LockingScript

	var opReturn int
	for i := 0; i < len(script); {
		startI := i
		op, err := lib.ReadOp(script, &i)
		if err != nil {
			break
		}
		switch op.OpCode {
		case script.OpRETURN:
			if opReturn == 0 {
				opReturn = startI
			}
			ParseBitcom(txo.Tx, txo, &i)
		case script.OpDATA1:
			if op.Data[0] == '|' && opReturn > 0 {
				ParseBitcom(txo.Tx, txo, &i)
			}
		}
	}
}

func ParseBitcom(tx *transaction.Transaction, txo *lib.Txo, idx *int) {
	startIdx := *idx
	op, err := lib.ReadOp(txo.Script, idx)
	if err != nil {
		return
	}
	var bitcom lib.IIndexable
	switch string(op.Data) {
	case MAP:
		mod := ParseMAP(txo.Script, idx)
		bitcom = mod
	case B:
		mod := ParseB(txo.Script, idx)
		bitcom = mod
	case "SIGMA":
		sigma := ParseSigma(tx, txo.Script, startIdx, idx)
		sigmas := txo.Data["sigma"].(*Sigmas)
		if sigmas == nil {
			sigmas = &Sigmas{}
		}
		*sigmas = append(*sigmas, sigma)
		bitcom = sigmas
	default:
		*idx--
	}
	if bitcom != nil {
		txo.AddData(bitcom.Tag(), bitcom)
	}
}
