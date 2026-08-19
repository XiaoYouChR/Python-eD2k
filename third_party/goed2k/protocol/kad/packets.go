package kad

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

const (
	BootstrapReqOp        byte = 0x01
	BootstrapResOp        byte = 0x09
	ReqOp                 byte = 0x21
	ResOp                 byte = 0x29
	HelloReqOp            byte = 0x11
	HelloResOp            byte = 0x19
	HelloResAckOp         byte = 0x22
	SearchKeysReqOp       byte = 0x33
	SearchNotesResOp      byte = 0x3A
	CallbackReqOp         byte = 0x52
	FindBuddyReqOp        byte = 0x51
	FindBuddyResOp        byte = 0x5A
	LegacyFirewalledReqOp byte = 0x50
	FirewalledReqOp       byte = 0x53
	FirewalledResOp       byte = 0x58
	FirewalledUdpOp       byte = 0x62
	PingOp                byte = 0x60
	PongOp                byte = 0x61
	PublishSourceReqOp    byte = 0x44
	PublishKeysReqOp      byte = 0x43
	PublishNotesReqOp     byte = 0x45
	PublishNotesResOp     byte = 0x4A
	PublishResOp          byte = 0x4B
	PublishResAckOp       byte = 0x4C
	SearchNotesReqOp      byte = 0x35

	KademliaVersion byte = 0x05

	FindValue byte = 0x02
	Store     byte = 0x04
	FindNode  byte = 0x0B
)

func encodePacket(opcode byte, payload []byte) []byte {
	packet := make([]byte, 0, 2+len(payload))
	packet = append(packet, ProtocolHeader, opcode)
	packet = append(packet, payload...)
	return packet
}

type EmptyPacket struct {
	Opcode byte
}

func (e EmptyPacket) Pack() ([]byte, error) {
	return encodePacket(e.Opcode, nil), nil
}

type HelloResAck struct{}

func (HelloResAck) Pack() ([]byte, error) { return encodePacket(HelloResAckOp, nil), nil }

type PublishResAck struct{}

func (PublishResAck) Pack() ([]byte, error) { return encodePacket(PublishResAckOp, nil), nil }

type CallbackReq struct{}

func (CallbackReq) Pack() ([]byte, error) { return encodePacket(CallbackReqOp, nil), nil }

type FindBuddyReq struct{}

func (FindBuddyReq) Pack() ([]byte, error) { return encodePacket(FindBuddyReqOp, nil), nil }

type FindBuddyRes struct{}

func (FindBuddyRes) Pack() ([]byte, error) { return encodePacket(FindBuddyResOp, nil), nil }

type SearchNotesRes struct{}

func (SearchNotesRes) Pack() ([]byte, error) { return encodePacket(SearchNotesResOp, nil), nil }

type LegacyFirewalledReq struct {
	TCPPort uint16
}

func (f *LegacyFirewalledReq) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	port, err := protocol.ReadUInt16(reader)
	if err != nil {
		return err
	}
	f.TCPPort = port
	return nil
}

func (f LegacyFirewalledReq) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := protocol.WriteUInt16(&payload, f.TCPPort); err != nil {
		return nil, err
	}
	return encodePacket(LegacyFirewalledReqOp, payload.Bytes()), nil
}

type BootstrapReq struct{}

func (BootstrapReq) Pack() ([]byte, error) {
	return encodePacket(BootstrapReqOp, nil), nil
}

type BootstrapRes struct {
	ID       ID
	TCPPort  uint16
	Version  byte
	Contacts []Entry
}

func (b *BootstrapRes) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	if err := b.ID.Get(reader); err != nil {
		return err
	}
	port, err := protocol.ReadUInt16(reader)
	if err != nil {
		return err
	}
	b.TCPPort = port
	version, err := reader.ReadByte()
	if err != nil {
		return err
	}
	b.Version = version
	count, err := protocol.ReadUInt16(reader)
	if err != nil {
		return err
	}
	b.Contacts = make([]Entry, int(count))
	for i := 0; i < int(count); i++ {
		if err := b.Contacts[i].Get(reader); err != nil {
			return err
		}
	}
	return nil
}

func (b BootstrapRes) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := b.ID.Put(&payload); err != nil {
		return nil, err
	}
	if err := protocol.WriteUInt16(&payload, b.TCPPort); err != nil {
		return nil, err
	}
	if err := payload.WriteByte(b.Version); err != nil {
		return nil, err
	}
	if err := protocol.WriteUInt16(&payload, uint16(len(b.Contacts))); err != nil {
		return nil, err
	}
	for _, contact := range b.Contacts {
		if err := contact.Put(&payload); err != nil {
			return nil, err
		}
	}
	return encodePacket(BootstrapResOp, payload.Bytes()), nil
}

type Hello struct {
	ID      ID
	TCPPort uint16
	Version byte
	Tags    []Tag
}

