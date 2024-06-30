package lib

import (
	"encoding/json"

	"github.com/bitcoin-sv/go-sdk/script"
)

type PKHash []byte

func (p *PKHash) Address() (string, error) {
	add, err := script.NewAddressFromPublicKeyHash(*p, true)
	if err != nil {
		return "", err
	}
	return add.AddressString, nil
}

// MarshalJSON serializes ByteArray to hex
func (p PKHash) MarshalJSON() ([]byte, error) {
	add, err := p.Address()
	if err != nil {
		return nil, err
	}
	return json.Marshal(add)
}

func NewPKHashFromAddress(a string) (p *PKHash, err error) {
	add, err := script.NewAddressFromString(a)
	// script, err := script.NewP2PKHFromAddress(a)
	if err != nil {
		return
	}

	pkh := PKHash(add.PublicKeyHash)
	return &pkh, nil
}

func NewPKHashFromScript(s []byte) (*PKHash, error) {
	if len(s) < 25 {
		return nil, nil
	}
	p := script.NewFromBytes(s[:25])
	if parts, err := p.ParseOps(); err != nil {
		return nil, err
	} else if len(parts) >= 5 &&
		parts[0].Op == script.OpDUP &&
		parts[1].Op == script.OpHASH160 &&
		len(parts[2].Data) == 20 &&
		parts[23].Op == script.OpEQUALVERIFY &&
		parts[24].Op == script.OpCHECKSIG {

		pkh := PKHash(parts[2].Data)
		return &pkh, nil
	}
	return nil, nil
}

func (p *PKHash) UnmarshalJSON(data []byte) error {
	var add string
	err := json.Unmarshal(data, &add)
	if err != nil {
		return err
	}
	if pkh, err := NewPKHashFromAddress(add); err != nil {
		return err
	} else {
		*p = *pkh
	}
	return nil
}
