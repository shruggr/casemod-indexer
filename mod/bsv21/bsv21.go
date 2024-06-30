package bsv21

import (
	"encoding/json"

	"github.com/shruggr/casemod-indexer/mod/ord"
	"github.com/shruggr/casemod-indexer/types"
	"google.golang.org/protobuf/proto"
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
	insc := &ord.Inscription{}
	if err := proto.Unmarshal(i.Data, insc); err != nil {
		panic(err)
	}
	if insc.File.Type != "application/bsv-20" {
		return nil
	}
	bsv21 := &Bsv21{}
	if err := json.Unmarshal(insc.File.Content, bsv21); err != nil {
		return nil
	}

	switch bsv21.Op {
	case "deploy+mint":
		if bsv21.Dec > 18 {
			return nil
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
		Events: make([]*types.Event, 0, 2),
	}
	idxData.Events = append(idxData.Events, &types.Event{
		Id:    "id",
		Value: types.NewOutpointFromBytes(bsv21.Id).JsonString(),
	})
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
		} else if err := proto.Unmarshal(b.Data, bsv21); err != nil {
			panic(err)
		} else if bsv21.Status == int32(Valid) {
			if _, ok := spends[bsv21.Id]; !ok {
				spends[bsv21.Id] = []*types.Txo{}
				tokensIn[bsv21.Id] = []*Bsv21{}
				balance[bsv21.Id] = 0
			}
			spends[bsv21.Id] = append(spends[bsv21.Id])
			tokensIn[bsv21.Id] = append(tokensIn[bsv21.Id], bsv21)
			balance[bsv21.Id] += spend.Satoshis
		}
	}

	txos := map[string][]*types.Txo{}
	tokensOut := map[string][]*Bsv21{}
	reasons := map[string]string{}
	for _, txo := range idxCtx.Txos {
		bsv21 := &Bsv21{}
		if b, ok := txo.Data["bsv21"]; !ok {
			continue
		} else if err := proto.Unmarshal(b.Data, bsv21); err != nil {
			panic(err)
		} else if bsv21.Op != "transfer" && bsv21.Op != "burn" {
			continue
		} else if bal, ok := balance[bsv21.Id]; !ok || bal < bsv21.Amt {
			reasons[bsv21.Id] = "insufficient-funds"
		}

		var token *Bsv21
		if spends, ok := spends[bsv21.Id]; ok {
			for i, spend := range spends {
				txo.Data["bsv21"].Deps = append(txo.Data["bsv21"].Deps, spend.Outpoint)
				if i == 0 {
					token = tokensIn[bsv21.Id][i]
				}
			}
		}
		if token != nil {
			bsv21.Sym = token.Sym
			bsv21.Icon = token.Icon
			bsv21.Contract = token.Contract
		}
		if _, ok := tokensOut[bsv21.Id]; !ok {
			tokensOut[bsv21.Id] = []*types.Txo{}
		}
		tokensOut[bsv21.Id] = append(tokensOut[bsv21.Id])
		balance[bsv21.Id] -= txo.Satoshis
	}

	for id, txos := range tokensOut {
		for _, txo := range txos {
			if reason, ok := reasons[id]; ok {
				txo.Events = append(txo.Events, &types.Event{
					Id:    "reason",
					Value: reason,
				})
			}
		}
	}

}
