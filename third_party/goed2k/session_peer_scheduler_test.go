package goed2k

import (
	"testing"

	"github.com/monkeyWie/goed2k/protocol"
)

func schedulerTestSession(t *testing.T, peerCount int, settings Settings) *Session {
	t.Helper()
	session := NewSession(settings)
	transfer, err := NewTransfer(session, AddTransferParams{
		Hash:       protocol.EMule,
		CreateTime: CurrentTimeMillis(),
		Size:       PieceSize,
	})
	if err != nil {
		t.Fatal(err)
	}
	session.transfers[transfer.hash] = transfer
	for i := 1; i <= peerCount; i++ {
		endpoint := protocol.NewEndpoint(int32(0x01000000|i), 4662)
		if err := transfer.AddPeer(endpoint, int(PeerServer)); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, connection := range session.connections {
			connection.Close(NoError)
		}
	})
	return session
}

func TestPeerSchedulerHonorsSessionConnectionLimit(t *testing.T) {
	settings := NewSettings()
	settings.SessionConnectionsLimit = 3
	settings.MaxConnectionsPerSecond = 10
	session := schedulerTestSession(t, 10, settings)

	session.connectNewPeers(Seconds(100))

	if got := len(session.connections); got != 3 {
		t.Fatalf("created %d connections with a limit of 3", got)
	}
}

func TestPeerSchedulerHonorsPerSecondConnectionLimit(t *testing.T) {
	settings := NewSettings()
	settings.SessionConnectionsLimit = 20
	settings.MaxConnectionsPerSecond = 2
	session := schedulerTestSession(t, 10, settings)

	session.connectNewPeers(Seconds(100))
	session.connectNewPeers(Seconds(100) + 500)
	if got := len(session.connections); got != 2 {
		t.Fatalf("created %d connections inside one second with a limit of 2", got)
	}

	session.connectNewPeers(Seconds(101))
	if got := len(session.connections); got != 4 {
		t.Fatalf("created %d connections after the next rate window, want 4", got)
	}
}