func (h *Hello) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	if err := h.ID.Get(reader); err != nil {
		return err
	}
	port, err := protocol.ReadUInt16(reader)
	if err != nil {
		return err
	}
	h.TCPPort = port
	version, err := reader.ReadByte()
	if err != nil {
		return err
	}
	h.Version = version
	count, err := reader.ReadByte()
	if err != nil {
		return err
	}
	h.Tags = make([]Tag, int(count))
	for i := 0; i < int(count); i++ {
		if err := h.Tags[i].Get(reader); err != nil {
			return err
		}
	}
	return nil
}

func (h Hello) Pack(opcode byte) ([]byte, error) {
	var payload bytes.Buffer
	if err := h.ID.Put(&payload); err != nil {
		return nil, err
	}
	if err := protocol.WriteUInt16(&payload, h.TCPPort); err != nil {
		return nil, err
	}
	version := h.Version
	if version == 0 {
		version = KademliaVersion
	}
	if err := payload.WriteByte(version); err != nil {
		return nil, err
	}
	if err := payload.WriteByte(byte(len(h.Tags))); err != nil {
		return nil, err
	}
	for _, tag := range h.Tags {
		if err := tag.Put(&payload); err != nil {
			return nil, err
		}
	}
	return encodePacket(opcode, payload.Bytes()), nil
}

type Req struct {
	SearchType byte
	Target     ID
	Receiver   ID
}

func (r Req) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := payload.WriteByte(r.SearchType); err != nil {
		return nil, err
	}
	if err := r.Target.Put(&payload); err != nil {
		return nil, err
	}
	if err := r.Receiver.Put(&payload); err != nil {
		return nil, err
	}
	return encodePacket(ReqOp, payload.Bytes()), nil
}

func (r *Req) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	op, err := reader.ReadByte()
	if err != nil {
		return err
	}
	r.SearchType = op
	if err := r.Target.Get(reader); err != nil {
		return err
	}
	return r.Receiver.Get(reader)
}

type Res struct {
	Target  ID
	Results []Entry
}

func (r *Res) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	if err := r.Target.Get(reader); err != nil {
		return err
	}
	count, err := reader.ReadByte()
	if err != nil {
		return err
	}
	r.Results = make([]Entry, int(count))
	for i := 0; i < int(count); i++ {
		if err := r.Results[i].Get(reader); err != nil {
			return err
		}
	}
	return nil
}

func (r Res) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := r.Target.Put(&payload); err != nil {
		return nil, err
	}
	if err := payload.WriteByte(byte(len(r.Results))); err != nil {
		return nil, err
	}
	for _, entry := range r.Results {
		if err := entry.Put(&payload); err != nil {
			return nil, err
		}
	}
	return encodePacket(ResOp, payload.Bytes()), nil
}

type Ping struct{}

func (Ping) Pack() ([]byte, error) {
	return encodePacket(PingOp, nil), nil
}

type Pong struct {
	UDPPort uint16
}

func (p *Pong) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	port, err := protocol.ReadUInt16(reader)
	if err != nil {
		return err
	}
	p.UDPPort = port
	return nil
}

func (p Pong) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := protocol.WriteUInt16(&payload, p.UDPPort); err != nil {
		return nil, err
	}
	return encodePacket(PongOp, payload.Bytes()), nil
}

type PublishSourcesReq struct {
	FileID ID
	Source SearchEntry
}

func (p *PublishSourcesReq) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	if err := p.FileID.Get(reader); err != nil {
		return err
	}
	return p.Source.Get(reader)
}

func (p PublishSourcesReq) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := p.FileID.Put(&payload); err != nil {
		return nil, err
	}
	if err := p.Source.Put(&payload); err != nil {
		return nil, err
	}
	return encodePacket(PublishSourceReqOp, payload.Bytes()), nil
}

type PublishRes struct {
	FileID ID
	Count  byte
}

type SearchKeysReq struct {
	Target   ID
	StartPos uint16
}

func (s *SearchKeysReq) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	if err := s.Target.Get(reader); err != nil {
		return err
	}
	startPos, err := protocol.ReadUInt16(reader)
	if err != nil {
		return err
	}
	s.StartPos = startPos
	return nil
}

func (s SearchKeysReq) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := s.Target.Put(&payload); err != nil {
		return nil, err
	}
	if err := protocol.WriteUInt16(&payload, s.StartPos); err != nil {
		return nil, err
	}
	return encodePacket(SearchKeysReqOp, payload.Bytes()), nil
}

type PublishKeysReq struct {
	KeywordID ID
	Sources   []SearchEntry
}

func (p *PublishKeysReq) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	if err := p.KeywordID.Get(reader); err != nil {
		return err
	}
	count, err := protocol.ReadUInt16(reader)
	if err != nil {
		return err
	}
	p.Sources = make([]SearchEntry, int(count))
	for i := 0; i < int(count); i++ {
		if err := p.Sources[i].Get(reader); err != nil {
			return err
		}
	}
	return nil
}

