package kad

import "errors"

type PacketCombiner struct{}

func (PacketCombiner) Unpack(packet []byte) (byte, any, error) {
	opcode, payload, err := DecodePacket(packet)
	if err != nil {
		return 0, nil, err
	}
	msg, err := unpackByOpcode(opcode, payload)
	if err != nil {
		return 0, nil, err
	}
	return opcode, msg, nil
}

func (PacketCombiner) Pack(packet any, extra ...byte) ([]byte, error) {
	switch p := packet.(type) {
	case BootstrapReq:
		return p.Pack()
	case BootstrapRes:
		return p.Pack()
	case SearchRes:
		return p.Pack()
	case SearchSourcesReq:
		return p.Pack()
	case SearchKeysReq:
		return p.Pack()
	case SearchNotesReq:
		return p.Pack()
	case Req:
		return p.Pack()
	case Hello:
		opcode := HelloReqOp
		if len(extra) > 0 && extra[0] != 0 {
			opcode = extra[0]
		}
		return p.Pack(opcode)
	case Res:
		return p.Pack()
	case PublishSourcesReq:
		return p.Pack()
	case PublishKeysReq:
		return p.Pack()
	case PublishNotesReq:
		return p.Pack()
	case PublishRes:
		return p.Pack()
	case PublishNotesRes:
		return p.Pack()
	case Pong:
		return p.Pack()
	case Ping:
		return p.Pack()
	case FirewalledReq:
		return p.Pack()
	case FirewalledRes:
		return p.Pack()
	case FirewalledUDP:
		return p.Pack()
	case HelloResAck:
		return p.Pack()
	case PublishResAck:
		return p.Pack()
	case CallbackReq:
		return p.Pack()
	case FindBuddyReq:
		return p.Pack()
	case FindBuddyRes:
		return p.Pack()
	case SearchNotesRes:
		return p.Pack()
	case LegacyFirewalledReq:
		return p.Pack()
	default:
		return nil, errors.New("unsupported kad packet type")
	}
}

func unpackByOpcode(opcode byte, payload []byte) (any, error) {
	var msg any
	switch opcode {
	case SearchResOp:
		msg = &SearchRes{}
	case SearchSrcReqOp:
		msg = &SearchSourcesReq{}
	case SearchKeysReqOp:
		msg = &SearchKeysReq{}
	case BootstrapResOp:
		msg = &BootstrapRes{}
	case ResOp:
		msg = &Res{}
	case HelloReqOp, HelloResOp:
		msg = &Hello{}
	case BootstrapReqOp:
		msg = &BootstrapReq{}
	case ReqOp:
		msg = &Req{}
	case PublishSourceReqOp:
		msg = &PublishSourcesReq{}
	case PublishKeysReqOp:
		msg = &PublishKeysReq{}
	case PublishNotesReqOp:
		msg = &PublishNotesReq{}
	case PublishResOp:
		msg = &PublishRes{}
	case PublishNotesResOp:
		msg = &PublishNotesRes{}
	case PingOp:
		msg = &Ping{}
	case PongOp:
		msg = &Pong{}
	case FirewalledReqOp:
		msg = &FirewalledReq{}
	case FirewalledResOp:
		msg = &FirewalledRes{}
	case FirewalledUdpOp:
		msg = &FirewalledUDP{}
	case SearchNotesReqOp:
		msg = &SearchNotesReq{}
	case SearchNotesResOp:
		msg = &SearchNotesRes{}
	case HelloResAckOp:
		msg = &HelloResAck{}
	case PublishResAckOp:
		msg = &PublishResAck{}
	case CallbackReqOp:
		msg = &CallbackReq{}
	case FindBuddyReqOp:
		msg = &FindBuddyReq{}
	case FindBuddyResOp:
		msg = &FindBuddyRes{}
	case LegacyFirewalledReqOp:
		msg = &LegacyFirewalledReq{}
	default:
		return nil, errors.New("unsupported kad opcode")
	}
	switch p := msg.(type) {
	case *BootstrapReq, *Ping, *SearchNotesRes, *HelloResAck, *PublishResAck, *CallbackReq, *FindBuddyReq, *FindBuddyRes:
		return msg, nil
	case *SearchRes:
		return msg, p.Unpack(payload)
	case *SearchSourcesReq:
		return msg, p.Unpack(payload)
	case *SearchKeysReq:
		return msg, p.Unpack(payload)
	case *BootstrapRes:
		return msg, p.Unpack(payload)
	case *Res:
		return msg, p.Unpack(payload)
	case *Hello:
		return msg, p.Unpack(payload)
	case *Req:
		return msg, p.Unpack(payload)
	case *PublishSourcesReq:
		return msg, p.Unpack(payload)
	case *PublishKeysReq:
		return msg, p.Unpack(payload)
	case *PublishNotesReq:
		return msg, p.Unpack(payload)
	case *PublishRes:
		return msg, p.Unpack(payload)
	case *PublishNotesRes:
		return msg, p.Unpack(payload)
	case *Pong:
		return msg, p.Unpack(payload)
	case *FirewalledReq:
		return msg, p.Unpack(payload)
	case *FirewalledRes:
		return msg, p.Unpack(payload)
	case *FirewalledUDP:
		return msg, p.Unpack(payload)
	case *SearchNotesReq:
		return msg, p.Unpack(payload)
	case *LegacyFirewalledReq:
		return msg, p.Unpack(payload)
	default:
		return nil, errors.New("unsupported kad packet payload")
	}
}
