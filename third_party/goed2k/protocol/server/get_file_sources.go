package server

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type GetFileSources struct {
	Hash    protocol.Hash
	LowPart int32
	HiPart  int32
}

func (g *GetFileSources) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	g.Hash = hash
	val, err := protocol.ReadInt32(src)
	if err != nil {
		return err
	}
	if val == 0 {
		low, err := protocol.ReadInt32(src)
		if err != nil {
			return err
		}
		high, err := protocol.ReadInt32(src)
		if err != nil {
			return err
		}
		g.LowPart = low
		g.HiPart = high
		return nil
	}
	g.LowPart = val
	return nil
}

func (g GetFileSources) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, g.Hash); err != nil {
		return err
	}
	if g.HiPart != 0 {
		if err := protocol.WriteInt32(dst, 0); err != nil {
			return err
		}
	}
	if err := protocol.WriteInt32(dst, g.LowPart); err != nil {
		return err
	}
	if g.HiPart != 0 {
		return protocol.WriteInt32(dst, g.HiPart)
	}
	return nil
}

func (g GetFileSources) BytesCount() int {
	if g.HiPart != 0 {
		return 16 + 8
	}
	return 16 + 4
}