func (p PublishKeysReq) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := p.KeywordID.Put(&payload); err != nil {
		return nil, err
	}
	if err := protocol.WriteUInt16(&payload, uint16(len(p.Sources))); err != nil {
		return nil, err
	}
	for _, source := range p.Sources {
		if err := source.Put(&payload); err != nil {
			return nil, err
		}
	}
	return encodePacket(PublishKeysReqOp, payload.Bytes()), nil
}

type PublishNotesReq struct {
	FileID ID
	Notes  []SearchEntry
}

func (p *PublishNotesReq) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	if err := p.FileID.Get(reader); err != nil {
		return err
	}
	count, err := protocol.ReadUInt16(reader)
	if err != nil {
		return err
	}
	p.Notes = make([]SearchEntry, int(count))
	for i := 0; i < int(count); i++ {
		if err := p.Notes[i].Get(reader); err != nil {
			return err
		}
	}
	return nil
}

func (p PublishNotesReq) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := p.FileID.Put(&payload); err != nil {
		return nil, err
	}
	if err := protocol.WriteUInt16(&payload, uint16(len(p.Notes))); err != nil {
		return nil, err
	}
	for _, note := range p.Notes {
		if err := note.Put(&payload); err != nil {
			return nil, err
		}
	}
	return encodePacket(PublishNotesReqOp, payload.Bytes()), nil
}

type SearchNotesReq struct {
	Target   ID
	FileSize uint64
}

func (s *SearchNotesReq) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	if err := s.Target.Get(reader); err != nil {
		return err
	}
	fileSize, err := protocol.ReadUInt64(reader)
	if err != nil {
		return err
	}
	s.FileSize = fileSize
	return nil
}

func (s SearchNotesReq) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := s.Target.Put(&payload); err != nil {
		return nil, err
	}
	if err := protocol.WriteUInt64(&payload, s.FileSize); err != nil {
		return nil, err
	}
	return encodePacket(SearchNotesReqOp, payload.Bytes()), nil
}

type FirewalledReq struct {
	TCPPort uint16
	ID      ID
	Options byte
}

func (f *FirewalledReq) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	port, err := protocol.ReadUInt16(reader)
	if err != nil {
		return err
	}
	f.TCPPort = port
	if err := f.ID.Get(reader); err != nil {
		return err
	}
	options, err := reader.ReadByte()
	if err != nil {
		return err
	}
	f.Options = options
	return nil
}

func (f FirewalledReq) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := protocol.WriteUInt16(&payload, f.TCPPort); err != nil {
		return nil, err
	}
	if err := f.ID.Put(&payload); err != nil {
		return nil, err
	}
	if err := payload.WriteByte(f.Options); err != nil {
		return nil, err
	}
	return encodePacket(FirewalledReqOp, payload.Bytes()), nil
}

type FirewalledRes struct {
	IP uint32
}

func (f *FirewalledRes) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	ip, err := protocol.ReadUInt32(reader)
	if err != nil {
		return err
	}
	f.IP = ip
	return nil
}

func (f FirewalledRes) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := protocol.WriteUInt32(&payload, f.IP); err != nil {
		return nil, err
	}
	return encodePacket(FirewalledResOp, payload.Bytes()), nil
}

type FirewalledUDP struct {
	ErrorCode byte
	TCPPort   uint16
}

func (f *FirewalledUDP) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	code, err := reader.ReadByte()
	if err != nil {
		return err
	}
	f.ErrorCode = code
	port, err := protocol.ReadUInt16(reader)
	if err != nil {
		return err
	}
	f.TCPPort = port
	return nil
}

func (f FirewalledUDP) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := payload.WriteByte(f.ErrorCode); err != nil {
		return nil, err
	}
	if err := protocol.WriteUInt16(&payload, f.TCPPort); err != nil {
		return nil, err
	}
	return encodePacket(FirewalledUdpOp, payload.Bytes()), nil
}

func (p *PublishRes) Unpack(payload []byte) error {
	reader := bytes.NewReader(payload)
	if err := p.FileID.Get(reader); err != nil {
		return err
	}
	count, err := reader.ReadByte()
	if err != nil {
		return err
	}
	p.Count = count
	return nil
}

func (p PublishRes) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := p.FileID.Put(&payload); err != nil {
		return nil, err
	}
	if err := payload.WriteByte(p.Count); err != nil {
		return nil, err
	}
	return encodePacket(PublishResOp, payload.Bytes()), nil
}

type PublishNotesRes struct {
	PublishRes
}

func (p PublishNotesRes) Pack() ([]byte, error) {
	var payload bytes.Buffer
	if err := p.FileID.Put(&payload); err != nil {
		return nil, err
	}
	if err := payload.WriteByte(p.Count); err != nil {
		return nil, err
	}
	return encodePacket(PublishNotesResOp, payload.Bytes()), nil
}
