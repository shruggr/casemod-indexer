package lib

import (
	"github.com/shruggr/casemod-indexer/types"
)

type Event struct {
	Id    string
	Value string
}

type Indexer interface {
	Tag() string
	Parse(*types.IndexContext, uint32) *models.IndexData
	Save(*types.IndexContext)
	FromJSON(j []byte) (*types.IndexData, error)
}

// type IndexData interface {
// 	Emit(string, string)
// 	Events() map[string][]string
// 	Deps() []ByteString
// 	// Serialize() []byte
// }

// type IndexData struct {
// 	Events []Event
// 	Deps   []ByteString
// }

// func (i *IndexData) Emit(id string, value string) {
// 	if i.Events == nil {
// 		i.Events = make(map[string][]string)
// 	}
// 	i.Events[id] = append(i.Events[id], value)
// }

// func (i *IndexerDataBase) AddDependencies(txids ...ByteString) {
// 	i.deps = append(i.deps, txids...)
// }

// func (i *IndexData) Events() map[string][]string {
// 	return i.events
// }

// func (i *IndexData) Deps() []ByteString {
// 	return i.deps
// }

// func (i *IndexerDataBase) Bind(obj *IndexerData) error {
// 	if b, err := json.Marshal(i); err != nil {
// 		return err
// 	} else {
// 		return json.Unmarshal(b, obj)
// 	}
// }

// func (i *IndexerDataBase) Serialize() []byte {
// 	b, _ := json.Marshal(i)
// 	return b
// }

// func (i *IndexerDataBase) Deserialize(b []byte) (IndexerData, error) {
// 	err := json.Unmarshal(b, i)
// 	return i, err
// }
