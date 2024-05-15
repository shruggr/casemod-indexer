package ordinals

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/GorillaPool/go-junglebus/models"
	"github.com/fxamacker/cbor"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/shruggr/fungibles-indexer/lib"
)

var AsciiRegexp = regexp.MustCompile(`^[[:ascii:]]*$`)

// var Db *pgxpool.Pool
// var Rdb *redis.Client

// func Initialize(db *pgxpool.Pool, rdb *redis.Client) (err error) {
// 	Db = db
// 	Rdb = rdb

// 	lib.Initialize(db, rdb)
// 	return
// }

func IndexTxn(rawtx []byte, blockId string, height uint32, idx uint64) (ctx *lib.IndexContext) {
	ctx, err := lib.ParseTxn(rawtx, blockId, height, idx)
	if err != nil {
		log.Panicln(err)
	}

	IndexInscriptions(ctx)
	return
}

func IndexInscriptions(ctx *lib.IndexContext) {
	CalculateOrigins(ctx)
	ParseInscriptions(ctx)
	ctx.SaveSpends()
	ctx.Save()

	lib.Db.Exec(context.Background(),
		`INSERT INTO txn_indexer(txid, indexer) 
		VALUES ($1, 'ord')
		ON CONFLICT DO NOTHING`,
		ctx.Txid,
	)
}

func CalculateOrigins(ctx *lib.IndexContext) {
	for _, txo := range ctx.Txos {
		if txo.Satoshis != 1 {
			continue
		}
		txo.Origin = LoadOrigin(txo.Outpoint, txo.OutAcc)
	}
}

func ParseInscriptions(ctx *lib.IndexContext) {
	for _, txo := range ctx.Txos {
		if txo.PKHash != nil && len(*txo.PKHash) != 0 { //&& txo.Satoshis != 1 {
			continue
		}
		ParseScript(txo)
	}
}

func ParseScript(txo *lib.Txo) {
	vout := txo.Outpoint.Vout()
	script := *txo.Tx.Outputs[vout].LockingScript

	start := 0
	if len(script) >= 25 && bscript.NewFromBytes(script[:25]).IsP2PKH() {
		pkhash := lib.PKHash(script[3:23])
		txo.PKHash = &pkhash
		start = 25
	}

	var opReturn int
	for i := start; i < len(script); {
		startI := i
		op, err := lib.ReadOp(script, &i)
		if err != nil {
			break
		}
		switch op.OpCode {
		case bscript.OpRETURN:
			if opReturn == 0 {
				opReturn = startI
			}
			bitcom, err := lib.ParseBitcom(txo.Tx, vout, &i)
			if err != nil {
				continue
			}
			addBitcom(txo, bitcom)

		case bscript.OpDATA1:
			if op.Data[0] == '|' && opReturn > 0 {
				bitcom, err := lib.ParseBitcom(txo.Tx, vout, &i)
				if err != nil {
					continue
				}
				addBitcom(txo, bitcom)
			}
		case bscript.OpDATA3:
			if i > 2 && bytes.Equal(op.Data, []byte("ord")) && script[startI-2] == 0 && script[startI-1] == bscript.OpIF {
				ParseInscription(txo, script, &i)
			}
		}
	}
}

