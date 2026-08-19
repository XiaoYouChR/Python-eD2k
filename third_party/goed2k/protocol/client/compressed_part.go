package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type CompressedPart32 struct {
	Hash             protocol.Hash
	BeginOffset      uint32
	CompressedLength uint32
}

func (c *CompressedPart32) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	c.Hash = hash
	begin, err := protocol.ReadUInt32(src)
	if err != nil {
		return err
	}
	length, err := protocol.ReadUInt32(src)
	if err != nil {
		return err
	}
	c.BeginOffset = begin
	c.CompressedLength = length
	return nil
}

func (c CompressedPart32) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, c.Hash); err != nil {
		return err
	}
	if err := protocol.WriteUInt32(dst, c.BeginOffset); err != nil {
		return err
	}
	return protocol.WriteUInt32(dst, c.CompressedLength)
}

func (c CompressedPart32) BytesCount() int {
	return 16 + 8
}

func (c CompressedPart32) PayloadSize() int {
	return int(c.CompressedLength)
}

type CompressedPart64 struct {
	Hash             protocol.Hash
	BeginOffset      uint64
	CompressedLength uint32
}

func (c *CompressedPart64) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	c.Hash = hash
	begin, err := protocol.ReadUInt64(src)
	if err != nil {
		return err
	}
	length, err := protocol.ReadUInt32(src)
	if err != nil {
		return err
	}
	c.BeginOffset = begin
	c.CompressedLength = length
	return nil
}

func (c CompressedPart64) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, c.Hash); err != nil {
		return err
	}
	if err := protocol.WriteUInt64(dst, c.BeginOffset); err != nil {
		return err
	}
	return protocol.WriteUInt32(dst, c.CompressedLength)
}

func (c CompressedPart64) BytesCount() int {
	return 16 + 12
}

func (c CompressedPart64) PayloadSize() int {
	return int(c.CompressedLength)
}
