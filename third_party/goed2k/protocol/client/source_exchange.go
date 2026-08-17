package client

import (
	"bytes"
	"fmt"

	"github.com/monkeyWie/goed2k/protocol"
)

const sourceExchangeV3EntrySize = 4 + 2 + 4 + 2 + 16

type SourceExchangeRequest struct {
	Hash protocol.Hash
}

func (s *SourceExchangeRequest) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	s.Hash = hash
	return nil
}

func (s SourceExchangeRequest) Put(dst *bytes.Buffer) error {
	return protocol.WriteHash(dst, s.Hash)
}

func (s SourceExchangeRequest) BytesCount() int {
	return 16
}

type SourceExchangeSource struct {
	ClientID   uint32
	ClientPort uint16
	ServerIP   uint32
	ServerPort uint16
	UserHash   protocol.Hash
}

type SourceExchangeAnswer struct {
	Hash    protocol.Hash
	Sources []SourceExchangeSource
}

func (s *SourceExchangeAnswer) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	count, err := protocol.ReadUInt16(src)
	if err != nil {
		return err
	}
	if src.Len() != int(count)*sourceExchangeV3EntrySize {
		return fmt.Errorf("source exchange v3 declares %d sources in %d bytes", count, src.Len())
	}
	s.Hash = hash
	s.Sources = make([]SourceExchangeSource, int(count))
	for i := range s.Sources {
		entry := &s.Sources[i]
		if entry.ClientID, err = protocol.ReadUInt32(src); err != nil {
			return err
		}
		if entry.ClientPort, err = protocol.ReadUInt16(src); err != nil {
			return err
		}
		if entry.ServerIP, err = protocol.ReadUInt32(src); err != nil {
			return err
		}
		if entry.ServerPort, err = protocol.ReadUInt16(src); err != nil {
			return err
		}
		if entry.UserHash, err = protocol.ReadHash(src); err != nil {
			return err
		}
	}
	return nil
}

func (s SourceExchangeAnswer) Put(dst *bytes.Buffer) error {
	if len(s.Sources) > int(^uint16(0)) {
		return fmt.Errorf("too many source exchange entries: %d", len(s.Sources))
	}
	if err := protocol.WriteHash(dst, s.Hash); err != nil {
		return err
	}
	if err := protocol.WriteUInt16(dst, uint16(len(s.Sources))); err != nil {
		return err
	}
	for _, entry := range s.Sources {
		if err := protocol.WriteUInt32(dst, entry.ClientID); err != nil {
			return err
		}
		if err := protocol.WriteUInt16(dst, entry.ClientPort); err != nil {
			return err
		}
		if err := protocol.WriteUInt32(dst, entry.ServerIP); err != nil {
			return err
		}
		if err := protocol.WriteUInt16(dst, entry.ServerPort); err != nil {
			return err
		}
		if err := protocol.WriteHash(dst, entry.UserHash); err != nil {
			return err
		}
	}
	return nil
}

func (s SourceExchangeAnswer) BytesCount() int {
	return 16 + 2 + len(s.Sources)*sourceExchangeV3EntrySize
}
