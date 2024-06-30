package lib

type Spend struct {
	Txid  ByteString `json:"txid"`
	Vin   uint32     `json:"vin"`
	InAcc uint64     `json:"inacc"`
	Block Block      `json:"block"`
}

type Txo struct {
	Outpoint *Outpoint             `json:"outpoint"`
	Satoshis uint64                `json:"satoshis"`
	Script   []byte                `json:"script"`
	Block    Block                 `json:"block"`
	Owner    *PKHash               `json:"pkhash"`
	Spend    *Spend                `json:"spend"`
	Data     map[string]*IndexData `json:"data"`
}

func (t *Txo) ID() string {
	return "txo:" + t.Outpoint.String()
}

// func (t *Txo) AddData(key string, value IndexerData) {
// 	if t.Data == nil {
// 		t.Data = map[string]interface{}{}
// 	}
// 	t.Data[key] = value
// }

// func (t *Txo) AddLog(logName string, logValues map[string]string) {
// 	if t.logs == nil {
// 		t.logs = make(map[string]map[string]string)
// 	}
// 	log := t.logs[logName]
// 	if log == nil {
// 		log = make(map[string]string)
// 		t.logs[logName] = log
// 	}
// 	for k, v := range logValues {
// 		log[k] = v
// 	}
// }

// func (t *Txo) SetSpend(txCtx *IndexContext, cmdable redis.Cmdable, spentScore float64) {
// 	if j, err := json.Marshal(t.Spend); err != nil {
// 		log.Panic(err)
// 	} else if err := cmdable.JSONMerge(ctx, t.ID(), "$.spend", string(j)).Err(); err != nil {
// 		log.Panic(err)
// 	} else if err := cmdable.ZAdd(ctx, "txi:"+txCtx.Txid.String(), redis.Z{
// 		Score:  float64(t.Spend.Vin),
// 		Member: t.Outpoint.String(),
// 	}).Err(); err != nil {
// 		log.Panic(err)
// 	}

// 	// for tag, mod := range t.Data {
// 	// 	mod.SetSpend(txCtx, cmdable, t)
// 	// 	if txCtx.Block != nil {
// 	// 		for logName, logValues := range mod.Logs() {
// 	// 			t.AddLog(fmt.Sprintf("%s:%s", tag, logName), logValues)
// 	// 		}
// 	// 	}
// 	// 	for idxName, idxValue := range mod.OutputIndex() {
// 	// 		idxKey := strings.Join([]string{"io", tag, idxName}, ":")
// 	// 		cmdable.ZAdd(ctx, idxKey, redis.Z{
// 	// 			Score:  spentScore,
// 	// 			Member: idxValue,
// 	// 		})
// 	// 	}
// 	// }
// 	if err := cmdable.ZAdd(ctx, "txo:state", redis.Z{
// 		Score:  spentScore,
// 		Member: t.Outpoint.String(),
// 	}).Err(); err != nil {
// 		panic(err)
// 	}
// 	// if Rdb != nil {
// 	// 	Rdb.Publish(context.Background(), hex.EncodeToString(*txo.PKHash), txo.Outpoint.String())
// 	// }
// }

// func (t *Txo) Save(txCtx *IndexContext, cmdable redis.Cmdable) {
// 	spent := 0
// 	if t.Spend != nil {
// 		spent = 1
// 	}
// 	spentScore, err := strconv.ParseFloat(fmt.Sprintf("%d.%010d", spent, t.Block.Height), 64)
// 	if err != nil {
// 		panic(err)
// 	}

// 	key := t.ID()
// 	if exists, err := Rdb.Exists(ctx, key, key).Result(); err != nil {
// 		panic(err)
// 	} else if exists == 0 {
// 		if err = cmdable.JSONSet(ctx, key, "$", t).Err(); err != nil {
// 			panic(err)
// 		}
// 	} else {
// 		if err := cmdable.JSONSet(ctx, key, "height", t.Block).Err(); err != nil {
// 			panic(err)
// 		}
// 	}
// 	// for tag, mod := range t.Data {
// 	// 	mod.Save(txCtx, cmdable, t)
// 	// 	for idxName, idxValue := range mod.OutputIndex() {
// 	// 		idxKey := strings.Join([]string{"io", tag, idxName}, ":")
// 	// 		cmdable.ZAdd(ctx, idxKey, redis.Z{
// 	// 			Score:  spentScore,
// 	// 			Member: idxValue,
// 	// 		})
// 	// 	}
// 	// }
// 	if err := cmdable.ZAdd(ctx, "txo:state", redis.Z{
// 		Score:  spentScore,
// 		Member: t.Outpoint.String(),
// 	}).Err(); err != nil {
// 		panic(err)
// 	}

// 	// if Rdb != nil {
// 	// 	Rdb.Publish(context.Background(), hex.EncodeToString(*txo.PKHash), txo.Outpoint.String())
// 	// }
// }

// func LoadTxo(outpoint string) (txo *Txo, err error) {
// 	if j, err := Rdb.JSONGet(ctx, "txo:"+outpoint).Result(); err == redis.Nil {
// 		return nil, nil
// 	} else if err != nil {
// 		return nil, err
// 	} else {
// 		txo := &Txo{}
// 		err := json.Unmarshal([]byte(j), txo)
// 		return txo, err
// 	}
// }

// func LoadTxos(outpoints []string) ([]*Txo, error) {
// 	items := make([]*Txo, len(outpoints))
// 	for i, outpoint := range outpoints {
// 		if item, err := LoadTxo(outpoint); err != nil {
// 			return nil, err
// 		} else {
// 			items[i] = item
// 		}
// 	}

// 	return items, nil
// }

// func RefreshAddress(ctx context.Context, address string) error {
// 	lastHeight, err := Rdb.HGet(ctx, "add:sync", address).Uint64()
// 	txns, err := JB.GetAddressTransactions(ctx, address, uint32(lastHeight))
// 	if err != nil {
// 		return err
// 	}

// 	for _, txn := range txns {
// 		if _, err := Rdb.ZScore(ctx, "tx:log", txn.ID).Result(); err == nil {
// 			continue
// 		} else if err != redis.Nil {
// 			return err
// 		}
// 		if txn.BlockHeight > uint32(lastHeight) {
// 			lastHeight = uint64(txn.BlockHeight)
// 		}
// 		// if rawtx, err := JB.GetRawTransaction(ctx, txn.ID); err != nil {
// 		// 	return err
// 		// } else if _, err := IndexTxn(rawtx, &Block{
// 		// 	Hash:   NewByteStringFromHex(txn.BlockHash),
// 		// 	Height: txn.BlockHeight,
// 		// 	Idx:    txn.BlockIndex,
// 		// }); err != nil {
// 		// 	return err
// 		// }
// 	}
// 	if lastHeight > 5 {
// 		lastHeight -= 5
// 	}
// 	return Rdb.HSet(ctx, "add:sync", address, lastHeight).Err()
// }
