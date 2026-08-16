package goed2k

import "testing"

func TestPeerInfoSourceLabels(t *testing.T) {
	info := PeerInfo{SourceFlag: int(PeerServer) | int(PeerDHT) | int(PeerResume)}

	labels := info.SourceLabels()
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	if got := info.SourceString(); got != "server|kad|resume" {
		t.Fatalf("unexpected source string: %s", got)
	}
	if !info.HasSource(PeerDHT) {
		t.Fatal("expected kad source flag")
	}
	if info.HasSource(PeerIncoming) {
		t.Fatal("did not expect incoming source flag")
	}
}
