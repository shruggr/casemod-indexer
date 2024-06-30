package bitcom

import (
	"bytes"
	"encoding/json"
	"unicode/utf8"

	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/shruggr/casemod-indexer/lib"
)

type Map struct {
	lib.IndexData
	Data map[string]interface{}
}

func (m *Map) Tag() string {
	return "map"
}

func (m *Map) Parse(ic *lib.IndexContext, vout uint32) lib.IndexData {
	txo := ic.Txos[vout]
	if bitcom, ok := txo.Data["bitcom"].(*Bitcom); ok {
		return bitcom.Map
	}
	return nil
}

func ParseMAP(s []byte, idx *int) *Map {
	op, err := lib.ReadOp(s, idx)
	if err != nil {
		return nil
	}
	if string(op.Data) != "SET" {
		return nil
	}
	mp := Map{}
	for {
		prevIdx := *idx
		op, err = lib.ReadOp(s, idx)
		if err != nil || op.OpCode == script.OpRETURN || (op.OpCode == 1 && op.Data[0] == '|') {
			*idx = prevIdx
			break
		}
		opKey := op.Data
		prevIdx = *idx
		op, err = lib.ReadOp(s, idx)
		if err != nil || op.OpCode == script.OpRETURN || (op.OpCode == 1 && op.Data[0] == '|') {
			*idx = prevIdx
			break
		}

		if len(opKey) > 256 || len(op.Data) > 1024 {
			continue
		}

		if !utf8.Valid(opKey) || !utf8.Valid(op.Data) {
			continue
		}

		if len(opKey) == 1 && opKey[0] == 0 {
			opKey = []byte{}
		}
		if len(op.Data) == 1 && op.Data[0] == 0 {
			op.Data = []byte{}
		}

		mp.Data[string(opKey)] = string(op.Data)
		mp.LogEvent(string(opKey), string(op.Data))

	}
	if val, ok := mp.Data["subTypeData"].(string); ok {
		if bytes.Contains([]byte(val), []byte{0}) || bytes.Contains([]byte(val), []byte("\\u0000")) {
			delete(mp.Data, "subTypeData")
		} else {
			var subTypeData map[string]interface{}
			if err := json.Unmarshal([]byte(val), &subTypeData); err == nil {
				mp.Data["subTypeData"] = subTypeData
				for k, v := range subTypeData {
					if sv, ok := v.(string); ok {
						mp.LogEvent("subTypeData."+k, sv)
					}
				}
			}
		}
	}

	return &mp
}
