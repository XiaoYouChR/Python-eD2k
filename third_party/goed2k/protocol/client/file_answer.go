package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type FileAnswer struct {
	Hash protocol.Hash
	Name protocol.ByteContainer16
}

func (f *FileAnswer) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	f.Hash = hash
	return f.Name.Get(src)
}

func (f FileAnswer) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, f.Hash); err != nil {
		return err
	}
	return f.Name.Put(dst)
}

func (f FileAnswer) BytesCount() int {
	return 16 + f.Name.BytesCount()
}
