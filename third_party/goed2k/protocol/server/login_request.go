package server

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

const (
	opLoginRequest byte = 0x01

	CapableZlib      = 0x0001
	CapableIPInLogin = 0x0002
	CapableAuxPort   = 0x0004
	CapableNewTags   = 0x0008
	CapableUnicode   = 0x0010
	CapableLargeFile = 0x0100

	JED2KVersionMajor = 1
	JED2KVersionMinor = 1
	JED2KVersionTiny  = 0

	ctName         = 0x01
	ctVersion      = 0x11
	ctServerFlags  = 0x20
	ctEMuleVersion = 0xFB
)

type LoginRequest struct {
	Hash       protocol.Hash
	Point      protocol.Endpoint
	Properties protocol.TagList
}

func (l *LoginRequest) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	point, err := protocol.ReadEndpoint(src)
	if err != nil {
		return err
	}
	count, err := protocol.ReadUInt32(src)
	if err != nil {
		return err
	}
	l.Hash = hash
	l.Point = point
	l.Properties = make(protocol.TagList, int(count))
	for i := 0; i < int(count); i++ {
		var tag protocol.SimpleTag
		if err := tag.Get(src); err != nil {
			return err
		}
		l.Properties[i] = tag
	}
	return nil
}

func (l LoginRequest) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, l.Hash); err != nil {
		return err
	}
	if err := protocol.WriteEndpoint(dst, l.Point); err != nil {
		return err
	}
	if err := protocol.WriteUInt32(dst, uint32(len(l.Properties))); err != nil {
		return err
	}
	for _, property := range l.Properties {
		if err := property.Put(dst); err != nil {
			return err
		}
	}
	return nil
}

func (l LoginRequest) BytesCount() int {
	size := 16 + 6 + 4
	for _, property := range l.Properties {
		size += property.BytesCount()
	}
	return size
}

func NewLoginRequest(userAgent protocol.Hash, listenPort int, clientName string) LoginRequest {
	versionClient := uint32((JED2KVersionMajor << 24) | (JED2KVersionMinor << 17) | (JED2KVersionTiny << 10) | (1 << 7))
	capability := uint32(CapableAuxPort | CapableNewTags | CapableUnicode | CapableLargeFile | CapableZlib)
	return LoginRequest{
		Hash:  userAgent,
		Point: protocol.NewEndpoint(0, listenPort),
		Properties: protocol.TagList{
			protocol.NewUInt32Tag(ctVersion, 0x3c),
			protocol.NewUInt32Tag(ctServerFlags, capability),
			protocol.NewStringTag(ctName, clientName),
			protocol.NewUInt32Tag(ctEMuleVersion, versionClient),
		},
	}
}

var _ protocol.Serializable = (*LoginRequest)(nil)
