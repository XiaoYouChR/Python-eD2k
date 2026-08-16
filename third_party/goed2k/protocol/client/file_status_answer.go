package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type FileStatusAnswer struct {
	Hash     protocol.Hash
	BitField protocol.BitField
}

func (f *FileStatusAnswer) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	f.Hash = hash
	return f.BitField.Get(src)
}

func (f FileStatusAnswer) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, f.Hash); err != nil {
		return err
	}
	return f.BitField.Put(dst)
}

func (f FileStatusAnswer) BytesCount() int {
	return 16 + f.BitField.BytesCount()
}