func ParseInscription(txo *lib.Txo, script []byte, fromPos *int) {
	pos := *fromPos
	ins := &Inscription{
		File: &lib.File{},
	}

ordLoop:
	for {
		op, err := lib.ReadOp(script, &pos)
		if err != nil || op.OpCode > bscript.Op16 {
			return
		}

		op2, err := lib.ReadOp(script, &pos)
		if err != nil || op2.OpCode > bscript.Op16 {
			return
		}

		var field int
		if op.OpCode > bscript.OpPUSHDATA4 && op.OpCode <= bscript.Op16 {
			field = int(op.OpCode) - 80
		} else if op.Len == 1 {
			field = int(op.Data[0])
		} else if op.Len > 1 {
			if ins.Fields == nil {
				ins.Fields = lib.Map{}
			}
			if op.Len <= 64 && utf8.Valid(op.Data) && !bytes.Contains(op.Data, []byte{0}) && !bytes.Contains(op.Data, []byte("\\u0000")) {
				ins.Fields[string(op.Data)] = op2.Data
			}
			if string(op.Data) == lib.MAP {
				script := bscript.NewFromBytes(op2.Data)
				pos := 0
				md := lib.ParseMAP(script, &pos)
				if md != nil {
					txo.AddData("map", md)
				}
			}
			continue
		}

		switch field {
		case 0:
			ins.File.Content = op2.Data
			break ordLoop
		case 1:
			if len(op2.Data) < 256 && utf8.Valid(op2.Data) {
				ins.File.Type = string(op2.Data)
			}
		case 2:
			pointer := binary.LittleEndian.Uint64(op2.Data)
			ins.Pointer = &pointer
		case 3:
			if parent, err := lib.NewOutpointFromTxOutpoint(op2.Data); err == nil {
				ins.Parent = parent
			}
		case 5:
			md := &lib.Map{}
			if err := cbor.Unmarshal(op2.Data, md); err == nil {
				ins.Metadata = *md
			}
		case 7:
			ins.Metaproto = op2.Data
		case 9:
			ins.File.Encoding = string(op2.Data)
		default:
			if ins.Fields == nil {
				ins.Fields = lib.Map{}
			}

		}
	}
	op, err := lib.ReadOp(script, &pos)
	if err != nil || op.OpCode != bscript.OpENDIF {
		return
	}
	*fromPos = pos

	ins.File.Size = uint32(len(ins.File.Content))
	hash := sha256.Sum256(ins.File.Content)
	ins.File.Hash = hash[:]
	insType := "file"
	var bsv20 interface{}
	if ins.File.Size <= 1024 && utf8.Valid(ins.File.Content) && !bytes.Contains(ins.File.Content, []byte{0}) && !bytes.Contains(ins.File.Content, []byte("\\u0000")) {
		mime := strings.ToLower(ins.File.Type)
		if strings.HasPrefix(mime, "application") ||
			strings.HasPrefix(mime, "text") {

			var data json.RawMessage
			err := json.Unmarshal(ins.File.Content, &data)
			if err == nil {
				insType = "json"
				ins.Json = data
				bsv20, _ = ParseBsv20Inscription(ins.File, txo)
			} else if AsciiRegexp.Match(ins.File.Content) {
				if insType == "file" {
					insType = "text"
				}
				ins.Text = string(ins.File.Content)
				re := regexp.MustCompile(`\W`)
				words := map[string]struct{}{}
				for _, word := range re.Split(ins.Text, -1) {
					if len(word) > 0 {
						word = strings.ToLower(word)
						words[word] = struct{}{}
					}
				}
				if len(words) > 0 {
					ins.Words = make([]string, 0, len(words))
					for word := range words {
						ins.Words = append(ins.Words, word)
					}
				}
			}
		}
	}
	if txo.Data == nil {
		txo.Data = map[string]interface{}{}
	}
	txo.Data["insc"] = ins
	var types []string
	if prev, ok := txo.Data["types"].([]string); ok {
		types = prev
	}
	types = append(types, insType)
	txo.Data["types"] = types
	if bsv20 != nil {
		txo.Data["bsv20"] = bsv20
	}

	if txo.PKHash != nil && len(*txo.PKHash) == 0 {
		if len(script) >= pos+25 && bscript.NewFromBytes(script[pos:pos+25]).IsP2PKH() {
			pkhash := lib.PKHash(script[pos+3 : pos+23])
			txo.PKHash = &pkhash
		} else if len(script) >= pos+26 &&
			script[pos] == bscript.OpCODESEPARATOR &&
			bscript.NewFromBytes(script[pos+1:pos+26]).IsP2PKH() {
			pkhash := lib.PKHash(script[pos+4 : pos+24])
			txo.PKHash = &pkhash
		}
	}
}

func addBitcom(txo *lib.Txo, bitcom interface{}) {
	if bitcom == nil {
		return
	}
	switch bc := bitcom.(type) {
	case *lib.Sigma:
		var sigmas []*lib.Sigma
		if prev, ok := txo.Data["sigma"].([]*lib.Sigma); ok {
			sigmas = prev
		}
		sigmas = append(sigmas, bc)
		txo.AddData("sigma", sigmas)
	case lib.Map:
		txo.AddData("map", bc)
	case *lib.File:
		txo.AddData("b", bc)
	}
}

