package client

import "github.com/monkeyWie/goed2k/protocol"

const (
	opHello             byte = 0x01
	opEmuleInfo         byte = 0x01
	opEmuleInfoAnswer   byte = 0x02
	opQueueRanking      byte = 0x60
	opHelloAnswer       byte = 0x4C
	opSetReqFileID      byte = 0x4F
	opFileStatus        byte = 0x50
	opHashSetRequest    byte = 0x51
	opHashSetAnswer     byte = 0x52
	opStartUploadReq    byte = 0x54
	opAcceptUploadReq   byte = 0x55
	opCancelTransfer    byte = 0x56
	opOutOfPartReqs     byte = 0x57
	opRequestFilename   byte = 0x58
	opReqFilenameAnswer byte = 0x59
	opFileReqAnsNoFil   byte = 0x48
	opRequestParts32    byte = 0x47
	opRequestParts64    byte = 0xA3
	opSendingPart32     byte = 0x46
	opSendingPart64     byte = 0xA2
	opCompressedPart32  byte = 0x40
	opCompressedPart64  byte = 0xA1
)

func NewPacketCombiner() protocol.PacketCombiner {
	pc := protocol.NewPacketCombiner()
	pc.SetServiceSizer(func(header protocol.PacketHeader) int {
		switch header.Key() {
		case protocol.PK(protocol.EdonkeyProt, opSendingPart32):
			return (&SendingPart32{}).BytesCount()
		case protocol.PK(protocol.EMuleProt, opSendingPart64):
			return (&SendingPart64{}).BytesCount()
		case protocol.PK(protocol.EMuleProt, opCompressedPart32):
			return (&CompressedPart32{}).BytesCount()
		case protocol.PK(protocol.EMuleProt, opCompressedPart64):
			return (&CompressedPart64{}).BytesCount()
		default:
			return int(header.SizePacket())
		}
	})
	pc.Register(protocol.PK(protocol.EdonkeyProt, opHello), "client.Hello", func() protocol.Serializable { return &Hello{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opHelloAnswer), "client.HelloAnswer", func() protocol.Serializable { return &HelloAnswer{} })
	pc.Register(protocol.PK(protocol.EMuleProt, opEmuleInfo), "client.ExtHello", func() protocol.Serializable { return &ExtHello{} })
	pc.Register(protocol.PK(protocol.EMuleProt, opEmuleInfoAnswer), "client.ExtHelloAnswer", func() protocol.Serializable { return &ExtHelloAnswer{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opRequestFilename), "client.FileRequest", func() protocol.Serializable { return &FileRequest{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opReqFilenameAnswer), "client.FileAnswer", func() protocol.Serializable { return &FileAnswer{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opSetReqFileID), "client.FileStatusRequest", func() protocol.Serializable { return &FileStatusRequest{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opFileStatus), "client.FileStatusAnswer", func() protocol.Serializable { return &FileStatusAnswer{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opFileReqAnsNoFil), "client.NoFileStatus", func() protocol.Serializable { return &NoFileStatus{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opHashSetRequest), "client.HashSetRequest", func() protocol.Serializable { return &HashSetRequest{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opHashSetAnswer), "client.HashSetAnswer", func() protocol.Serializable { return &HashSetAnswer{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opStartUploadReq), "client.StartUpload", func() protocol.Serializable { return &StartUpload{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opAcceptUploadReq), "client.AcceptUpload", func() protocol.Serializable { return &AcceptUpload{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opCancelTransfer), "client.CancelTransfer", func() protocol.Serializable { return &CancelTransfer{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opOutOfPartReqs), "client.OutOfParts", func() protocol.Serializable { return &OutOfParts{} })
	pc.Register(protocol.PK(protocol.EMuleProt, opQueueRanking), "client.QueueRanking", func() protocol.Serializable { return &QueueRanking{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opRequestParts32), "client.RequestParts32", func() protocol.Serializable { return &RequestParts32{} })
	pc.Register(protocol.PK(protocol.EMuleProt, opRequestParts64), "client.RequestParts64", func() protocol.Serializable { return &RequestParts64{} })
	pc.Register(protocol.PK(protocol.EdonkeyProt, opSendingPart32), "client.SendingPart32", func() protocol.Serializable { return &SendingPart32{} })
	pc.Register(protocol.PK(protocol.EMuleProt, opSendingPart64), "client.SendingPart64", func() protocol.Serializable { return &SendingPart64{} })
	pc.Register(protocol.PK(protocol.EMuleProt, opCompressedPart32), "client.CompressedPart32", func() protocol.Serializable { return &CompressedPart32{} })
	pc.Register(protocol.PK(protocol.EMuleProt, opCompressedPart64), "client.CompressedPart64", func() protocol.Serializable { return &CompressedPart64{} })
	return pc
}
