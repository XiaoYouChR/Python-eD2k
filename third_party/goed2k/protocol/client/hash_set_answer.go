package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type HashSetAnswer struct {
	Hash  protocol.Hash
	Parts []protocol.Hash
}

func (h *HashSetAnswer) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	h.Hash = hash
	count, err := protocol.ReadUInt16(src)
	if err != nil {
		return err
	}
	h.Parts = make([]protocol.Hash, int(count))
	for i := range h.Parts {
		part, err := protocol.ReadHash(src)
		if err != nil {
			return err
		}
		h.Parts[i] = part
	}
	return nil
}

func (h HashSetAnswer) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, h.Hash); err != nil {
		return err
	}
	if err := protocol.WriteUInt16(dst, uint16(len(h.Parts))); err != nil {
		return err
	}
	for _, part := range h.Parts {
		if err := protocol.WriteHash(dst, part); err != nil {
			return err
		}
	}
	return nil
}

func (h HashSetAnswer) BytesCount() int {
	return 16 + 2 + len(h.Parts)*16
}
