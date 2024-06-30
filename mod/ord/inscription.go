package ord

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/shruggr/casemod-indexer/lib"
	"github.com/shruggr/casemod-indexer/types"
	"google.golang.org/protobuf/proto"
)

var AsciiRegexp = regexp.MustCompile(`^[[:ascii:]]*$`)

type InscriptionIndexer struct{}

func (i *InscriptionIndexer) Tag() string {
	return "ord"
}

func (i *InscriptionIndexer) Save(idxCtx *types.IndexContext) {}

func (i *InscriptionIndexer) Parse(idxCtx *types.IndexContext, vout uint32) *types.IndexData {
	txo := idxCtx.Txos[vout]
	for i := 0; i < len(txo.Script); {
		startI := i
		if op, err := lib.ReadOp(txo.Script, &i); err != nil {
			break
		} else if op.OpCode == script.OpDATA3 && i > 2 &&
			bytes.Equal(op.Data, []byte("ord")) &&
			txo.Script[startI-2] == 0 &&
			txo.Script[startI-1] == script.OpIF {
			return ParseInscription(idxCtx, vout, txo.Script, &i)
		}
	}
	return nil
}

// func (i *InscriptionIndexer) UnmarshalData(target interface{}) error {

// }

func ParseInscription(txCtx *types.IndexContext, vout uint32, s []byte, fromPos *int) *types.IndexData {
	txo := txCtx.Txos[vout]
	pos := *fromPos
	idxData := &types.IndexData{
		Events: []*types.Event{},
		Deps:   []*types.Outpoint{},
	}

	ins := &Inscription{
		File: &File{},
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
			ins.Fields = append(ins.Fields, &Field{
				Id:    op.Data,
				Value: op2.Data,
			})
			continue
		}

		switch field {
		case 0:
			ins.File.Content = op2.Data
			break ordLoop
		case 1:
			if len(op2.Data) < 256 && utf8.Valid(op2.Data) {
				ins.File.Type = string(op2.Data)
				idxData.Events = append(idxData.Events, &types.Event{
					Id:    "type",
					Value: ins.File.Type,
				})
			}
		case 3:
			if parent, err := types.NewOutpointFromBytes(op2.Data); err == nil {
				if slices.ContainsFunc(txCtx.Spends, func(spend *types.Txo) bool {
					if o, ok := spend.Data["origin"]; ok {
						origin := &Origin{}
						if err := proto.Unmarshal(o.Data, origin); err != nil {
							panic(err)
						}
						return bytes.Equal(spend.Outpoint.Bytes(), parent.Bytes())
					}
					return false
				}) {
					ins.Parent = parent.Bytes()
					idxData.Events = append(idxData.Events, &types.Event{
						Id:    "parent",
						Value: parent.JsonString(),
					})
				}
			}
		default:
			ins.Fields = append(ins.Fields, &Field{
				Id:    []byte{byte(field)},
				Value: op2.Data,
			})
		}
	}
	op, err := lib.ReadOp(s, &pos)
	if err != nil || op.OpCode != script.OpENDIF {
		return nil
	}
	if len(txo.Owner) == 0 {
		if len(s) >= pos+25 && script.NewFromBytes(s[pos:pos+25]).IsP2PKH() {
			pkhash := lib.PKHash(s[pos+3 : pos+23])
			txo.Owner = pkhash
		} else if len(s) >= pos+26 &&
			s[pos] == script.OpCODESEPARATOR &&
			script.NewFromBytes(s[pos+1:pos+26]).IsP2PKH() {
			pkhash := lib.PKHash(s[pos+4 : pos+24])
			txo.Owner = pkhash
		}
	}
	*fromPos = pos

	ins.File.Size = uint32(len(ins.File.Content))
	hash := sha256.Sum256(ins.File.Content)
	ins.File.Hash = hash[:]
	if ins.File.Size <= 1024 && utf8.Valid(ins.File.Content) && !bytes.Contains(ins.File.Content, []byte{0}) && !bytes.Contains(ins.File.Content, []byte("\\u0000")) {
		mime := strings.ToLower(ins.File.Type)
		if strings.HasPrefix(mime, "application") ||
			strings.HasPrefix(mime, "text") {
			var data json.RawMessage
			err := json.Unmarshal(ins.File.Content, &data)
			if err == nil {
				// TODO:  raise events
			} else if AsciiRegexp.Match(ins.File.Content) {
				text := string(ins.File.Content)
				re := regexp.MustCompile(`\W`)
				words := map[string]struct{}{}
				for _, word := range re.Split(text, -1) {
					if len(word) > 0 {
						word = strings.ToLower(word)
						words[word] = struct{}{}
					}
				}
				if len(words) > 0 {
					for word := range words {
						idxData.Events = append(idxData.Events, &types.Event{
							Id:    "word",
							Value: word,
						})
					}
				}
			}
		}
	}
	idxData.Data, _ = proto.Marshal(ins)
	return idxData
}
