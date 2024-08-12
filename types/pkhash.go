package types

import (
	"encoding/json"

	"github.com/bitcoin-sv/go-sdk/script"
)

type PKHash []byte

func (p *PKHash) Address() string {
	add, _ := script.NewAddressFromPublicKeyHash(*p, true)
	return add.AddressString
}

func (p *PKHash) String() string {
	return p.Address()
}

func (p PKHash) MarshalJSON() ([]byte, error) {
	add := p.Address()
	return json.Marshal(add)
}

func NewPKHashFromAddress(a string) (p *PKHash, err error) {
	add, err := script.NewAddressFromString(a)
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
		parts[3].Op == script.OpEQUALVERIFY &&
		parts[4].Op == script.OpCHECKSIG {

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
