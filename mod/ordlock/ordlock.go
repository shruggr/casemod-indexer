package ordlock

import (
	"bytes"
	"encoding/hex"
	"math"

	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/shruggr/casemod-indexer/mod/bsv21"
	"github.com/shruggr/casemod-indexer/types"
)

var OrdLockPrefix, _ = hex.DecodeString("2097dfd76851bf465e8f715593b217714858bbe9570ff3bd5e33840a34e20ff0262102ba79df5f8ae7604a9830f03c7933028186aede0675a16f025dc4f8be8eec0382201008ce7480da41702918d1ec8e6849ba32b4d65b1e40dc669c31a1e6306b266c0000")
var OrdLockSuffix, _ = hex.DecodeString("615179547a75537a537a537a0079537a75527a527a7575615579008763567901c161517957795779210ac407f0e4bd44bfc207355a778b046225a7068fc59ee7eda43ad905aadbffc800206c266b30e6a1319c66dc401e5bd6b432ba49688eecd118297041da8074ce081059795679615679aa0079610079517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e01007e81517a75615779567956795679567961537956795479577995939521414136d08c5ed2bf3ba048afe6dcaebafeffffffffffffffffffffffffffffff00517951796151795179970079009f63007952799367007968517a75517a75517a7561527a75517a517951795296a0630079527994527a75517a6853798277527982775379012080517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f517f7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e7c7e01205279947f7754537993527993013051797e527e54797e58797e527e53797e52797e57797e0079517a75517a75517a75517a75517a75517a75517a75517a75517a75517a75517a75517a75517a756100795779ac517a75517a75517a75517a75517a75517a75517a75517a75517a7561517a75517a756169587951797e58797eaa577961007982775179517958947f7551790128947f77517a75517a75618777777777777777777767557951876351795779a9876957795779ac777777777777777767006868")
var Db *pgxpool.Pool
var Rdb *redis.Client

func Initialize(db *pgxpool.Pool, rdb *redis.Client) (err error) {
	Db = db
	Rdb = rdb
	return
}

func Parse(idxCtx *types.IndexContext, vout uint32) *types.IndexData {
	txo := idxCtx.Txos[vout]
	script := idxCtx.Tx.Outputs[txo.Outpoint.Vout].LockingScript
	sCryptPrefixIndex := bytes.Index(*script, OrdLockPrefix)
	if sCryptPrefixIndex == -1 {
		return nil
	}
	ordLockSuffixIndex := bytes.Index(*script, OrdLockSuffix)
	if ordLockSuffixIndex == -1 {
		return nil
	}
	ordLock := (*script)[sCryptPrefixIndex+len(OrdLockPrefix) : ordLockSuffixIndex]
	if ordLockParts, err := ordLock.ParseOps(); err != nil || len(ordLockParts) == 0 {
		return nil
	} else {
		pkhash := types.PKHash(ordLockParts[0].Data)
		payOutput := &transaction.TransactionOutput{}
		listing := &Listing{
			Price:  payOutput.Satoshis,
			PayOut: payOutput.Bytes(),
		}
		if data, ok := txo.Data["bsv21"]; ok {
			bsv21 := data.Item.(*bsv21.Bsv21)
			listing.PricePer = float64(listing.Price) / (float64(bsv21.Amt) / math.Pow10(int(bsv21.Dec)))
		}
		if _, err = payOutput.ReadFrom(bytes.NewReader(ordLockParts[1].Data)); err != nil {
			return nil
		}
		txo.Owner = &pkhash
		return &types.IndexData{
			Events: []*types.Event{
				{
					Id:    "listing",
					Value: txo.Outpoint.String(),
				},
			},
			Item: listing,
		}
	}
}
