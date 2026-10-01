package daemon

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	goed2k "github.com/monkeyWie/goed2k"
)

func TestToTransferIncludesActiveAndTotalPeers(t *testing.T) {
	got := toTransfer(goed2k.TransferSnapshot{
		Status: goed2k.TransferStatus{
			ActivePeers: 2,
			NumPeers:    10,
		},
	})
	if got.ActivePeers != 2 || got.Peers != 10 {
		t.Fatalf("peer counts = %d/%d, want 2/10", got.ActivePeers, got.Peers)
	}
}

func TestConnectServersBestEffortContinuesAfterFailure(t *testing.T) {
	var attempted []string
	err := connectServersBestEffort([]string{"dead:1", "live:2", "later:3"}, func(address string) error {
		attempted = append(attempted, address)
		if address == "dead:1" {
			return errors.New("timed out")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("connectServersBestEffort() error = %v", err)
	}
	if want := []string{"dead:1", "live:2", "later:3"}; !reflect.DeepEqual(attempted, want) {
		t.Fatalf("attempted = %v, want %v", attempted, want)
	}
}

func TestConnectServersBestEffortFailsWhenEveryServerFails(t *testing.T) {
	err := connectServersBestEffort([]string{"first:1", "second:2"}, func(address string) error {
		return errors.New("timed out")
	})

	if err == nil {
		t.Fatal("connectServersBestEffort() error = nil")
	}
	for _, address := range []string{"first:1", "second:2"} {
		if !strings.Contains(err.Error(), address) {
			t.Fatalf("error %q does not contain %q", err, address)
		}
	}
}

func TestConnectServersBestEffortRejectsAnEmptyList(t *testing.T) {
	err := connectServersBestEffort(nil, func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "no server address") {
		t.Fatalf("connectServersBestEffort() error = %v", err)
	}
}

func TestServerMetAddressesMergesEverySourceAndSkipsFailures(t *testing.T) {
	lists := map[string][]string{
		"a.met": {"1.1.1.1:4661", "2.2.2.2:4661"},
		"c.met": {"2.2.2.2:4661", "3.3.3.3:4661"},
	}
	got, failures := serverMetAddresses(" a.met, dead.met ,c.met,", func(source string) ([]string, error) {
		if addresses, ok := lists[source]; ok {
			return addresses, nil
		}
		return nil, errors.New("unreachable")
	})

	if want := []string{"1.1.1.1:4661", "2.2.2.2:4661", "3.3.3.3:4661"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("addresses = %v, want %v", got, want)
	}
	if len(failures) != 1 || !strings.Contains(failures[0].Error(), "dead.met") {
		t.Fatalf("failures = %v", failures)
	}
}

func TestToSnapshotReportsNetworkAndUploadTotals(t *testing.T) {
	got := toSnapshot(goed2k.ClientStatus{
		Servers: []goed2k.ServerSnapshot{
			{Connected: true},
			{Connected: true, HandshakeCompleted: true},
		},
		Transfers: []goed2k.TransferSnapshot{{Status: goed2k.TransferStatus{Upload: 4096}}},
	}, goed2k.DHTStatus{LiveNodes: 134})

	if !got.ServerConnected || got.KadNodes != 134 || got.Transfers[0].Upload != 4096 {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestToSnapshotNeedsACompletedServerHandshake(t *testing.T) {
	got := toSnapshot(goed2k.ClientStatus{
		Servers: []goed2k.ServerSnapshot{{Connected: true}},
	}, goed2k.DHTStatus{})

	if got.ServerConnected {
		t.Fatal("serverConnected = true before handshake")
	}
}
