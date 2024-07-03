package bsv21

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/shruggr/casemod-indexer/mod/ord"
	"github.com/shruggr/casemod-indexer/types"
	"google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

type Bsv21Indexer struct{}

type Bsv21Status int32

const (
	Invalid Bsv21Status = -1
	Pending Bsv21Status = 0
	Valid   Bsv21Status = 1
)

func (b *Bsv21Indexer) Tag() string {
	return "bsv21"
}

func (b *Bsv21Indexer) Parse(idxCtx *types.IndexContext, vout uint32) *types.IndexData {
	txo := idxCtx.Txos[vout]
	i, ok := txo.Data["insc"]
	if !ok {
		return nil
	}
	insc := i.Item.(*ord.Inscription)
	if insc.File.Type != "application/bsv-20" {
		return nil
	}
	bsv21Insc := map[string]string{}
	if err := json.Unmarshal(insc.File.Content, &bsv21Insc); err != nil {
		return nil
	}
	bsv21 := &Bsv21{
		Id:       bsv21Insc["id"],
		Op:       bsv21Insc["op"],
		Sym:      bsv21Insc["sym"],
		Contract: bsv21Insc["contract"],
	}
	if amtStr, ok := bsv21Insc["amt"]; ok {
		if amt, err := strconv.ParseUint(amtStr, 10, 64); err != nil {
			log.Println("bsv21: invalid amount", bsv21Insc["amt"])
			return nil
		} else {
			bsv21.Amt = amt
		}
	}

	switch bsv21.Op {
	case "deploy+mint":
		if decStr, ok := bsv21Insc["dec"]; ok {
			if dec, err := strconv.ParseUint(decStr, 10, 8); err != nil {
				log.Println("bsv21: invalid dec", bsv21Insc["dec"])
				return nil
			} else if dec > 18 {
				return nil
			} else {
				bsv21.Dec = uint32(dec)
			}
		}

		bsv21.Id = txo.Outpoint.JsonString()
		bsv21.Status = 1
	case "transfer", "burn":
	default:
		return nil
	}
	if len(bsv21.Id) == 0 {
		return nil
	}

	idxData := &types.IndexData{
		Item: bsv21,
	}
	return idxData
}

func (b *Bsv21Indexer) Save(idxCtx *types.IndexContext) {
	balance := map[string]uint64{}
	spends := map[string][]*types.Txo{}
	tokensIn := map[string][]*Bsv21{}
	for _, spend := range idxCtx.Spends {
		bsv21 := &Bsv21{}
		if b, ok := spend.Data["bsv21"]; !ok {
			continue
		} else {
			bsv21 = b.Item.(*Bsv21)
		}
		if bsv21.Status == int32(Valid) {
			if _, ok := spends[bsv21.Id]; !ok {
				spends[bsv21.Id] = []*types.Txo{}
				tokensIn[bsv21.Id] = []*Bsv21{}
				balance[bsv21.Id] = 0
			}
			spends[bsv21.Id] = append(spends[bsv21.Id], spend)
			tokensIn[bsv21.Id] = append(tokensIn[bsv21.Id], bsv21)
			balance[bsv21.Id] += bsv21.Amt
		}
	}

	tokenTxos := map[string][]*types.Txo{}
	reasons := map[string]string{}
	for _, txo := range idxCtx.Txos {
		bsv21 := &Bsv21{}
		if b, ok := txo.Data["bsv21"]; !ok {
			continue
		} else {
			bsv21 = b.Item.(*Bsv21)
		}
		if bsv21.Op != "transfer" && bsv21.Op != "burn" {
			continue
		} else if bal, ok := balance[bsv21.Id]; !ok || bal < bsv21.Amt {
			reasons[bsv21.Id] = "insufficient-inputs"
		}

		var token *Bsv21
		if tokenSpends, ok := spends[bsv21.Id]; ok {
			for i, spend := range tokenSpends {
				txo.Data["bsv21"].Deps = append(txo.Data["bsv21"].Deps, spend.Outpoint)
				if token == nil {
					token = tokensIn[bsv21.Id][i]
				}
			}
		}
		if token != nil {
			bsv21.Sym = token.Sym
			bsv21.Icon = token.Icon
			bsv21.Contract = token.Contract
		}
		if _, ok := tokenTxos[bsv21.Id]; !ok {
			tokenTxos[bsv21.Id] = []*types.Txo{}
		}
		tokenTxos[bsv21.Id] = append(tokenTxos[bsv21.Id], txo)
		balance[bsv21.Id] -= txo.Satoshis
	}

	for id, txos := range tokenTxos {
		reason := reasons[id]
		for _, txo := range txos {
			idxData := txo.Data["bsv21"]
			bsv21 := idxData.Item.(*Bsv21)
			if reason != "" {
				bsv21.Status = int32(Invalid)
				bsv21.Reason = reason
				idxData.Events = append(idxData.Events, &types.Event{
					Id:    "reason",
					Value: reason,
				})
			} else {
				idxData.Events = append(idxData.Events, &types.Event{
					Id:    "id",
					Value: bsv21.Id,
				})
				if bsv21.Contract != "" {
					idxData.Events = append(idxData.Events, &types.Event{
						Id:    "contract",
						Value: bsv21.Contract,
					})
				}
				bsv21.Status = int32(Valid)
			}
		}
	}
}

func (b *Bsv21Indexer) UnmarshalData(raw []byte) (protoreflect.ProtoMessage, error) {
	bsv21 := &Bsv21{}
	if err := proto.Unmarshal(raw, bsv21); err != nil {
		return nil, err
	} else {
		return bsv21, nil
	}
}
