package goed2k

import (
	"net"
	"sort"

	"github.com/monkeyWie/goed2k/protocol"
	kadproto "github.com/monkeyWie/goed2k/protocol/kad"
)

const (
	kadObserverFlagQueried      = 1 << 0
	kadObserverFlagInitial      = 1 << 1
	kadObserverFlagNoID         = 1 << 2
	kadObserverFlagShortTimeout = 1 << 3
	kadObserverFlagFailed       = 1 << 4
	kadObserverFlagAlive        = 1 << 5
	kadObserverFlagDone         = 1 << 6

	kadTraversalPreventRequest = 1
	kadTraversalShortTimeout   = 2
	kadTraversalMaxResults     = 100
)

type kadTraversalKind string

const (
	kadTraversalBootstrap     kadTraversalKind = "bootstrap"
	kadTraversalFindSources   kadTraversalKind = "find-sources"
	kadTraversalSearchSources kadTraversalKind = "search-sources"
	kadTraversalFindKeywords  kadTraversalKind = "find-keywords"
	kadTraversalSearchKeyword kadTraversalKind = "search-keywords"
	kadTraversalRefresh       kadTraversalKind = "refresh"
	kadTraversalFirewalled    kadTraversalKind = "firewalled"
)

type kadObserver struct {
	traversal          *kadTraversal
	endpoint           *net.UDPAddr
	id                 kadproto.ID
	portTCP            uint16
	version            byte
	flags              int
	processedResponses int
	entries            []kadproto.SearchEntry
	externalIP         uint32
}

func (o *kadObserver) expectMultipleResponses() bool {
	if o == nil || o.traversal == nil {
		return false
	}
	return o.traversal.kind == kadTraversalSearchSources || o.traversal.kind == kadTraversalSearchKeyword
}

func (o *kadObserver) shortTimeout() {
	if o == nil || o.traversal == nil || o.flags&kadObserverFlagShortTimeout != 0 {
		return
	}
	o.traversal.failed(o, kadTraversalShortTimeout)
}

func (o *kadObserver) timeout() {
	if o == nil || o.traversal == nil || o.flags&kadObserverFlagDone != 0 {
		return
	}
	if o.expectMultipleResponses() && o.processedResponses > 0 {
		o.done()
		return
	}
	o.flags |= kadObserverFlagDone
	o.traversal.failed(o, 0)
}

func (o *kadObserver) abort() {
	if o == nil || o.traversal == nil || o.flags&kadObserverFlagDone != 0 {
		return
	}
	o.flags |= kadObserverFlagDone
	o.traversal.failed(o, kadTraversalPreventRequest)
}

func (o *kadObserver) done() {
	if o == nil || o.traversal == nil || o.flags&kadObserverFlagDone != 0 {
		return
	}
	o.flags |= kadObserverFlagDone
	o.traversal.finished(o)
}

type kadTraversal struct {
	node           *kadNodeImpl
	target         kadproto.ID
	kind           kadTraversalKind
	results        []*kadObserver
	invokeCount    int
	branchFactor   int
	responses      int
	timeouts       int
	numTargetNodes int
	size           int64
	listener       func([]kadproto.SearchEntry)
	accum          []kadproto.SearchEntry
	externalIPs    []uint32
}

func newKadTraversal(node *kadNodeImpl, target kadproto.ID, kind kadTraversalKind, size int64, listener func([]kadproto.SearchEntry)) *kadTraversal {
	traversal := &kadTraversal{
		node:        node,
		target:      target,
		kind:        kind,
		size:        size,
		listener:    listener,
		results:     make([]*kadObserver, 0, 16),
		accum:       make([]kadproto.SearchEntry, 0, 16),
		externalIPs: make([]uint32, 0, 2),
	}
	if node != nil && node.tracker != nil && node.tracker.table != nil {
		traversal.numTargetNodes = node.tracker.table.bucketSize * 2
	}
	if traversal.numTargetNodes <= 0 {
		traversal.numTargetNodes = 20
	}
	return traversal
}

func (t *kadTraversal) key() string {
	if t == nil {
		return ""
	}
	return string(t.kind) + ":" + t.target.Hash.String()
}

func (t *kadTraversal) start() bool {
	if t == nil || t.node == nil || t.node.tracker == nil {
		return false
	}
	if !t.node.addTraversal(t) {
		return false
	}
	t.branchFactor = t.node.searchBranching()
	if t.kind == kadTraversalRefresh {
		t.numTargetNodes = 1
	}
	if t.kind == kadTraversalSearchSources || t.kind == kadTraversalSearchKeyword {
		t.numTargetNodes = len(t.results)
	}
	if t.node.tracker.table != nil {
		t.node.tracker.table.TouchBucket(t.target)
	}
	if len(t.results) == 0 {
		t.addRouterEntries()
	}
	t.addRequests()
	if t.invokeCount == 0 {
		t.done()
	}
	return true
}

