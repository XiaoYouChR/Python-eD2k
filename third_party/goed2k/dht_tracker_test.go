package goed2k

import (
	"net"
	"testing"
	"time"

	"github.com/monkeyWie/goed2k/protocol"
	kadproto "github.com/monkeyWie/goed2k/protocol/kad"
)

func TestMaybeSendHelloLockedDoesNotRelockTracker(t *testing.T) {
	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	sender, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()

	tracker := NewDHTTracker(0, 0)
	tracker.conn = sender
	tracker.listenPort = sender.LocalAddr().(*net.UDPAddr).Port
	node := &KadRoutingNode{Addr: receiver.LocalAddr().(*net.UDPAddr)}
	done := make(chan struct{})
	go func() {
		tracker.mu.Lock()
		tracker.maybeSendHelloLocked(node)
		tracker.mu.Unlock()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("maybeSendHelloLocked deadlocked while the tracker mutex was held")
	}
}

func TestDHTTrackerBootstrapResponseAddsContacts(t *testing.T) {
	tracker := NewDHTTracker(0, 0)
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:4672")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	tracker.handleBootstrapResponse(addr, kadproto.BootstrapRes{
		ID:      kadproto.NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		TCPPort: 4661,
		Version: kadproto.KademliaVersion,
		Contacts: []kadproto.Entry{
			{
				ID: kadproto.NewID(protocol.MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")),
				Endpoint: kadproto.Endpoint{
					IP:      0x7f000001,
					UDPPort: 4672,
					TCPPort: 4661,
				},
				Version: 8,
			},
		},
	})
	if got := len(tracker.nodes); got != 2 {
		t.Fatalf("expected 2 nodes after bootstrap response, got %d", got)
	}
}

func TestDHTSearchCallbackCanReenterTracker(t *testing.T) {
	tracker := NewDHTTracker(0, 0)
	defer tracker.Close()
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:4672")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	tracker.AddNode(addr)

	called := false
	hash := protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")
	if !tracker.SearchKeywords(hash, func([]kadproto.SearchEntry) {
		_ = tracker.Status()
		called = true
	}) {
		t.Fatal("expected keyword search to start")
	}
	if !called {
		t.Fatal("expected search callback")
	}
}

func TestDHTTrackerFindResponseAddsDiscoveredNodes(t *testing.T) {
	tracker := NewDHTTracker(0, 0)
	target := protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:4672")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	tracker.handleFindResponse(addr, kadproto.Res{
		Target: kadproto.NewID(target),
		Results: []kadproto.Entry{
			{
				ID: kadproto.NewID(protocol.MustHashFromString("31D6CFE0D14CE931B73C59D7E0C04BC0")),
				Endpoint: kadproto.Endpoint{
					IP:      0x7f000001,
					UDPPort: 4672,
					TCPPort: 4661,
				},
				Version: 8,
			},
		},
	})
	if got := len(tracker.nodes); got != 1 {
		t.Fatalf("expected 1 discovered node, got %d", got)
	}
}

func TestDHTTrackerHandlePublishAndSearchSources(t *testing.T) {
	tracker := NewDHTTracker(0, 0)
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:4672")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	fileHash := protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")
	req := kadproto.PublishSourcesReq{
		FileID: kadproto.NewID(fileHash),
		Source: kadproto.SearchEntry{
			ID: kadproto.NewID(protocol.MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")),
			Tags: []kadproto.Tag{
				{Type: kadproto.TagTypeUint8, ID: kadproto.TagSourceType, UInt64: 1},
				{Type: kadproto.TagTypeUint32, ID: kadproto.TagSourceIP, UInt64: 0x7f000001},
				{Type: kadproto.TagTypeUint16, ID: kadproto.TagSourcePort, UInt64: 4662},
			},
		},
	}
	tracker.handlePublishSourcesRequest(addr, req)
	results := tracker.searchEntriesLocked(fileHash)
	if len(results) != 1 {
		t.Fatalf("expected 1 indexed result, got %d", len(results))
	}
	if endpoint, ok := results[0].SourceEndpoint(); !ok || endpoint.String() != "127.0.0.1:4662" {
		t.Fatalf("unexpected indexed endpoint %v %v", endpoint, ok)
	}
}