func RefreshAddress(ctx context.Context, address string) error {
	row := lib.Db.QueryRow(ctx,
		"SELECT height, updated FROM addresses WHERE address=$1",
		address,
	)
	var lastHeight uint32
	var updated time.Time
	row.Scan(&lastHeight, &updated)

	// if time.Since(updated) < 30*time.Minute {
	// 	log.Println("Frequent Update", address)
	// }
	txns, err := lib.JB.GetAddressTransactionDetails(ctx, address, lastHeight)
	if err != nil {
		return err
	}

	// txids := make([][]byte, len(txns))
	toIndex := map[string]*models.Transaction{}
	batches := [][][]byte{}
	batch := make([][]byte, 0, 100)
	// log.Println("Txns:", len(txns))
	for i, txn := range txns {
		batch = append(batch, txn.ID)
		toIndex[txn.ID.String()] = txn
		if txn.BlockHeight > lastHeight {
			lastHeight = txn.BlockHeight
		}

		if i%100 == 99 || i == len(txns)-1 {
			batches = append(batches, batch)
			batch = make([][]byte, 0, 100)
		}
	}

	for _, batch := range batches {
		// log.Println("Batch", len(batch))
		if len(batch) == 0 {
			break
		}
		rows, err := lib.Db.Query(ctx, `
			SELECT encode(txid, 'hex')
			FROM txn_indexer 
			WHERE indexer='ord' AND txid = ANY($1)`,
			batch,
		)
		if err != nil {
			log.Println(err)
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var txid string
			err := rows.Scan(&txid)
			if err != nil {
				return err
			}
			delete(toIndex, txid)
		}
		rows.Close()
	}
	var wg sync.WaitGroup
	limiter := make(chan struct{}, 32)
	for txid, txn := range toIndex {
		wg.Add(1)
		limiter <- struct{}{}
		go func(txid string, txn *models.Transaction) {
			defer func() {
				wg.Done()
				<-limiter
			}()

			IndexTxn(txn.Transaction, txn.BlockHash.String(), txn.BlockHeight, txn.BlockIndex)
		}(txid, txn)
	}
	wg.Wait()
	if lastHeight == 0 {
		lastHeight = 817000
	}
	_, err = lib.Db.Exec(ctx, `
		INSERT INTO addresses(address, height, updated)
		VALUES ($1, $2, CURRENT_TIMESTAMP) 
		ON CONFLICT (address) DO UPDATE SET 
			height = EXCLUDED.height, 
			updated = CURRENT_TIMESTAMP`,
		address,
		lastHeight-6,
	)
	return err
}

func GetLatestOutpoint(ctx context.Context, origin *lib.Outpoint) (*lib.Outpoint, error) {
	var latest *lib.Outpoint

	// Update spends on all known unspent txos
	rows, err := lib.Db.Query(ctx, `
		SELECT outpoint
		FROM txos
		WHERE origin=$1 AND spend='\x'
		ORDER BY height DESC, idx DESC
		LIMIT 1`,
		origin,
	)
	if err != nil {
		// log.Println("FastForwardOrigin", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var outpoint *lib.Outpoint
		err := rows.Scan(&outpoint)
		if err != nil {
			log.Println("FastForwardOrigin", err)
			return nil, err
		}

		spend, err := lib.JB.GetSpend(ctx, hex.EncodeToString(outpoint.Txid()), outpoint.Vout())
		if err != nil {
			log.Println("GetSpend", err)
			return nil, err
		}

		if len(spend) == 0 {
			latest = outpoint
			break
		}

		rawtx, err := lib.LoadRawtx(hex.EncodeToString(spend))
		if err != nil {
			log.Println("GetTransaction", err)
			return nil, err
		}
		if len(rawtx) < 100 {
			log.Println("transaction too short", string(rawtx))
			return nil, fmt.Errorf("transaction too short")
		}
		IndexTxn(rawtx, "", 0, 0)
	}

	if latest != nil {
		return latest, nil
	}

	// Fast-forward origin
	row := lib.Db.QueryRow(ctx, `
		SELECT outpoint
		FROM txos
		WHERE origin = $1
		ORDER BY CASE WHEN spend='\x' THEN 1 ELSE 0 END DESC, height DESC, idx DESC
		LIMIT 1`,
		origin,
	)
	err = row.Scan(&latest)
	if err != nil {
		log.Println("Lookup latest", err)
		return nil, err
	}

	for {
		spend, err := lib.JB.GetSpend(ctx, hex.EncodeToString(latest.Txid()), latest.Vout())
		if err != nil {
			log.Println("GetSpend", err)
			return nil, err
		}

		if len(spend) == 0 {
			return latest, nil
		}

		txn, err := lib.JB.GetTransaction(ctx, hex.EncodeToString(spend))
		// rawtx, err := lib.LoadRawtx(hex.EncodeToString(spend))
		if err != nil {
			log.Println("GetTransaction", err)
			return nil, err
		}

		// log.Printf("Indexing: %s\n", hex.EncodeToString(spend))
		txCtx := IndexTxn(txn.Transaction, txn.BlockHash.String(), txn.BlockHeight, txn.BlockIndex)
		for _, txo := range txCtx.Txos {
			if txo.Origin != nil && bytes.Equal(*txo.Origin, *origin) {
				latest = txo.Outpoint
				break
			}
		}

		if !bytes.Equal(latest.Txid(), txCtx.Txid) {
			return latest, nil
		}
	}
}
