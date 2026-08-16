package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type SendingPart32 struct {
	Hash        protocol.Hash
	BeginOffset uint32
	EndOffset   uint32
}

func (s *SendingPart32) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	s.Hash = hash
	begin, err := protocol.ReadUInt32(src)
	if err != nil {
		return err
	}
	end, err := protocol.ReadUInt32(src)
	if err != nil {
		return err
	}
	s.BeginOffset = begin
	s.EndOffset = end
	return nil
}

func (s SendingPart32) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, s.Hash); err != nil {
		return err
	}
	if err := protocol.WriteUInt32(dst, s.BeginOffset); err != nil {
		return err
	}
	return protocol.WriteUInt32(dst, s.EndOffset)
}

func (s SendingPart32) BytesCount() int {
	return 16 + 8
}

func (s SendingPart32) PayloadSize() int {
	return int(s.EndOffset - s.BeginOffset)
}

type SendingPart64 struct {
	Hash        protocol.Hash
	BeginOffset uint64
	EndOffset   uint64
}

func (s *SendingPart64) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	s.Hash = hash
	begin, err := protocol.ReadUInt64(src)
	if err != nil {
		return err
	}
	end, err := protocol.ReadUInt64(src)
	if err != nil {
		return err
	}
	s.BeginOffset = begin
	s.EndOffset = end
	return nil
}

func (s SendingPart64) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, s.Hash); err != nil {
		return err
	}
	if err := protocol.WriteUInt64(dst, s.BeginOffset); err != nil {
		return err
	}
	return protocol.WriteUInt64(dst, s.EndOffset)
}

func (s SendingPart64) BytesCount() int {
	return 16 + 16
}

func (s SendingPart64) PayloadSize() int {
	return int(s.EndOffset - s.BeginOffset)
}
