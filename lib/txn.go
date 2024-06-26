package lib

const THREADS = 64

// func IndexTxn(rawtx []byte, block *Block, indexers []Indexer) (ctx *IndexContext, err error) {
// 	ctx, err = ParseTxn(rawtx, block, indexers)
// 	if err != nil {
// 		return
// 	}
// 	pipe := Rdb.Pipeline()

// 	ctx.SaveSpends(pipe)

// 	ctx.SaveTxos(pipe)
// 	pipe.ZAddNX(context.Background(), "tx:log", redis.Z{
// 		Score:  float64(ctx.Block.Height),
// 		Member: hex.EncodeToString(ctx.Txid),
// 	})
// 	_, err = pipe.Exec(context.Background())
// 	return
// }

// func ParseTxn(rawtx []byte, block *Block, indexers []Indexer) (ctx *IndexContext, err error) {
// 	tx, err := transaction.NewTransactionFromBytes(rawtx)
// 	if err != nil {
// 		panic(err)
// 	}
// 	txid := tx.TxIDBytes()
// 	ctx = &IndexContext{
// 		Tx:     tx,
// 		Txid:   txid,
// 		Spends: make([]*Txo, 0, len(tx.Inputs)),
// 		Txos:   make([]*Txo, 0, len(tx.Outputs)),
// 	}
// 	if block == nil {
// 		block = &Block{
// 			Height: uint32(time.Now().Unix()),
// 		}
// 	}
// 	ctx.Block = block

// 	if !tx.IsCoinbase() {
// 		ParseSpends(ctx)
// 	}

// 	ParseTxos(ctx, indexers)
// 	return
// }

// func ParseSpends(ctx *IndexContext) {
// 	inAcc := uint64(0)
// 	for vin, txin := range ctx.Tx.Inputs {
// 		outpoint := NewOutpoint(txin.SourceTXID, txin.SourceTxOutIndex)
// 		spend, err := LoadTxo(outpoint.String())
// 		if err != nil {
// 			panic(err)
// 		}
// 		if spend == nil {
// 			if inTx, err := LoadTx(txin.PreviousTxIDStr()); err != nil {
// 				panic(err)
// 			} else {
// 				inCtx := &IndexContext{
// 					Tx:   inTx,
// 					Txid: inTx.TxIDBytes(),
// 				}
// 				ParseTxos(inCtx, []Indexer{})
// 				spend = inCtx.Txos[txin.SourceTxOutIndex]
// 			}
// 		}

// 		spend.Spend = &Spend{
// 			Txid:  ctx.Txid,
// 			Vin:   uint32(vin),
// 			InAcc: inAcc,
// 			Block: *ctx.Block,
// 		}
// 		ctx.Spends = append(ctx.Spends, spend)
// 		inAcc += spend.Satoshis
// 	}
// }

// func ParseTxos(ctx *IndexContext, indexers []Indexer) {
// 	accSats := uint64(0)
// 	for vout, txout := range ctx.Tx.Outputs {
// 		outpoint := Outpoint(binary.BigEndian.AppendUint32(ctx.Txid, uint32(vout)))
// 		txo := &Txo{
// 			Outpoint: &outpoint,
// 			Satoshis: txout.Satoshis,
// 			OutAcc:   accSats,
// 			Script:   *txout.LockingScript,
// 			Block:    ctx.Block,
// 		}

// 		if len(txo.Script) >= 25 && script.NewFromBytes(txo.Script[:25]).IsP2PKH() {
// 			pkhash := PKHash(txo.Script[3:23])
// 			txo.Owner = &pkhash
// 		}

// 		for _, indexer := range indexers {
// 			indexer.Parse(ctx, uint32(vout))
// 		}
// 		ctx.Txos = append(ctx.Txos, txo)
// 		accSats += txout.Satoshis
// 	}
// }

// var spendsCache = make(map[string][]*Txo)
// var m sync.Mutex

// func LoadSpends(txid ByteString, tx *bt.Tx) []*Txo {
// 	// fmt.Println("Loading Spends", hex.EncodeToString(txid))
// 	var err error
// 	if tx == nil {
// 		tx, err = LoadTx(hex.EncodeToString(txid))
// 		if err != nil {
// 			log.Panicf("[LoadSpends] %x %v\n", txid, err)
// 		}
// 	}

// 	outpoints := make([]string, len(tx.Inputs))
// 	for vin, txin := range tx.Inputs {
// 		outpoints[vin] = NewOutpoint(txin.PreviousTxID(), txin.PreviousTxOutIndex).String()
// 	}
// 	spendByOutpoint := make(map[string]*Txo, len(tx.Inputs))
// 	spends := make([]*Txo, 0, len(tx.Inputs))

// 	spends, err :=
// 	rows, err := Db.Query(context.Background(), `
// 		SELECT outpoint, satoshis, outacc
// 		FROM txos
// 		WHERE spend=$1`,
// 		txid,
// 	)
// 	if err != nil {
// 		log.Panic(err)
// 	}
// 	defer rows.Close()

// 	for rows.Next() {
// 		spend := &Txo{
// 			Spend: &txid,
// 		}
// 		var satoshis sql.NullInt64
// 		var outAcc sql.NullInt64
// 		err = rows.Scan(&spend.Outpoint, &satoshis, &outAcc)
// 		if err != nil {
// 			log.Panic(err)
// 		}
// 		if satoshis.Valid && outAcc.Valid {
// 			spend.Satoshis = uint64(satoshis.Int64)
// 			spend.OutAcc = uint64(outAcc.Int64)
// 			spendByOutpoint[spend.Outpoint.String()] = spend
// 		}
// 	}

// 	var inSats uint64
// 	for vin, txin := range tx.Inputs {
// 		outpoint := NewOutpoint(txin.PreviousTxID(), txin.PreviousTxOutIndex)
// 		spend, ok := spendByOutpoint[outpoint.String()]
// 		if !ok {
// 			spend = &Txo{
// 				Outpoint: outpoint,
// 				Spend:    &txid,
// 				Vin:      uint32(vin),
// 			}

// 			tx, err := LoadTx(txin.PreviousTxIDStr())
// 			if err != nil {
// 				log.Panic(txin.PreviousTxIDStr(), err)
// 			}
// 			var outSats uint64
// 			for vout, txout := range tx.Outputs {
// 				if vout < int(spend.Outpoint.Vout()) {
// 					outSats += txout.Satoshis
// 					continue
// 				}
// 				spend.Satoshis = txout.Satoshis
// 				spend.OutAcc = outSats
// 				spend.Save()
// 				spendByOutpoint[outpoint.String()] = spend
// 				break
// 			}
// 		} else {
// 			spend.Vin = uint32(vin)
// 		}
// 		spends = append(spends, spend)
// 		inSats += spend.Satoshis
// 		// fmt.Println("Inputs:", spends[vin].Outpoint)
// 	}
// 	return spends
// }
