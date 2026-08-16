package protocol

import "fmt"

type PacketKey struct {
	Protocol byte
	Packet   byte
}

func PK(protocol, packet byte) PacketKey {
	return PacketKey{Protocol: protocol, Packet: packet}
}

func (p PacketKey) Compare(other PacketKey) int {
	if other.Protocol != PackedProt &&
		p.Protocol != PackedProt &&
		other.Protocol != KadCompressedUDP &&
		p.Protocol != KadCompressedUDP {
		if p.Protocol > other.Protocol {
			return 1
		}
		if p.Protocol < other.Protocol {
			return -1
		}
	}
	if p.Packet > other.Packet {
		return 1
	}
	if p.Packet < other.Packet {
		return -1
	}
	return 0
}

func (p PacketKey) NormalizedProtocol() byte {
	if p.Protocol == EdonkeyHeader {
		return PackedProt
	}
	if p.Protocol == EMuleProt {
		return PackedProt
	}
	if p.Protocol == KademliaHeader {
		return KadCompressedUDP
	}
	return p.Protocol
}

func (p PacketKey) String() string {
	return fmt.Sprintf("PK {%02X:%02X}", p.Protocol, p.Packet)
}
