package protocol

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"

	"github.com/monkeyWie/goed2k/internal/logx"
)

type packetFactory func() Serializable
type serviceSizer func(PacketHeader) int

type PacketCombiner struct {
	keyToPacket map[PacketKey]packetFactory
	typeToKey   map[string]PacketKey
	reusable    PacketHeader
	service     serviceSizer
}

func NewPacketCombiner() PacketCombiner {
	return PacketCombiner{
		keyToPacket: make(map[PacketKey]packetFactory),
		typeToKey:   make(map[string]PacketKey),
	}
}

func (p *PacketCombiner) Register(key PacketKey, typeName string, factory packetFactory) {
	p.keyToPacket[key] = factory
	p.typeToKey[typeName] = key
}

func (p *PacketCombiner) SetServiceSizer(fn serviceSizer) {
	p.service = fn
}

func (p PacketCombiner) ServiceSize(header PacketHeader) int {
	if p.service != nil {
		return p.service(header)
	}
	return int(header.SizePacket())
}

func (p *PacketCombiner) Unpack(header PacketHeader, src []byte) (Serializable, error) {
	body := src
	key := header.Key()
	if header.Protocol == PackedProt || header.Protocol == KadCompressedUDP {
		reader, err := zlib.NewReader(bytes.NewReader(src))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		raw, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		body = raw
		if header.Protocol == PackedProt {
			if factory := p.keyToPacket[PK(EMuleProt, header.Packet)]; factory != nil {
				key = PK(EMuleProt, header.Packet)
			} else {
				key = PK(EdonkeyProt, header.Packet)
			}
		}
	}
	factory := p.keyToPacket[key]
	if factory == nil {
		logx.Debug("unsupported packet key", "key", key.String(), "body_len", len(body), "original", header.Key().String())
		packet := &WithoutDataPacket{}
		reader := bytes.NewReader(body)
		if err := packet.Get(reader); err != nil {
			return nil, err
		}
		return packet, nil
	}
	packet := factory()
	reader := bytes.NewReader(body)
	if err := packet.Get(reader); err != nil {
		return nil, err
	}
	return packet, nil
}

func (p *PacketCombiner) UnpackFrame(frame []byte) (PacketHeader, Serializable, error) {
	reader := bytes.NewReader(frame)
	var header PacketHeader
	if err := header.Get(reader); err != nil {
		return PacketHeader{}, nil, err
	}
	body, err := readBytes(reader, int(header.SizePacket()))
	if err != nil {
		return PacketHeader{}, nil, err
	}
	packet, err := p.Unpack(header, body)
	if err != nil {
		return PacketHeader{}, nil, err
	}
	return header, packet, nil
}

func (p *PacketCombiner) Pack(typeName string, object Serializable) ([]byte, error) {
	raw, _, err := p.PackPayload(typeName, object, nil)
	return raw, err
}

func (p *PacketCombiner) PackPayload(typeName string, object Serializable, payload []byte) ([]byte, int, error) {
	key, ok := p.typeToKey[typeName]
	if !ok {
		return nil, 0, errors.New("packet key not registered")
	}
	body := bytes.NewBuffer(make([]byte, 0, object.BytesCount()))
	if err := object.Put(body); err != nil {
		return nil, 0, err
	}
	header := p.reusable
	header.ResetWithKey(key, int32(body.Len()+len(payload)+1))
	out := bytes.NewBuffer(make([]byte, 0, header.BytesCount()+body.Len()+len(payload)))
	if err := header.Put(out); err != nil {
		return nil, 0, err
	}
	if _, err := out.Write(body.Bytes()); err != nil {
		return nil, 0, err
	}
	if len(payload) > 0 {
		if _, err := out.Write(payload); err != nil {
			return nil, 0, err
		}
	}
	return out.Bytes(), header.BytesCount() + body.Len(), nil
}