func TestKadEndpointUsesNetworkByteOrder(t *testing.T) {
	endpoint := kadproto.Endpoint{IP: 0x73d9faf6, UDPPort: 4672}
	addr := udpAddrFromKad(endpoint)
	if got := addr.String(); got != "115.217.250.246:4672" {
		t.Fatalf("kad endpoint = %s, want 115.217.250.246:4672", got)
	}
	if got := protocolIP(addr.IP); got != endpoint.IP {
		t.Fatalf("kad IP round trip = %#x, want %#x", got, endpoint.IP)
	}
}

func TestDHTTrackerHandlePublishAndSearchKeys(t *testing.T) {
	tracker := NewDHTTracker(0, 0)
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:4672")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	keyword := protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")
	req := kadproto.PublishKeysReq{
		KeywordID: kadproto.NewID(keyword),
		Sources: []kadproto.SearchEntry{
			{
				ID: kadproto.NewID(protocol.MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")),
				Tags: []kadproto.Tag{
					{Type: kadproto.TagTypeString, ID: 0x01, String: "demo.epub"},
				},
			},
		},
	}
	tracker.handlePublishKeysRequest(addr, req)
	results := tracker.keywordEntriesLocked(keyword)
	if len(results) != 1 {
		t.Fatalf("expected 1 keyword result, got %d", len(results))
	}
	if name, ok := results[0].StringTag(0x01); !ok || name != "demo.epub" {
		t.Fatalf("unexpected keyword entry name %q %v", name, ok)
	}
}

func TestDHTTrackerHandlePublishAndSearchNotes(t *testing.T) {
	tracker := NewDHTTracker(0, 0)
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:4672")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	fileHash := protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")
	req := kadproto.PublishNotesReq{
		FileID: kadproto.NewID(fileHash),
		Notes: []kadproto.SearchEntry{
			{
				ID: kadproto.NewID(protocol.MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")),
				Tags: []kadproto.Tag{
					{Type: kadproto.TagTypeString, ID: 0x01, String: "note text"},
				},
			},
		},
	}
	tracker.handlePublishNotesRequest(addr, req)
	results := tracker.notesEntriesLocked(fileHash)
	if len(results) != 1 {
		t.Fatalf("expected 1 note result, got %d", len(results))
	}
	if note, ok := results[0].StringTag(0x01); !ok || note != "note text" {
		t.Fatalf("unexpected note %q %v", note, ok)
	}
}

func TestDHTTrackerHandleFirewalledResponse(t *testing.T) {
	tracker := NewDHTTracker(0, 0)
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:4672")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	tracker.nodes[addr.String()] = &KadRoutingNode{
		ID:   kadproto.NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		Addr: addr,
	}
	tracker.handleFirewalledResponse(addr, kadproto.FirewalledRes{IP: 0x0100007f})
	if len(tracker.externalIPs) != 1 {
		t.Fatalf("expected 1 sampled external ip, got %d", len(tracker.externalIPs))
	}
}

func TestDHTTrackerSnapshotAndApplyState(t *testing.T) {
	tracker := NewDHTTracker(0, 0)
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:4672")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	router, err := net.ResolveUDPAddr("udp", "5.6.7.8:4672")
	if err != nil {
		t.Fatalf("resolve router: %v", err)
	}
	tracker.table.AddRouterNode(router)
	tracker.SetStoragePoint(router)
	tracker.addOrUpdateNode(kadproto.NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")), addr, 4661, 8, true)
	tracker.nodes[addr.String()].Pinged = true
	tracker.table.NodeSeen(tracker.nodes[addr.String()])
	tracker.firewalled = false
	tracker.lastBootstrap = time.Now().Add(-time.Minute)
	tracker.lastRefresh = time.Now().Add(-time.Second)

	state := tracker.SnapshotState()
	if state == nil || len(state.Nodes) != 1 {
		t.Fatalf("expected one persisted DHT node, got %+v", state)
	}

	restored := NewDHTTracker(0, 0)
	if err := restored.ApplyState(state); err != nil {
		t.Fatalf("apply dht state: %v", err)
	}
	status := restored.Status()
	if !status.Bootstrapped {
		t.Fatal("expected restored tracker to be bootstrapped")
	}
	if status.RouterNodes != 1 {
		t.Fatalf("expected 1 router node, got %d", status.RouterNodes)
	}
	if status.StoragePoint != router.String() {
		t.Fatalf("expected storage point %s, got %s", router.String(), status.StoragePoint)
	}
}
