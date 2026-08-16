package goed2k

import (
	"net"
	"testing"

	kadproto "github.com/monkeyWie/goed2k/protocol/kad"
)

func TestKadFindTraversalKeepsRoutingTableTarget(t *testing.T) {
	tracker := NewDHTTracker(0, 0)
	traversal := newKadTraversal(tracker.node, kadproto.ID{}, kadTraversalFindSources, 1, func([]kadproto.SearchEntry) {})

	for i := 0; i < 50; i++ {
		traversal.addNode(&net.UDPAddr{IP: net.IPv4(127, 0, 0, byte(i+1)), Port: 4665}, kadproto.ID{}, 4662, 10)
	}

	if got, want := traversal.numTargetNodes, tracker.table.bucketSize*2; got != want {
		t.Fatalf("find traversal target = %d, want routing-table target %d", got, want)
	}
}

func TestKadShortTimeoutStartsAnotherRequest(t *testing.T) {
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
	traversal := newKadTraversal(tracker.node, kadproto.ID{}, kadTraversalFindSources, 1, func([]kadproto.SearchEntry) {})
	traversal.addNode(receiver.LocalAddr().(*net.UDPAddr), kadproto.ID{}, 4662, 10)
	traversal.addNode(receiver.LocalAddr().(*net.UDPAddr), kadproto.ID{}, 4662, 10)
	traversal.branchFactor = 1
	traversal.invokeCount = 1
	traversal.results[0].flags |= kadObserverFlagQueried

	traversal.failed(traversal.results[0], kadTraversalShortTimeout)

	if traversal.results[1].flags&kadObserverFlagQueried == 0 {
		t.Fatal("short timeout did not start the next request")
	}
	if got, want := traversal.invokeCount, 2; got != want {
		t.Fatalf("in-flight request count = %d, want %d", got, want)
	}
}

func TestKadFindResponseStartsDirectSourceSearch(t *testing.T) {
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
	find := newKadTraversal(tracker.node, kadproto.ID{}, kadTraversalFindSources, 1, func([]kadproto.SearchEntry) {})
	responder := find.newObserver(receiver.LocalAddr().(*net.UDPAddr), kadproto.ID{}, 4662, 10)

	find.searchSourcesAt(responder)

	if find.direct == nil {
		t.Fatal("find response did not create a direct source traversal")
	}
	if got, want := find.direct.kind, kadTraversalSearchSources; got != want {
		t.Fatalf("direct traversal kind = %q, want %q", got, want)
	}
	if len(find.direct.results) != 1 || find.direct.results[0].flags&kadObserverFlagQueried == 0 {
		t.Fatal("direct source request was not sent to the responder")
	}
}

func TestKadFindResponseJoinsRunningDirectSourceSearch(t *testing.T) {
	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	secondReceiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer secondReceiver.Close()
	sender, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()

	tracker := NewDHTTracker(0, 0)
	tracker.conn = sender
	firstFind := newKadTraversal(tracker.node, kadproto.ID{}, kadTraversalFindSources, 1, func([]kadproto.SearchEntry) {})
	secondFind := newKadTraversal(tracker.node, kadproto.ID{}, kadTraversalFindSources, 1, func([]kadproto.SearchEntry) {})
	firstFind.searchSourcesAt(firstFind.newObserver(receiver.LocalAddr().(*net.UDPAddr), kadproto.ID{}, 4662, 10))
	secondFind.searchSourcesAt(secondFind.newObserver(secondReceiver.LocalAddr().(*net.UDPAddr), kadproto.ID{}, 4662, 10))

	if firstFind.direct == nil || secondFind.direct != firstFind.direct {
		t.Fatal("overlapping find traversal did not join the running direct source search")
	}
	if got, want := len(firstFind.direct.results), 2; got != want {
		t.Fatalf("direct source responders = %d, want %d", got, want)
	}
}

func TestKadFindRequestType(t *testing.T) {
	tests := []struct {
		name string
		kind kadTraversalKind
		want byte
	}{
		{name: "sources", kind: kadTraversalFindSources, want: kadproto.FindValue},
		{name: "keywords", kind: kadTraversalFindKeywords, want: kadproto.FindValue},
		{name: "refresh", kind: kadTraversalRefresh, want: kadproto.FindNode},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := kadFindRequestType(test.kind); got != test.want {
				t.Fatalf("kadFindRequestType(%q) = %#x, want %#x", test.kind, got, test.want)
			}
		})
	}
}
