package mod

import (
	"github.com/shruggr/casemod-indexer/types"
)

type Event struct {
	Id    string
	Value string
}

type Indexer interface {
	Tag() string
	Parse(*types.IndexContext, uint32) *types.IndexData
	Save(*types.IndexContext)
	// FromJSON(j []byte) (*types.IndexData, error)
	// UnmarshalData(target any) error
}

// type BaseIndexer struct{}

// func (i *BaseIndexer) Parse(*types.IndexContext, uint32) *types.IndexData {
// 	return nil
// }

// func (i *BaseIndexer) Save(*types.IndexContext) {}

// func (i *BaseIndexer) FromJSON(j []byte) (*types.IndexData, error) {
// 	return nil, nil
// }

// func (i *BaseIndexer) UnmarshalData(target any) error {
// 	return nil
// }
