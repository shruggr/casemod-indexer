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

func EventKey(tag string, e *types.EventLog) string {
	return fmt.Sprintf("evt:%s:%s:%s", tag, e.Label, e.Value)
}

func OwnerEventKey(owner string, tag string, e *types.EventLog) string {
	return fmt.Sprintf("oev:%s:%s:%s:%s", owner, tag, e.Label, e.Value)
}

// func EventMember(value string, outpoint *types.Outpoint) string {
// 	return fmt.Sprintf("%s:%s", value, outpoint.String())
// }

func LogKey(indexer string) string {
	return fmt.Sprintf("log:%s", indexer)
}

func BlockHeightKey(height uint32) string {
	return fmt.Sprintf("%07d", height)
}

var BlockKey = "block"
var BlockIdKey = "blockid"
var ProgressKey = "progress"
var RawtxKey = "rawtx"
var ProofKey = "proof"

// var RawtxKey = "rawtx"

/*
 * Txn Keys
 */
// var TxVersionSuffix = "ver"
// var TxInsSuffix = "ins"
// var TxOutsSuffix = "outs"
// var TxLocktimeSuffix = "nlt"

// func TxKey(txid string) string {
// 	return fmt.Sprintf("txn:%s:raw", txid)
// }

// func TxOutsKey(txid string) string {
// 	return fmt.Sprintf("txn:%s:outs", txid)
// }

// func TxOutMember(vout uint32) string {
// 	return fmt.Sprintf("%s:%08x", TxOutsMember, vout)
// }

var TxStatusKey = "txs"

/*
 * Txo Keys
 */
var TxoPrefix = "txo:"

func TxoKey(outpoint *types.Outpoint) string {
	return TxoPrefix + outpoint.String()
}

func TxoTxidKey(txid string) string {
	return fmt.Sprintf("%s%s*", TxoPrefix, txid)
}

// type TxoMemeber string
// var (
// 	SpendMember = TxoMemeber("spn")
// )
// func SpendMember(vout uint32) string {
// 	return fmt.Sprintf("spn", vout)
// }

var OutputMember = "out"
var SpendMember = "spn"
var DepSuffix = "dep"
var EventSuffix = "evt"
var DataSuffix = "dat"

func DepMember(tag string) string {
	return fmt.Sprintf("%s:%s", tag, DepSuffix)
}

func EventMember(tag string) string {
	return fmt.Sprintf("%s:%s", tag, EventSuffix)
}

func DataMember(tag string) string {
	return fmt.Sprintf("%s:%s", tag, DataSuffix)
}

func TxoEventKey(tag string, e *types.EventLog) string {
	return fmt.Sprintf("evt:%s:%s:%s", tag, e.Label, e.Value)
}

func TxoOwnerKey(owner *types.PKHash, tag string, e *types.EventLog) string {
	return fmt.Sprintf("oev:%s:%s:%s:%s", owner.String(), tag, e.Label, e.Value)
}
