package ord

import (
	"context"
	"encoding/hex"
	"log"

	"github.com/shruggr/casemod-indexer/db"
	store "github.com/shruggr/casemod-indexer/txostore"
	"github.com/shruggr/casemod-indexer/types"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const MAX_DEPTH = 1024

type OriginIndexer struct{}

const TAG = "origin"

func (o *OriginIndexer) Tag() string {
	return TAG
}

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
	idxData.Events = append(idxData.Events, &types.Event{
		Id:    "outpoint",
		Value: txo.Outpoint.JsonString(),
	})
	return idxData
}

func (o *OriginIndexer) UnmarshalData(raw []byte) (protoreflect.ProtoMessage, error) {
	origin := &Origin{}
	if err := proto.Unmarshal(raw, origin); err != nil {
		return nil, err
	} else {
		return origin, nil
	}
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
	for _, spend := range txCtx.Spends {
		if inSat == outSat && spend.Satoshis == 1 {
			if o, ok := spend.Data[TAG]; ok {
				if origin, ok = o.Item.(*Origin); ok {
					origin.Nonce++
				}
			} else if tx, err := db.LoadTx(context.Background(), hex.EncodeToString(spend.Outpoint.Txid)); err != nil {
				log.Panic(err)
			} else if spendCtx, err := store.Parse(context.Background(), tx, []types.Indexer{
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