func (t *kadTraversal) addRouterEntries() {
	if t == nil || t.node == nil || t.node.tracker == nil || t.node.tracker.table == nil {
		return
	}
	for _, addr := range t.node.tracker.table.RouterNodes() {
		t.addEntry(kadproto.ID{}, addr, kadObserverFlagInitial, 0, 0)
	}
}

func (t *kadTraversal) newObserver(endpoint *net.UDPAddr, id kadproto.ID, portTCP uint16, version byte) *kadObserver {
	return &kadObserver{
		traversal: t,
		endpoint:  normalizeUDPAddr(endpoint),
		id:        id,
		portTCP:   portTCP,
		version:   version,
	}
}

func (t *kadTraversal) addNode(endpoint *net.UDPAddr, id kadproto.ID, portTCP uint16, version byte) {
	o := t.newObserver(endpoint, id, portTCP, version)
	t.results = append(t.results, o)
	t.numTargetNodes = len(t.results)
}

func (t *kadTraversal) addEntry(id kadproto.ID, endpoint *net.UDPAddr, flags int, portTCP uint16, version byte) {
	if t == nil || endpoint == nil {
		return
	}
	o := t.newObserver(endpoint, id, portTCP, version)
	if o.id.Hash.Equal(protocol.Invalid) {
		randomID, err := protocol.RandomHash(false)
		if err == nil {
			o.id = kadproto.NewID(randomID)
			o.flags |= kadObserverFlagNoID
		}
	}
	o.flags |= flags
	insertPos := sort.Search(len(t.results), func(i int) bool {
		return kadproto.DistanceCompare(t.results[i].id, o.id, t.target) >= 0
	})
	if insertPos < len(t.results) {
		existing := t.results[insertPos]
		if existing != nil && existing.id.Hash.Equal(o.id.Hash) {
			return
		}
	}
	t.results = append(t.results, nil)
	copy(t.results[insertPos+1:], t.results[insertPos:])
	t.results[insertPos] = o
	if len(t.results) > kadTraversalMaxResults {
		t.results = t.results[:kadTraversalMaxResults]
	}
}

func (t *kadTraversal) addRequests() {
	resultsTarget := t.numTargetNodes
	for i := 0; i < len(t.results) && resultsTarget > 0 && t.invokeCount < t.branchFactor; i++ {
		o := t.results[i]
		if o == nil {
			continue
		}
		if o.flags&kadObserverFlagAlive != 0 {
			resultsTarget--
		}
		if o.flags&kadObserverFlagQueried != 0 {
			continue
		}
		o.flags |= kadObserverFlagQueried
		if t.invoke(o) {
			t.invokeCount++
		} else {
			o.flags |= kadObserverFlagFailed
		}
	}
}

func (t *kadTraversal) invoke(o *kadObserver) bool {
	if t == nil || t.node == nil || o == nil || o.endpoint == nil {
		return false
	}
	switch t.kind {
	case kadTraversalBootstrap:
		return t.node.invoke(kadproto.BootstrapReq{}, o.endpoint, o, kadproto.BootstrapResOp, nil, false)
	case kadTraversalFindSources, kadTraversalRefresh:
		searchType := kadproto.FindNode
		return t.node.invoke(kadproto.Req{
			SearchType: searchType,
			Target:     t.target,
			Receiver:   o.id,
		}, o.endpoint, o, kadproto.ResOp, &t.target.Hash, false)
	case kadTraversalFindKeywords:
		searchType := kadproto.FindValue
		return t.node.invoke(kadproto.Req{
			SearchType: searchType,
			Target:     t.target,
			Receiver:   o.id,
		}, o.endpoint, o, kadproto.ResOp, &t.target.Hash, false)
	case kadTraversalSearchSources:
		return t.node.invoke(kadproto.SearchSourcesReq{
			Target:   t.target,
			StartPos: 0,
			Size:     uint64(t.size),
		}, o.endpoint, o, kadproto.SearchResOp, &t.target.Hash, true)
	case kadTraversalSearchKeyword:
		return t.node.invoke(kadproto.SearchKeysReq{
			Target:   t.target,
			StartPos: 0,
		}, o.endpoint, o, kadproto.SearchResOp, &t.target.Hash, true)
		case kadTraversalFirewalled:
			return t.node.invoke(kadproto.FirewalledReq{
				TCPPort: uint16(t.node.listenPort()),
				ID:      t.target,
				Options: 0,
			}, o.endpoint, o, kadproto.FirewalledResOp, nil, false)
	default:
		return false
	}
}

