package ord

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/fxamacker/cbor"
	"github.com/shruggr/casemod-indexer/lib"
	"github.com/shruggr/casemod-indexer/mod/bitcom"
	"github.com/shruggr/casemod-indexer/types"
)

var AsciiRegexp = regexp.MustCompile(`^[[:ascii:]]*$`)

type Inscription struct {
	Json      json.RawMessage        `json:"json,omitempty"`
	Text      string                 `json:"text,omitempty"`
	File      *lib.File              `json:"file,omitempty"`
	Pointer   *uint64                `json:"pointer,omitempty"`
	Parent    *types.Outpoint        `json:"parent,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Metaproto []byte                 `json:"metaproto,omitempty"`
	Fields    map[string]interface{} `json:"-"`
}

func (i *Inscription) Tag() string {
	return "ord"
}

func (i *Inscription) Parse(idxCtx *types.IndexContext, vout uint32) *types.IndexData {
	txo := idxCtx.Txos[vout]
	// s := *idxCtx.Tx.Outputs[vout].LockingScript

	idxData := &types.IndexData{
		Events: []*types.Event{},
		Deps:   []*types.Outpoint{},
	}
	for i := 0; i < len(txo.Script); {
		startI := i
		if op, err := lib.ReadOp(txo.Script, &i); err != nil {
			break
		} else if op.OpCode == script.OpDATA3 && i > 2 && bytes.Equal(op.Data, []byte("ord")) && s[startI-2] == 0 && s[startI-1] == script.OpIF {
			insc := ParseInscription(idxCtx, vout, txo.Script, &i)
			idxData.Data = 
		}
	}
	return nil
}

func ParseInscription(txCtx *types.IndexContext, vout uint32, s []byte, fromPos *int) *Inscription {
	txo := txCtx.Txos[vout]
	pos := *fromPos
	ins := &Inscription{
		File: &lib.File{},
	}

ordLoop:
	for {
		op, err := lib.ReadOp(s, &pos)
		if err != nil || op.OpCode > script.Op16 {
			return nil
		}

		op2, err := lib.ReadOp(s, &pos)
		if err != nil || op2.OpCode > script.Op16 {
			return nil
		}

		var field int
		if op.OpCode > script.OpPUSHDATA4 && op.OpCode <= script.Op16 {
			field = int(op.OpCode) - 80
		} else if op.Len == 1 {
			field = int(op.Data[0])
		} else if op.Len > 1 {
			if ins.Fields == nil {
				ins.Fields = map[string]interface{}{}
			}
			if op.Len <= 64 && utf8.Valid(op.Data) && !bytes.Contains(op.Data, []byte{0}) && !bytes.Contains(op.Data, []byte("\\u0000")) {
				ins.Fields[string(op.Data)] = op2.Data
			}
			if string(op.Data) == bitcom.MAP {
				script := script.NewFromBytes(op2.Data)
				pos := 0
				md := bitcom.ParseMAP(*script, &pos)
				if md != nil {

					txo.AddData("map", md)
				}
			}
			continue
		}

		switch field {
		case 0:
			ins.File.Content = op2.Data
			break ordLoop
		case 1:
			if len(op2.Data) < 256 && utf8.Valid(op2.Data) {
				ins.File.Type = string(op2.Data)
				ins.LogEvent("type", ins.File.Type)
			}
		case 2:
			pointer := binary.LittleEndian.Uint64(op2.Data)
			ins.Pointer = &pointer
		case 3:
			if parent, err := lib.NewOutpointFromTxOutpoint(op2.Data); err == nil {
				if slices.ContainsFunc(txCtx.Spends, func(spend *lib.Txo) bool {
					origin := spend.Data["origin"]
					return origin != nil && bytes.Equal(*spend.Outpoint, *parent)
				}) {
					ins.Parent = parent
					ins.LogEvent("parent", parent.String())
				}
			}
		case 5:
			md := &bitcom.Map{}
			if err := cbor.Unmarshal(op2.Data, md.Data); err == nil {
				ins.Metadata = md.Data
			}
		case 7:
			ins.Metaproto = op2.Data
		case 9:
			ins.File.Encoding = string(op2.Data)
		default:
			if ins.Fields == nil {
				ins.Fields = map[string]interface{}{}
			}

		}
	}
	op, err := lib.ReadOp(s, &pos)
	if err != nil || op.OpCode != script.OpENDIF {
		return nil
	}
	*fromPos = pos

	ins.File.Size = uint32(len(ins.File.Content))
	hash := sha256.Sum256(ins.File.Content)
	ins.File.Hash = hash[:]
	insType := "file"
	if ins.File.Size <= 1024 && utf8.Valid(ins.File.Content) && !bytes.Contains(ins.File.Content, []byte{0}) && !bytes.Contains(ins.File.Content, []byte("\\u0000")) {
		mime := strings.ToLower(ins.File.Type)
		if strings.HasPrefix(mime, "application") ||
			strings.HasPrefix(mime, "text") {

			var data json.RawMessage
			err := json.Unmarshal(ins.File.Content, &data)
			if err == nil {
				insType = "json"
				ins.Json = data
				// bsv20, _ = ParseBsv20Inscription(ins.File, txo)
			} else if AsciiRegexp.Match(ins.File.Content) {
				if insType == "file" {
					insType = "text"
				}
				ins.Text = string(ins.File.Content)
				re := regexp.MustCompile(`\W`)
				words := map[string]struct{}{}
				for _, word := range re.Split(ins.Text, -1) {
					if len(word) > 0 {
						word = strings.ToLower(word)
						words[word] = struct{}{}
					}
				}
				if len(words) > 0 {
					for word := range words {
						ins.LogEvent("word", word)
					}
				}
			}
		}
	}

	if txo.Owner == nil {
		if len(s) >= pos+25 && script.NewFromBytes(s[pos:pos+25]).IsP2PKH() {
			pkhash := lib.PKHash(s[pos+3 : pos+23])
			txo.Owner = &pkhash
		} else if len(s) >= pos+26 &&
			s[pos] == script.OpCODESEPARATOR &&
			script.NewFromBytes(s[pos+1:pos+26]).IsP2PKH() {
			pkhash := lib.PKHash(s[pos+4 : pos+24])
			txo.Owner = &pkhash
		}
	}

	return ins
}
