package types

func (txo *Txo) IndexData(tag string) *IndexData {
	for _, data := range txo.Data {
		if data.Tag == tag {
			return data
		}
	}
	return nil
}
