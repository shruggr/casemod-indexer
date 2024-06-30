package origin

import (
	"log"

	"github.com/shruggr/casemod-indexer/lib"
	"github.com/shruggr/casemod-indexer/mod/bitcom"
	"github.com/shruggr/casemod-indexer/mod/ord"
)

const MAX_DEPTH = 1024

type Origin struct {
	lib.IndexData
	Outpoint *lib.Outpoint          `json:"outpoint"`
	Nonce    uint32                 `json:"nonce"`
	Data     map[string]interface{} `json:"map,omitempty"`
}

func (o *Origin) Tag() string {
	return "origin"
}

func (o *Origin) Parse(txCtx *lib.IndexContext, vout uint32) lib.IndexData {
	txo := txCtx.Txos[vout]
	if txo.Satoshis != 1 {
		return nil
	}

	return calcOrigin(txCtx, vout, 0)
}

func calcOrigin(txCtx *lib.IndexContext, vout uint32, depth uint32) *Origin {
	if depth > MAX_DEPTH {
		return nil
	}
	txo := txCtx.Txos[vout]
	for vin, spend := range txCtx.Spends {
		if spend.Satoshis == 1 && spend.Spend.InAcc == txo.OutAcc {
			var origin *Origin
			if data, ok := spend.Data["origin"].(map[string]interface{}); !ok {
				if rawtx, err := lib.LoadRawtx(spend.Outpoint.TxidStr()); err != nil {
					log.Panic(err)
				} else if spendCtx, err := lib.ParseTxn(rawtx, nil, []lib.Indexer{
					&ord.Inscription{},
					&bitcom.Bitcom{},
					&bitcom.Map{},
					&Origin{},
				}); err != nil {
					log.Panic(err)
				} else if origin = calcOrigin(spendCtx, spend.Outpoint.Vout(), depth+1); origin != nil {
					for _, spend := range txCtx.Spends[0:vin] {
						origin.AddDependencies(spend.Outpoint.Txid())
					}
					spend.AddData("origin", origin)
					spend.Save(spendCtx, lib.Rdb)
				}
			} else {
				origin = &Origin{}
				for k, v := range data {
					switch k {
					case "outpoint":
						origin.Outpoint, _ = lib.NewOutpointFromString(v.(string))
					case "nonce":
						origin.Nonce = uint32(v.(float64))
					case "map":
						origin.Data = v.(map[string]interface{})
					}
				}
			}
			if origin != nil {
				origin.Nonce = origin.Nonce + 1
				if mp, ok := txo.Data["map"].(*bitcom.Map); ok {
					for k, v := range mp.Data {
						origin.Data[k] = v
					}
				}
				return origin
			}
			return nil
		} else if spend.Spend.InAcc > txo.OutAcc {
			break
		}
	}
	origin := &Origin{
		Outpoint: txo.Outpoint,
		Data:     map[string]interface{}{},
	}
	if mp, ok := txo.Data["map"].(*bitcom.Map); ok {
		for k, v := range mp.Data {
			origin.Data[k] = v
		}
	}
	return origin
}

// func (o *Origin) Save(ctx *lib.IndexContext, cmdable redis.Cmdable, txo *lib.Txo) {
// 	o.IndexBySpent(o.Outpoint.String(), txo.Outpoint.String())
// }

// func SaveMap(origin *lib.Outpoint) {
// 	rows, err := lib.Db.Query(context.Background(), `
// 		SELECT data->>'map'
// 		FROM txos
// 		WHERE origin=$1 AND data->>'map' IS NOT NULL
// 		ORDER BY height ASC, idx ASC, vout ASC`,
// 		origin,
// 	)
// 	if err != nil {
// 		log.Panicf("BuildMap Error: %s %+v\n", origin, err)
// 	}
// 	rows.Close()

// 	m := lib.Map{}
// 	for rows.Next() {
// 		var data lib.Map
// 		err = rows.Scan(&data)
// 		if err != nil {
// 			log.Panicln(err)
// 		}
// 		for k, v := range data {
// 			m[k] = v
// 		}
// 	}

// 	_, err = lib.Db.Exec(context.Background(), `
// 		INSERT INTO origins(origin, map)
// 		VALUES($1, $2)
// 		ON CONFLICT(origin) DO UPDATE SET
// 			map=EXCLUDED.map`,
// 		origin,
// 		m,
// 	)
// 	if err != nil {
// 		log.Panicf("Save Error: %s %+v\n", origin, err)
// 	}
// }

// func SetOriginNum(height uint32) (err error) {

// 	row := lib.Db.QueryRow(context.Background(),
// 		"SELECT MAX(num) FROM origins",
// 	)
// 	var dbNum sql.NullInt64
// 	err = row.Scan(&dbNum)
// 	if err != nil {
// 		log.Panic(err)
// 		return
// 	}
// 	var num uint64
// 	if dbNum.Valid {
// 		num = uint64(dbNum.Int64 + 1)
// 	}

// 	rows, err := lib.Db.Query(context.Background(), `
// 		SELECT origin
// 		FROM origins
// 		WHERE num = -1 AND height <= $1 AND height IS NOT NULL
// 		ORDER BY height, idx
// 		LIMIT 100`,
// 		height,
// 	)
// 	if err != nil {
// 		log.Panic(err)
// 		return
// 	}
// 	defer rows.Close()
// 	for rows.Next() {
// 		origin := &lib.Outpoint{}
// 		err = rows.Scan(&origin)
// 		if err != nil {
// 			log.Panic(err)
// 			return
// 		}
// 		// fmt.Printf("Origin Num %d %s\n", num, origin)
// 		_, err = lib.Db.Exec(context.Background(), `
// 			UPDATE origins
// 			SET num=$2
// 			WHERE origin=$1`,
// 			origin, num,
// 		)
// 		if err != nil {
// 			log.Panic(err)
// 			return
// 		}
// 		num++
// 	}
// 	lib.Rdb.Publish(context.Background(), "inscriptionNum", fmt.Sprintf("%d", num))
// 	// log.Println("Height", height, "Max Origin Num", num)
// 	return
// }

// func (t *Txo) SetOrigin(origin *Outpoint) {
// 	var err error
// 	for i := 0; i < 3; i++ {
// 		_, err = Db.Exec(context.Background(), `
// 			INSERT INTO txos(outpoint, origin, satoshis, outacc)
// 			VALUES($1, $2, $3, $4)
// 			ON CONFLICT(outpoint) DO UPDATE SET
// 				origin=EXCLUDED.origin`,
// 			t.Outpoint,
// 			origin,
// 			t.Satoshis,
// 			t.OutAcc,
// 		)

// 		if err != nil {
// 			var pgErr *pgconn.PgError
// 			if errors.As(err, &pgErr) {
// 				if pgErr.Code == "23505" {
// 					time.Sleep(100 * time.Millisecond)
// 					// log.Printf("Conflict. Retrying SetOrigin %s\n", t.Outpoint)
// 					continue
// 				}
// 			}
// 			log.Panicln("insTxo Err:", err)
// 		}
// 		break
// 	}
// 	if err != nil {
// 		log.Panicln("insTxo Err:", err)
// 	}
// }
