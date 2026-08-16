package goed2k

import "github.com/monkeyWie/goed2k/protocol"

type Peer struct {
	LastConnected  int64
	NextConnection int64
	FailCount      int
	Connectable    bool
	SourceFlag     int
	Connection     any
	Endpoint       protocol.Endpoint
}

func NewPeer(ep protocol.Endpoint) Peer {
	return Peer{Endpoint: ep}
}

func NewPeerWithSource(ep protocol.Endpoint, conn bool, sourceFlag int) Peer {
	return Peer{Endpoint: ep, Connectable: conn, SourceFlag: sourceFlag}
}

func (p Peer) Compare(other Peer) int {
	return p.Endpoint.Compare(other.Endpoint)
}

func (p Peer) Equal(other Peer) bool {
	return p.Endpoint.Equal(other.Endpoint)
}
