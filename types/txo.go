package types

type Txo struct {
	RawTxo
	Data map[string]*IndexData
}

func (t *Txo) FromRawTxo(raw *RawTxo, indexers []Indexer) {
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
		if item, err := indexer.UnmarshalData(raw.RawData[indexer.Tag()].Data); err != nil {
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

}
