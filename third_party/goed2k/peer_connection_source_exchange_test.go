package goed2k

import (
	"testing"

	"github.com/monkeyWie/goed2k/protocol"
	clientproto "github.com/monkeyWie/goed2k/protocol/client"
)

func TestPeerConnectionRequestsAndAddsSourceExchangePeers(t *testing.T) {
	session, transfer := newTestTransfer(t)
	remote, err := protocol.EndpointFromString("8.8.8.8", 4662)
	if err != nil {
		t.Fatal(err)
	}
	peer := NewPeerWithSource(remote, true, int(PeerServer))
	connection := NewPeerConnection(session, remote, transfer, &peer)
	connection.applyRemoteHello(&clientproto.HelloAnswer{Properties: protocol.TagList{
		protocol.NewUInt32Tag(0xFA, uint32(MiscOptions{SourceExchange1Ver: 3}.IntValue())),
	}})

	connection.SendSourceExchangeRequest(transfer.GetHash())
	if !connection.sourceRequestSent || len(connection.PendingPackets()) == 0 {
		t.Fatal("source exchange request was not queued")
	}

	connection.HandleSourceExchangeAnswer(&clientproto.SourceExchangeAnswer{
		Hash: transfer.GetHash(),
		Sources: []clientproto.SourceExchangeSource{{
			ClientID:   0x01020304,
			ClientPort: 4662,
		}},
	})
	want, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatal(err)
	}
	added := transfer.policy.FindPeer(want)
	if added == nil || added.SourceFlag&int(PeerExchange) == 0 {
		t.Fatalf("source exchange peer was not added: %+v", added)
	}
}
