package server

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type FoundFileSources struct {
	Hash    protocol.Hash
	Sources []protocol.Endpoint
}

func (f *FoundFileSources) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	f.Hash = hash
	countByte, err := src.ReadByte()
	if err != nil {
		return err
	}
	count := int(countByte)
	f.Sources = make([]protocol.Endpoint, count)
	for i := 0; i < count; i++ {
		ep, err := protocol.ReadEndpoint(src)
		if err != nil {
			return err
		}
		f.Sources[i] = ep
	}
	return nil
}

func (f FoundFileSources) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, f.Hash); err != nil {
		return err
	}
	if err := dst.WriteByte(byte(len(f.Sources))); err != nil {
		return err
	}
	for _, ep := range f.Sources {
		if err := protocol.WriteEndpoint(dst, ep); err != nil {
			return err
		}
	}
	return nil
}

func (f FoundFileSources) BytesCount() int {
	return 16 + 1 + len(f.Sources)*6
}
