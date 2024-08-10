package ord

import "github.com/shruggr/casemod-indexer/types"

type Origin struct {
	Outpoint *types.Outpoint   `json:"outpoint,omitempty"`
	Nonce    uint32            `json:"nonce,omitempty"`
	Map      map[string]string `json:"map,omitempty"`
}
