package bsv21

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/shruggr/casemod-indexer/mod/ord"
	"github.com/shruggr/casemod-indexer/types"
	"github.com/vmihailenco/msgpack/v5"
)

type Bsv21 struct {
	Id       *types.Outpoint `json:"id,omitempty"`
	Op       string          `json:"op,omitempty"`
	Amt      uint64          `json:"amt,omitempty"`
	Dec      uint32          `json:"dec,omitempty"`
	Sym      string          `json:"sym,omitempty"`
	Icon     *types.Outpoint `json:"icon,omitempty"`
	Contract string          `json:"contract,omitempty"`
	Status   int32           `json:"status,omitempty"`
	Reason   string          `json:"reason,omitempty"`
}

type Bsv21Indexer struct {
	types.BaseIndexer
	InscrptionIndexer *ord.InscriptionIndexer
}

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
	var i *types.IndexData
	if b.InscrptionIndexer != nil {
		i = b.InscrptionIndexer.Parse(idxCtx, vout)
	} else {
		i = txo.Data["insc"]
	}
	if i == nil {
		return nil
	}
	insc := i.Item.(*ord.Inscription)
	if insc == nil {
		return nil
	}
	if insc.File.Type != "application/bsv-20" {
		return nil
	}
	bsv21Insc := map[string]string{}
	if err := json.Unmarshal(insc.File.Content, &bsv21Insc); err != nil {
		return nil
	}
	bsv21 := &Bsv21{
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

		bsv21.Id = txo.Outpoint
		bsv21.Status = 1
	case "transfer", "burn":
		if tokenId, ok := bsv21Insc["id"]; !ok {
			return nil
		} else if bsv21Id, err := types.NewOutpointFromString(tokenId); err != nil {
			log.Println("bsv21: invalid id", tokenId)
			return nil
		} else {
			bsv21.Id = bsv21Id
		}
	default:
		return nil
	}
	if bsv21.Id == nil {
		return nil
	}

	idxData := &types.IndexData{
		Item: bsv21,
		Events: []*types.Event{
			{
				Id:    "id",
				Value: bsv21.Id.String(),
			},
		},
	}
	if bsv21.Contract != "" {
		idxData.Events = append(idxData.Events, &types.Event{
			Id:    "contract",
			Value: bsv21.Contract,
		})
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
			tokenId := bsv21.Id.String()
			if _, ok := spends[tokenId]; !ok {
				spends[tokenId] = []*types.Txo{}
				tokensIn[tokenId] = []*Bsv21{}
				balance[tokenId] = 0
			}
			spends[tokenId] = append(spends[tokenId], spend)
			tokensIn[tokenId] = append(tokensIn[tokenId], bsv21)
			balance[tokenId] += bsv21.Amt
		}
	}

	tokenTxos := map[string][]*types.Txo{}
	reasons := map[string]string{}
	for _, txo := range idxCtx.Txos {
		bsv21 := &Bsv21{}
		var bData *types.IndexData
		var ok bool
		if bData, ok = txo.Data["bsv21"]; !ok {
			continue
		} else if bsv21, ok = bData.Item.(*Bsv21); !ok {
			continue
		}
		tokenId := bsv21.Id.String()
		if bsv21.Op != "transfer" && bsv21.Op != "burn" {
			continue
		} else if bal, ok := balance[tokenId]; !ok || bal < bsv21.Amt {
			reasons[tokenId] = "insufficient-inputs"
		}

		var token *Bsv21
		if tokenSpends, ok := spends[tokenId]; ok {
			// txids := map[string]struct{}{}
			for i, spend := range tokenSpends {
				bData.Deps = append(bData.Deps, spend.Outpoint)
				// txid := spend.Outpoint.Txid.String()
				// if _, ok := txids[txid]; !ok {
				// 	txids[txid] = struct{}{}
				// }
				token = tokensIn[tokenId][i]
			}
		}
		if token != nil {
			bsv21.Sym = token.Sym
			bsv21.Icon = token.Icon
			bsv21.Contract = token.Contract
		}
		if _, ok := tokenTxos[tokenId]; !ok {
			tokenTxos[tokenId] = []*types.Txo{}
		}
		tokenTxos[tokenId] = append(tokenTxos[tokenId], txo)
		balance[tokenId] -= txo.Satoshis
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
					Value: bsv21.Id.String(),
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

func (b *Bsv21Indexer) UnmarshalData(raw []byte) (any, error) {
	bsv21 := &Bsv21{}
	if err := msgpack.Unmarshal(raw, bsv21); err != nil {
		return nil, err
	} else {
		return bsv21, nil
	}
}

func (b *Bsv21Indexer) MarshalData(data any) ([]byte, error) {
	bsv21 := data.(*Bsv21)
	return msgpack.Marshal(bsv21)
}
