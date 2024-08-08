package types

import "math"

type Txo struct {
	RawTxo
	Data map[string]*IndexData
}

func (t *Txo) FromRawTxo(raw *RawTxo, indexers []Indexer) *Txo {
	t.RawTxo = RawTxo{
		Outpoint: raw.Outpoint,
		Satoshis: raw.Satoshis,
		Script:   raw.Script,
		Block:    raw.Block,
		Spend:    raw.Spend,
		RawData:  raw.RawData,
		Owner:    raw.Owner,
		Events:   raw.Events,
	}
	t.Data = make(map[string]*IndexData)
	for _, indexer := range indexers {
		rawData := raw.RawData[indexer.Tag()]
		if rawData == nil {
			continue
		} else if item, err := indexer.UnmarshalData(raw.RawData[indexer.Tag()].Data); err != nil {
			panic(err)
		} else if item != nil {
			t.Data[indexer.Tag()] = &IndexData{
				RawData: RawData{
					Tag:    rawData.Tag,
					Data:   rawData.Data,
					Events: rawData.Events,
					Deps:   rawData.Deps,
				},
				Item: item,
			}
		}
	}
	return t
}

func (t *Txo) Score() (score float64) {
	if t.Spend != nil {
		score = 1 + float64(t.Spend.Block.Height)*math.Pow(2, -32)
	} else {
		score = 0 + float64(t.Block.Height)/math.Pow(2, -32)
	}

}
