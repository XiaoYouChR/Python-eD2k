package goed2k

import (
	"net"

	"github.com/monkeyWie/goed2k/protocol"
	serverproto "github.com/monkeyWie/goed2k/protocol/server"
)

type ServerConnection struct {
	Connection
	lastPingTime       int64
	handshakeCompleted bool
	identifier         string
	address            *net.TCPAddr
	clientID           int32
	tcpFlags           int32
	auxPort            int32
	combiner           protocol.PacketCombiner
}

func NewServerConnection(identifier string, address *net.TCPAddr, session *Session) *ServerConnection {
	return &ServerConnection{
		Connection:   NewConnection(session),
		lastPingTime: CurrentTime(),
		identifier:   identifier,
		address:      address,
		combiner:     serverproto.NewPacketCombiner(),
	}
}

func (s *ServerConnection) Connect() error {
	if s.address == nil {
		return NewError(InternalError)
	}
	if err := s.Connection.Connect(s.address); err != nil {
		return err
	}
	s.SendLoginRequest()
	return nil
}

func (s *ServerConnection) OnServerIDChange(clientID, tcpFlags, auxPort int32) {
	debugPeerf("server %s <- IdChange clientID=%d", s.identifier, clientID)
	s.clientID = clientID
	s.tcpFlags = tcpFlags
	s.auxPort = auxPort
	s.handshakeCompleted = true
	s.session.OnServerIDChange(s, clientID, tcpFlags, auxPort)
}

func (s *ServerConnection) OnDisconnect(ec BaseErrorCode) {
	debugPeerf("server %s disconnect code=%d", s.identifier, ec.Code())
	s.handshakeCompleted = false
	s.session.OnServerConnectionClosed(s, ec)
}

func (s *ServerConnection) SecondTick(currentSessionTime int64) {
	s.Connection.SecondTick(currentSessionTime)
	if s.session.settings.ServerPingTimeout > 0 {
		currentTime := CurrentTime()
		if s.MillisecondsSinceLastReceive() > s.session.settings.ServerPingTimeout*1500 {
			s.Close(ConnectionTimeout)
		} else if currentTime-s.lastPingTime > s.session.settings.ServerPingTimeout*1000 {
			s.lastPingTime = currentTime
			s.SendGetList()
		}
	}
}

func (s *ServerConnection) Endpoint() protocol.Endpoint {
	return protocol.Endpoint{}
}

func (s *ServerConnection) SendFileSourcesRequest(hash protocol.Hash, size int64) {
	debugPeerf("server %s -> GetFileSources %s size=%d", s.identifier, hash.String(), size)
	packet := serverproto.GetFileSources{
		Hash:    hash,
		LowPart: int32(LowPart(size)),
		HiPart:  int32(HiPart(size)),
	}
	if raw, err := s.combiner.Pack("server.GetFileSources", &packet); err == nil {
		s.QueuePacket(raw)
	}
}

func (s *ServerConnection) SendSearchRequest(packet *serverproto.SearchRequest) {
	if packet == nil {
		return
	}
	debugPeerf("server %s -> SearchRequest query=%q", s.identifier, packet.Query)
	if raw, err := s.combiner.Pack("server.SearchRequest", packet); err == nil {
		s.QueuePacket(raw)
	}
}

func (s *ServerConnection) SendSearchMore() {
	debugPeerf("server %s -> SearchMore", s.identifier)
	packet := serverproto.SearchMore{}
	if raw, err := s.combiner.Pack("server.SearchMore", &packet); err == nil {
		s.QueuePacket(raw)
	}
}

func (s *ServerConnection) SendLoginRequest() {
	debugPeerf("server %s -> LoginRequest", s.identifier)
	packet := serverproto.NewLoginRequest(s.session.GetUserAgent(), s.session.GetListenPort(), s.session.GetClientName())
	if raw, err := s.combiner.Pack("server.LoginRequest", &packet); err == nil {
		s.QueuePacket(raw)
	}
}

func (s *ServerConnection) SendCallbackRequest(clientID int32) {
	packet := serverproto.CallbackRequest{ClientID: clientID}
	if raw, err := s.combiner.Pack("server.CallbackRequest", &packet); err == nil {
		s.QueuePacket(raw)
	}
}

func (s *ServerConnection) SendGetList() {
	packet := serverproto.GetList{}
	if raw, err := s.combiner.Pack("server.GetList", &packet); err == nil {
		s.QueuePacket(raw)
	}
}

func (s *ServerConnection) SendOfferFiles(packet *serverproto.OfferFiles) {
	if packet == nil || len(packet.Entries) == 0 {
		return
	}
	debugPeerf("server %s -> OfferFiles count=%d", s.identifier, len(packet.Entries))
	if raw, err := s.combiner.Pack("server.OfferFiles", packet); err == nil {
		s.QueuePacket(raw)
	}
}

func (s *ServerConnection) ProcessIncoming() error {
	_, packets, err := s.DecodeFrames(&s.combiner)
	if err != nil {
		return err
	}
	for _, packet := range packets {
		switch value := packet.(type) {
		case *serverproto.IdChange:
			s.OnServerIDChange(value.ClientID, value.TCPFlags, value.AuxPort)
		case *serverproto.FoundFileSources:
			debugPeerf("server %s <- FoundFileSources hash=%s count=%d", s.identifier, value.Hash.String(), len(value.Sources))
			if transfer := s.session.LookupTransfer(value.Hash); transfer != nil {
				for _, ep := range value.Sources {
					if !IsLowID(ep.IP()) {
						_ = transfer.AddPeer(ep, int(PeerServer))
					}
				}
			}
		case *serverproto.CallbackRequestIncoming:
			debugPeerf("server %s <- CallbackRequestIncoming", s.identifier)
		case *serverproto.CallbackRequestFailed:
			debugPeerf("server %s <- CallbackRequestFailed", s.identifier)
		case *serverproto.Status:
			debugPeerf("server %s <- Status", s.identifier)
		case *serverproto.Message:
			debugPeerf("server %s <- Message %q", s.identifier, value.AsString())
		case *serverproto.SearchResult:
			debugPeerf("server %s <- SearchResult count=%d more=%t", s.identifier, len(value.Results), value.MoreResults)
			s.session.OnServerSearchResult(s, value)
		}
	}
	return nil
}

func (s *ServerConnection) GetIdentifier() string {
	return s.identifier
}

func (s *ServerConnection) GetAddress() *net.TCPAddr {
	return s.address
}

func (s *ServerConnection) IsHandshakeCompleted() bool {
	return s.handshakeCompleted
}

func (s *ServerConnection) ClientID() int32 {
	return s.clientID
}

func (s *ServerConnection) TCPFlags() int32 {
	return s.tcpFlags
}

func (s *ServerConnection) AuxPort() int32 {
	return s.auxPort
}
