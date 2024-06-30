package types

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
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

func NewOutpointFromBytes(p []byte) *Outpoint {
	if len(p) < 32 || len(p) > 36 {
		return nil
	}
	outpoint := Outpoint{
		Txid: util.ReverseBytes(p[:32]),
		Vout: binary.LittleEndian.Uint32(p[32:]),
	}

	return &outpoint
}

func (o *Outpoint) Bytes() []byte {
	b := make([]byte, 36)
	copy(b[:32], util.ReverseBytes(o.Txid))
	binary.LittleEndian.PutUint32(b[32:], o.Vout)
	return b
}

func (o *Outpoint) JsonString() string {
	return fmt.Sprintf("%x_%d", o.Txid, o.Vout)
}

func (o *Outpoint) MarshalJSON() (bytes []byte, err error) {
	return json.Marshal(o.JsonString())
}

func (o *Outpoint) UnmarshalJSON(data []byte) error {
	if outpoint, err := NewOutpointFromString(string(data)); err != nil {
		return err
	} else {
		*o = Outpoint{
			Txid: outpoint.Txid,
			Vout: outpoint.Vout,
		}
		return nil
	}
}
