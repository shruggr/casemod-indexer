package lib

type Block struct {
	Hash   ByteString `json:"hash"`
	Height uint32     `json:"height"`
	Idx    uint64     `json:"idx"`
}
