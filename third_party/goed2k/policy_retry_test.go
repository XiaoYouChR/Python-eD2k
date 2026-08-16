package goed2k

import (
	"testing"

	"github.com/monkeyWie/goed2k/protocol"
)

func TestFindConnectCandidateHonorsRetryDeadline(t *testing.T) {
	endpoint, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	policy := NewPolicy(&Transfer{})
	peer := NewPeerWithSource(endpoint, true, int(PeerServer))
	now := int64(100_000)
	peer.NextConnection = now + Seconds(10)
	if _, err := policy.AddPeer(peer); err != nil {
		t.Fatalf("add peer: %v", err)
	}

	if candidate := policy.FindConnectCandidate(now); candidate != nil {
		t.Fatal("peer became eligible before its retry deadline")
	}

	policy.peers[0].NextConnection = now - 1
	if candidate := policy.FindConnectCandidate(now); candidate == nil {
		t.Fatal("peer did not become eligible after its retry deadline")
	}
}
