package db

import (
	"fmt"

	"github.com/shruggr/casemod-indexer/types"
)

func QueueKey(indexer string) string {
	return fmt.Sprintf("que:%s:", indexer)
}

func OwnerKey(owner *types.PKHash) string {
	return fmt.Sprintf("own:%s", owner.String())
}

func LogKey(indexer string) string {
	return fmt.Sprintf("log:%s", indexer)
}

func DepKey(tag string) string {
	return fmt.Sprintf("dep:%s", tag)
}

func BlockHeightKey(height uint32) string {
	return fmt.Sprintf("%07d", height)
}

var BlockKey = "block"
var BlockIdKey = "blockid"
var ProgressKey = "progress"
var ProofKey = "proof"
var RawtxKey = "rawtx"
var SpendKey = "spend"
var TxoKey = "data"
