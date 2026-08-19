package server

import "github.com/monkeyWie/goed2k/protocol"

const (
	opGetServerList   byte = 0x14
	opOfferFiles      byte = 0x15
	opSearchRequest   byte = 0x16
	opGetSources      byte = 0x19
	opCallbackRequest byte = 0x1C
	opQueryMore       byte = 0x21
	opSearchResult    byte = 0x33
	opServerStatus    byte = 0x34
	opCallbackReqIn   byte = 0x35
	opCallbackFail    byte = 0x36
	opServerMessage   byte = 0x38
	opIDChange        byte = 0x40
	opFoundSources    byte = 0x42
)

func NewPacketCombiner() protocol.PacketCombiner {
	pc := protocol.NewPacketCombiner()
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opLoginRequest), "server.LoginRequest", func() protocol.Serializable { return &LoginRequest{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opGetServerList), "server.GetList", func() protocol.Serializable { return &GetList{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opOfferFiles), "server.OfferFiles", func() protocol.Serializable { return &OfferFiles{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opSearchRequest), "server.SearchRequest", func() protocol.Serializable { return &SearchRequest{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opGetSources), "server.GetFileSources", func() protocol.Serializable { return &GetFileSources{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opQueryMore), "server.SearchMore", func() protocol.Serializable { return &SearchMore{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opCallbackRequest), "server.CallbackRequest", func() protocol.Serializable { return &CallbackRequest{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opSearchResult), "server.SearchResult", func() protocol.Serializable { return &SearchResult{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opServerStatus), "server.Status", func() protocol.Serializable { return &Status{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opCallbackReqIn), "server.CallbackRequestIncoming", func() protocol.Serializable { return &CallbackRequestIncoming{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opCallbackFail), "server.CallbackRequestFailed", func() protocol.Serializable { return &CallbackRequestFailed{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opServerMessage), "server.Message", func() protocol.Serializable { return &Message{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opIDChange), "server.IdChange", func() protocol.Serializable { return &IdChange{} })
	pc.Register(protocol.PK(protocol.EdonkeyHeader, opFoundSources), "server.FoundFileSources", func() protocol.Serializable { return &FoundFileSources{} })
	return pc
}