func (t *kadTraversal) failed(o *kadObserver, flags int) {
	if t == nil || o == nil {
		return
	}
	if flags&kadTraversalShortTimeout != 0 {
		t.branchFactor++
		o.flags |= kadObserverFlagShortTimeout
		return
	}
	o.flags |= kadObserverFlagFailed
	if o.flags&kadObserverFlagShortTimeout != 0 && t.branchFactor > 1 {
		t.branchFactor--
	}
	t.writeFailedObserver(o)
	t.timeouts++
	if t.invokeCount > 0 {
		t.invokeCount--
	}
	if flags&kadTraversalPreventRequest != 0 && t.branchFactor > 1 {
		t.branchFactor--
	}
	t.addRequests()
	if t.invokeCount == 0 {
		t.done()
	}
}

func (t *kadTraversal) writeFailedObserver(o *kadObserver) {
	if t == nil || t.node == nil || t.node.tracker == nil || o == nil {
		return
	}
	if t.kind == kadTraversalFindSources || t.kind == kadTraversalSearchSources {
		return
	}
	if o.flags&kadObserverFlagNoID != 0 {
		return
	}
	node := t.node.tracker.nodes[o.endpoint.String()]
	if node != nil {
		t.node.tracker.table.NodeFailed(node)
	}
}

func (t *kadTraversal) finished(o *kadObserver) {
	if t == nil || o == nil {
		return
	}
	if o.flags&kadObserverFlagShortTimeout != 0 && t.branchFactor > 1 {
		t.branchFactor--
	}
	o.flags |= kadObserverFlagAlive
	t.responses++
	if t.invokeCount > 0 {
		t.invokeCount--
	}
	switch t.kind {
	case kadTraversalSearchSources, kadTraversalSearchKeyword:
		if len(o.entries) > 0 {
			t.accum = append(t.accum, o.entries...)
		}
	case kadTraversalFirewalled:
		if o.externalIP != 0 && len(t.externalIPs) < 2 {
			t.externalIPs = append(t.externalIPs, o.externalIP)
		}
	}
	t.addRequests()
	if t.invokeCount == 0 {
		t.done()
	}
}

func (t *kadTraversal) traverse(endpoint *net.UDPAddr, id kadproto.ID, portTCP uint16, version byte) {
	if t == nil || t.node == nil || t.node.tracker == nil {
		return
	}
	node := t.node.tracker.addOrUpdateNodeLocked(id, endpoint, portTCP, version, false)
	if node != nil && node.KnownID() {
		t.node.tracker.table.HeardAbout(node)
	}
	t.addEntry(id, endpoint, 0, portTCP, version)
}

func (t *kadTraversal) done() {
	if t == nil || t.node == nil {
		return
	}
	t.node.removeTraversal(t)
	switch t.kind {
	case kadTraversalBootstrap:
		for _, o := range t.results {
			if o == nil || o.flags&kadObserverFlagQueried != 0 {
				continue
			}
			t.node.addNode(o.endpoint, o.id, o.portTCP, o.version)
		}
	case kadTraversalFindSources:
		direct := newKadTraversal(t.node, t.target, kadTraversalSearchSources, t.size, t.listener)
		if sp := t.node.storagePoint(); sp != nil {
			direct.addNode(sp, t.target, 0, 0)
		}
		for _, o := range t.results {
			if o == nil || o.flags&kadObserverFlagFailed != 0 {
				continue
			}
			direct.addNode(o.endpoint, o.id, o.portTCP, o.version)
		}
		_ = direct.start()
	case kadTraversalFindKeywords:
		direct := newKadTraversal(t.node, t.target, kadTraversalSearchKeyword, 0, t.listener)
		if sp := t.node.storagePoint(); sp != nil {
			direct.addNode(sp, t.target, 0, 0)
		}
		for _, o := range t.results {
			if o == nil || o.flags&kadObserverFlagFailed != 0 {
				continue
			}
			direct.addNode(o.endpoint, o.id, o.portTCP, o.version)
		}
		_ = direct.start()
	case kadTraversalSearchSources:
		for _, entry := range t.accum {
			t.node.processPublishSourcesReq(nil, kadproto.PublishSourcesReq{
				FileID: t.target,
				Source: entry,
			})
		}
		if t.listener != nil {
			t.listener(t.accum)
		}
	case kadTraversalSearchKeyword:
		for _, entry := range t.accum {
			t.node.processPublishKeysReq(nil, kadproto.PublishKeysReq{
				KeywordID: t.target,
				Sources:   []kadproto.SearchEntry{entry},
			})
		}
		if t.listener != nil {
			t.listener(t.accum)
		}
	case kadTraversalFirewalled:
		t.node.processAddresses(t.externalIPs)
	}
}
