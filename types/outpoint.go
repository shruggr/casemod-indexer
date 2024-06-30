package types

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/bitcoin-sv/go-sdk/util"
)

func NewOutpointFromString(s string) (o *Outpoint, err error) {
	if len(s) < 66 {
		return nil, fmt.Errorf("invalid-string")
	}
	txid, err := hex.DecodeString(s[:64])
	if err != nil {
		return
	}
	vout, err := strconv.ParseUint(s[65:], 10, 32)
	if err != nil {
		return
	}
	outpoint := Outpoint{
		Txid: txid,
		Vout: uint32(vout),
	}
	o = &outpoint
	return
}

func NewOutpointFromTxOutpoint(p []byte) (o *Outpoint, err error) {
	if len(p) < 32 || len(p) > 36 {
		return nil, errors.New("invalid pointer")
	}
	outpoint := Outpoint{
		Txid: util.ReverseBytes(p[:32]),
		Vout: binary.LittleEndian.Uint32(p[32:]),
	}

	o = &outpoint
	return
}

func (o *Outpoint) String() string {
	return fmt.Sprintf("%x_%d", o.Txid, o.Vout)
}

func (o Outpoint) MarshalJSON() (bytes []byte, err error) {
	return json.Marshal(o.String())
}

func (o *Outpoint) UnmarshalJSON(data []byte) error {
	outpoint, err := NewOutpointFromString(string(data))
	*o = *outpoint
	return err
}
