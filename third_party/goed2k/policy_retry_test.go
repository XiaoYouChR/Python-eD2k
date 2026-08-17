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

func TestPolicyHonorsConfiguredPeerLimit(t *testing.T) {
	settings := NewSettings()
	settings.MaxPeerListSize = 2
	transfer := &Transfer{session: NewSession(settings)}
	policy := NewPolicy(transfer)

	for i, address := range []string{"1.2.3.1", "1.2.3.2"} {
		endpoint, err := protocol.EndpointFromString(address, 4662)
		if err != nil {
			t.Fatalf("endpoint %d: %v", i+1, err)
		}
		if _, err := policy.AddPeer(NewPeerWithSource(endpoint, true, int(PeerServer))); err != nil {
			t.Fatalf("add peer %d: %v", i+1, err)
		}
	}

	endpoint, err := protocol.EndpointFromString("1.2.3.3", 4662)
	if err != nil {
		t.Fatalf("endpoint 3: %v", err)
	}
	if _, err := policy.AddPeer(NewPeerWithSource(endpoint, true, int(PeerServer))); err == nil {
		t.Fatal("expected configured peer limit to reject the third peer")
	}
}
