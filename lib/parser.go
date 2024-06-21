package lib

import (
	"encoding/binary"
	"fmt"

	"github.com/bitcoin-sv/go-sdk/script"
)

type OpPart struct {
	OpCode byte
	Data   []byte
	Len    uint32
}

func ReadOp(b []byte, idx *int) (op *OpPart, err error) {
	if len(b) <= *idx {
		err = fmt.Errorf("ReadOp: %d %d", len(b), *idx)
		return
	}
	switch b[*idx] {
	case script.OpPUSHDATA1:
		if len(b) < *idx+2 {
			err = script.ErrDataTooSmall
			return
		}

		l := int(b[*idx+1])
		*idx += 2

		if len(b) < *idx+l {
			err = script.ErrDataTooSmall
			return
		}

		op = &OpPart{OpCode: script.OpPUSHDATA1, Data: b[*idx : *idx+l]}
		*idx += l

	case script.OpPUSHDATA2:
		if len(b) < *idx+3 {
			err = script.ErrDataTooSmall
			return
		}

		l := int(binary.LittleEndian.Uint16(b[*idx+1:]))
		*idx += 3

		if len(b) < *idx+l {
			err = script.ErrDataTooSmall
			return
		}

		op = &OpPart{OpCode: script.OpPUSHDATA2, Data: b[*idx : *idx+l]}
		*idx += l

	case script.OpPUSHDATA4:
		if len(b) < *idx+5 {
			err = script.ErrDataTooSmall
			return
		}

		l := int(binary.LittleEndian.Uint32(b[*idx+1:]))
		*idx += 5

		if len(b) < *idx+l {
			err = script.ErrDataTooSmall
			return
		}

		op = &OpPart{OpCode: script.OpPUSHDATA4, Data: b[*idx : *idx+l]}
		*idx += l

	default:
		if b[*idx] >= 0x01 && b[*idx] < script.OpPUSHDATA1 {
			l := b[*idx]
			if len(b) < *idx+int(1+l) {
				err = script.ErrDataTooSmall
				return
			}
			op = &OpPart{OpCode: b[*idx], Data: b[*idx+1 : *idx+int(l+1)]}
			*idx += int(1 + l)
		} else {
			op = &OpPart{OpCode: b[*idx]}
			*idx++
		}
	}

	op.Len = uint32(len(op.Data))
	return
}
