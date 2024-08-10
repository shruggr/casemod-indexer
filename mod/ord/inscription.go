package ord

import (
	"bytes"
	"crypto/sha256"
	"regexp"
	"slices"
	"unicode/utf8"

	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/shruggr/casemod-indexer/lib"
	"github.com/shruggr/casemod-indexer/types"
)

var AsciiRegexp = regexp.MustCompile(`^[[:ascii:]]*$`)

type File struct {
	Content []byte         `json:"content"`
	Size    uint32         `json:"size"`
	Type    string         `json:"type"`
	Hash    lib.ByteString `json:"hash"`
}

// type Field struct {
// 	Id    []byte `json:"id"`
// 	Value []byte `json:"value"`
// }

type Inscription struct {
	File   *File           `json:"file"`
	Parent *types.Outpoint `json:"parent"`
}

type InscriptionIndexer struct {
	types.BaseIndexer
}

func (i *InscriptionIndexer) Tag() string {
	return "insc"
}

func (i *InscriptionIndexer) Save(idxCtx *types.IndexContext) {}

func (ii *InscriptionIndexer) Parse(idxCtx *types.IndexContext, vout uint32) *types.IndexData {
	idxData := ParseInscription(idxCtx, vout)
	if idxData != nil {
		ii.IndexInscription(idxCtx, idxData)

	}
	return idxData

}

func ParseInscription(idxCtx *types.IndexContext, vout uint32) *types.IndexData {
	txo := idxCtx.Txos[vout]
	s := idxCtx.Tx.Outputs[vout].LockingScript

	var fromPos *int
	for i := 0; i < len(txo.Script); {
		startI := i
		if op, err := s.ReadOp(&i); err != nil {
			break
		} else if op.Op == script.OpDATA3 && i > 2 &&
			bytes.Equal(op.Data, []byte("ord")) &&
			txo.Script[startI-2] == 0 &&
			txo.Script[startI-1] == script.OpIF {
			fromPos = &i
			break
		}
	}

	if fromPos == nil {
		return nil
	}
	pos := *fromPos
	ins := &Inscription{
		File: &File{},
	}
	idxData := &types.IndexData{Item: ins}

ordLoop:
	for {
		op, err := s.ReadOp(&pos)
		if err != nil || op.Op > script.Op16 {
			return nil
		}

		op2, err := s.ReadOp(&pos)
		if err != nil || op2.Op > script.Op16 {
			return nil
		}

		var field int
		if op.Op > script.OpPUSHDATA4 && op.Op <= script.Op16 {
			field = int(op.Op) - 80
		} else if len(op.Data) == 1 {
			field = int(op.Data[0])
		} else if len(op.Data) > 1 {
			// TODO: Handle bitcom

			// ins.Fields = append(ins.Fields, &Field{
			// 	Id:    op.Data,
			// 	Value: op2.Data,
			// })
			continue
		}

		switch field {
		case 0:
			ins.File.Content = op2.Data
			ins.File.Size = uint32(len(ins.File.Content))
			hash := sha256.Sum256(ins.File.Content)
			ins.File.Hash = hash[:]
			break ordLoop
		case 1:
			if len(op2.Data) < 256 && utf8.Valid(op2.Data) {
				ins.File.Type = string(op2.Data)
			}
		case 3:
			ins.Parent = types.NewOutpointFromBytes(op2.Data)
		default:
			// ins.Fields = append(ins.Fields, &Field{
			// 	Id:    []byte{byte(field)},
			// 	Value: op2.Data,
			// })
		}
	}
	op, err := s.ReadOp(&pos)
	if err != nil || op.Op != script.OpENDIF {
		return nil
	}
	if txo.Owner == nil || len(*txo.Owner) == 0 {
		if len(*s) >= pos+25 && script.NewFromBytes((*s)[pos:pos+25]).IsP2PKH() {
			pkhash := types.PKHash((*s)[pos+3 : pos+23])
			txo.Owner = &pkhash
		} else if len(*s) >= pos+26 &&
			(*s)[pos] == script.OpCODESEPARATOR &&
			script.NewFromBytes((*s)[pos+1:pos+26]).IsP2PKH() {
			pkhash := types.PKHash((*s)[pos+4 : pos+24])
			txo.Owner = &pkhash
		}
	}
	*fromPos = pos

	return idxData
}

func (i *InscriptionIndexer) IndexInscription(idxCtx *types.IndexContext, idxData *types.IndexData) {
	ins := idxData.Item.(*Inscription)
	idxData.Events = append(idxData.Events, &types.Event{
		Id:    "type",
		Value: ins.File.Type,
	})
	if ins.Parent != nil {
		if slices.ContainsFunc(idxCtx.Spends, func(spend *types.Txo) bool {
			if o, ok := spend.Data["origin"]; ok {
				if origin, ok := o.Item.(*Origin); ok {
					return bytes.Equal(origin.Outpoint.Bytes(), ins.Parent.Bytes())
				}
			}
			return false
		}) {
			idxData.Events = append(idxData.Events, &types.Event{
				Id:    "parent",
				Value: ins.Parent.String(),
			})
		} else {
			ins.Parent = nil
		}
	}
	// if ins.File.Size <= 1024 && utf8.Valid(ins.File.Content) && !bytes.Contains(ins.File.Content, []byte{0}) && !bytes.Contains(ins.File.Content, []byte("\\u0000")) {
	// 	mime := strings.ToLower(ins.File.Type)
	// 	if strings.HasPrefix(mime, "application") ||
	// 		strings.HasPrefix(mime, "text") {
	// 		var data json.RawMessage
	// 		err := json.Unmarshal(ins.File.Content, &data)
	// 		if err == nil {
	// 			// TODO:  raise events
	// 		} else if AsciiRegexp.Match(ins.File.Content) {
	// 			text := string(ins.File.Content)
	// 			re := regexp.MustCompile(`\W`)
	// 			words := map[string]struct{}{}
	// 			for _, word := range re.Split(text, -1) {
	// 				if len(word) > 0 {
	// 					word = strings.ToLower(word)
	// 					words[word] = struct{}{}
	// 				}
	// 			}
	// 			if len(words) > 0 {
	// 				for word := range words {
	// 					idxData.Events = append(idxData.Events, &types.Event{
	// 						Id:    "word",
	// 						Value: word,
	// 					})
	// 				}
	// 			}
	// 		}
	// 	}
	// }
}

// func (i *InscriptionIndexer) UnmarshalData(raw []byte) (any, error) {
// 	ins := &Inscription{}
// 	if err := proto.Unmarshal(raw, ins); err != nil {
// 		return nil, err
// 	} else {
// 		return ins, nil
// 	}
// }
