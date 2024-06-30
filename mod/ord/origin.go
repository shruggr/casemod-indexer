package ord

import (
	"context"
	"encoding/hex"
	"log"

	"github.com/shruggr/casemod-indexer/db"
	"github.com/shruggr/casemod-indexer/mod"
	store "github.com/shruggr/casemod-indexer/txostore"
	"github.com/shruggr/casemod-indexer/types"
	"google.golang.org/protobuf/proto"
)

const MAX_DEPTH = 1024

type OriginIndexer struct{}

const TAG = "origin"

func (o *OriginIndexer) Tag() string {
	return TAG
}

// func (o *OriginIndexer) UnmarshalData(txo *types.Txo) (*Origin, error) {
// 	origin := &Origin{}
// 	data := txo.IndexData(o.Tag())
// 	if data == nil {
// 		return nil, nil
// 	} else if err := proto.Unmarshal(data.Data, origin); err != nil {
// 		return nil, err
// 	} else {
// 		return origin, nil
// 	}
// }

func (o *OriginIndexer) Parse(txCtx *types.IndexContext, vout uint32) *types.IndexData {
	txo := txCtx.Txos[vout]
	if txo.Satoshis != 1 {
		return nil
	}

	origin := calcOrigin(txCtx, vout, 0)
	if origin == nil {
		return nil
	}
	idxData := &types.IndexData{}
	var err error
	if idxData.Data, err = proto.Marshal(origin); err != nil {
		panic(err)
	}
	idxData.Events = append(idxData.Events, &types.Event{
		Id:    "outpoint",
		Value: txo.Outpoint.JsonString(),
	})
	return idxData
}

func (o *OriginIndexer) Save(txCtx *types.IndexContext) {}

func calcOrigin(txCtx *types.IndexContext, vout uint32, depth uint32) (origin *Origin) {
	if depth > MAX_DEPTH {
		return nil
	}
	outSat := uint64(0)
	for i := uint32(0); i < vout; i++ {
		outSat += txCtx.Txos[i].Satoshis
	}
	txo := txCtx.Txos[vout]
	inSat := uint64(0)
	var err error
	for _, spend := range txCtx.Spends {
		if inSat == outSat && spend.Satoshis == 1 {
			if o, ok := spend.Data[TAG]; ok {
				origin = &Origin{}
				if err = proto.Unmarshal(o.Data, origin); err != nil {
					log.Panic(err)
				}
				origin.Nonce++
			} else if tx, err := db.LoadTx(hex.EncodeToString(spend.Outpoint.Txid)); err != nil {
				log.Panic(err)
			} else if spendCtx, err := store.Parse(context.Background(), tx, []mod.Indexer{
				&InscriptionIndexer{},
				// &bitcom.Bitcom{},
				// &bitcom.Map{},
				&OriginIndexer{},
			}); err != nil {
				log.Panic(err)
			} else {
				origin = calcOrigin(spendCtx, spend.Outpoint.Vout, depth+1)
			}
			break
		} else if inSat > outSat {
			break
		}
		inSat += spend.Satoshis
	}
	if origin == nil {
		origin = &Origin{
			Outpoint: txo.Outpoint.Bytes(),
			Data:     map[string][]byte{},
		}
	}

	// TODO: add origin data
	return origin
}
