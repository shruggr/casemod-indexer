package bitcom

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"strconv"

	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/bitcoinschema/go-bitcoin"
	"github.com/shruggr/casemod-indexer/lib"
)

type Sigmas struct {
	lib.IndexData
	Items []*Sigma `json:"items"`
}

func (s *Sigmas) Tag() string {
	return "sigma"
}

func (s *Sigmas) Parse(txCtx *lib.IndexContext, vout uint32) lib.IndexData {
	txo := txCtx.Txos[vout]
	if bitcom, ok := txo.Data["bitcom"].(*Bitcom); ok {
		if len(bitcom.Sigmas) > 0 {
			sigmas := &Sigmas{Items: bitcom.Sigmas}
			addresses := make(map[string]struct{})
			for _, sigma := range sigmas.Items {
				addresses[sigma.Address] = struct{}{}
			}
			for address := range addresses {
				sigmas.LogEvent("address", address)
			}
			return sigmas
		}
	}
	return nil
}

type Sigma struct {
	lib.IndexData
	Algorithm string `json:"algorithm"`
	Address   string `json:"address"`
	Signature []byte `json:"signature"`
	Vin       uint32 `json:"vin"`
}

func (s *Sigma) Tag() string {
	return "sigma"
}

func ParseSigma(txCtx *lib.IndexContext, vout uint32, startIdx int, idx *int) (sigma *Sigma) {
	s := *txCtx.Tx.Outputs[vout].LockingScript
	sigma = &Sigma{}
	for i := 0; i < 4; i++ {
		prevIdx := *idx
		op, err := lib.ReadOp(s, idx)
		if err != nil || op.OpCode == script.OpRETURN || (op.OpCode == 1 && op.Data[0] == '|') {
			*idx = prevIdx
			break
		}

		switch i {
		case 0:
			sigma.Algorithm = string(op.Data)
		case 1:
			sigma.Address = string(op.Data)
		case 2:
			sigma.Signature = op.Data
		case 3:
			vin, err := strconv.ParseInt(string(op.Data), 10, 32)
			if err == nil {
				sigma.Vin = uint32(vin)
			}
		}
	}

	outpoint := txCtx.Tx.Inputs[sigma.Vin].SourceTXID
	outpoint = binary.LittleEndian.AppendUint32(outpoint, txCtx.Tx.Inputs[sigma.Vin].SourceTxOutIndex)
	inputHash := sha256.Sum256(outpoint)
	var scriptBuf []byte
	if s[startIdx-1] == script.OpRETURN {
		scriptBuf = s[:startIdx-1]
	} else if s[startIdx-1] == '|' {
		scriptBuf = s[:startIdx-2]
	} else {
		return nil
	}
	outputHash := sha256.Sum256(scriptBuf)
	msgHash := sha256.Sum256(append(inputHash[:], outputHash[:]...))
	if err := bitcoin.VerifyMessage(sigma.Address,
		base64.StdEncoding.EncodeToString(sigma.Signature),
		string(msgHash[:]),
	); err != nil {
		return nil
	}
	return
}
