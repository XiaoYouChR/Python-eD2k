package protocol

import (
	"bytes"
	"fmt"
)

const (
	OpUndefined      byte = 0x00
	EdonkeyHeader    byte = 0xE3
	EdonkeyProt      byte = 0xE3
	PackedProt       byte = 0xD4
	EMuleProt        byte = 0xC5
	KadCompressedUDP byte = 0xE5
	KademliaHeader   byte = 0xE4

	PacketHeaderSize = 6
)

type PacketHeader struct {
	Protocol byte
	Size     int32
	Packet   byte
}

func (p PacketHeader) IsDefined() bool {
	return p.Protocol != OpUndefined && p.Packet != OpUndefined
}

func (p *PacketHeader) Reset() {
	p.Protocol = OpUndefined
	p.Size = 0
	p.Packet = OpUndefined
}

func (p *PacketHeader) ResetWithKey(key PacketKey, size int32) {
	p.Protocol = key.Protocol
	p.Size = size
	p.Packet = key.Packet
}

func (p PacketHeader) String() string {
	return fmt.Sprintf("packet header {%02X:%d:%02X}", p.Protocol, p.Size, p.Packet)
}

func (p *PacketHeader) Get(src *bytes.Reader) error {
	protocol, err := readByte(src)
	if err != nil {
		return err
	}
	size, err := readInt32(src)
	if err != nil {
		return err
	}
	packet, err := readByte(src)
	if err != nil {
		return err
	}
	p.Protocol = protocol
	p.Size = size
	p.Packet = packet
	return nil
}

func (p PacketHeader) Put(dst *bytes.Buffer) error {
	if err := writeByte(dst, p.Protocol); err != nil {
		return err
	}
	if err := writeInt32(dst, p.Size); err != nil {
		return err
	}
	return writeByte(dst, p.Packet)
}

func (p PacketHeader) BytesCount() int {
	return PacketHeaderSize
}

func (p PacketHeader) SizePacket() int32 {
	return p.Size - 1
}

func (p PacketHeader) Key() PacketKey {
	return PacketKey{Protocol: p.Protocol, Packet: p.Packet}
}
