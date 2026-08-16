package goed2k

import (
	"net"
	"time"

	"github.com/monkeyWie/goed2k/protocol"
	kadproto "github.com/monkeyWie/goed2k/protocol/kad"
)

type kadNodeImpl struct {
	tracker                  *DHTTracker
	running                  map[string]*kadTraversal
	initialBootstrapRequired bool
}

func newKadNodeImpl(tracker *DHTTracker) *kadNodeImpl {
	return &kadNodeImpl{
		tracker:                  tracker,
		running:                  make(map[string]*kadTraversal),
		initialBootstrapRequired: true,
	}
}

func (n *kadNodeImpl) searchBranching() int {
	return 5
}

func (n *kadNodeImpl) listenPort() int {
	if n == nil || n.tracker == nil {
		return 0
	}
	return n.tracker.ListenPort()
}

func (n *kadNodeImpl) storagePoint() *net.UDPAddr {
	if n == nil || n.tracker == nil {
		return nil
	}
	return n.tracker.storagePoint
}

func (n *kadNodeImpl) addTraversal(t *kadTraversal) bool {
	if n == nil || t == nil {
		return false
	}
	key := t.key()
	if _, exists := n.running[key]; exists {
		return false
	}
	n.running[key] = t
	return true
}

func (n *kadNodeImpl) removeTraversal(t *kadTraversal) {
	if n == nil || t == nil {
		return
	}
	delete(n.running, t.key())
}

func (n *kadNodeImpl) addRouterNode(addr *net.UDPAddr) {
	if n == nil || n.tracker == nil || n.tracker.table == nil {
		return
	}
	n.tracker.table.AddRouterNode(addr)
}

func (n *kadNodeImpl) addNode(addr *net.UDPAddr, id kadproto.ID, portTCP uint16, version byte) {
	if n == nil || n.tracker == nil || addr == nil {
		return
	}
	node := n.tracker.addOrUpdateNodeLocked(id, addr, portTCP, version, false)
	if node == nil {
		return
	}
	n.tracker.maybeSendHelloLocked(node)
}

func (n *kadNodeImpl) bootstrap(addrs []*net.UDPAddr) bool {
	if n == nil {
		return false
	}
	t := newKadTraversal(n, n.tracker.selfID, kadTraversalBootstrap, 0, nil)
	for _, addr := range addrs {
		t.addEntry(kadproto.ID{}, addr, kadObserverFlagInitial, 0, 0)
	}
	for _, addr := range n.tracker.table.RouterNodes() {
		t.addEntry(kadproto.ID{}, addr, kadObserverFlagInitial, 0, 0)
	}
	return t.start()
}

func (n *kadNodeImpl) searchSources(hash protocol.Hash, size int64, cb func([]kadproto.SearchEntry)) bool {
	if n == nil || n.tracker == nil || size <= 0 || cb == nil {
		return false
	}
	t := newKadTraversal(n, kadproto.NewID(hash), kadTraversalFindSources, size, cb)
	for _, node := range n.tracker.closestNodesLocked(kadproto.NewID(hash), 50, true) {
		t.addNode(node.Addr, node.ID, node.TCPPort, node.Version)
	}
	return t.start()
}

func (n *kadNodeImpl) searchKeywords(hash protocol.Hash, cb func([]kadproto.SearchEntry)) bool {
	if n == nil || n.tracker == nil || cb == nil {
		return false
	}
	t := newKadTraversal(n, kadproto.NewID(hash), kadTraversalFindKeywords, 0, cb)
	for _, node := range n.tracker.closestNodesLocked(kadproto.NewID(hash), 50, true) {
		t.addNode(node.Addr, node.ID, node.TCPPort, node.Version)
	}
	return t.start()
}

func (n *kadNodeImpl) refresh(target kadproto.ID) bool {
	t := newKadTraversal(n, target, kadTraversalRefresh, 0, nil)
	for _, node := range n.tracker.closestNodesLocked(target, 50, true) {
		t.addNode(node.Addr, node.ID, node.TCPPort, node.Version)
	}
	return t.start()
}

func (n *kadNodeImpl) firewalled() bool {
	t := newKadTraversal(n, n.tracker.selfID, kadTraversalFirewalled, 0, nil)
	for _, node := range n.tracker.closestNodesLocked(n.tracker.selfID, 50, true) {
		t.addNode(node.Addr, node.ID, node.TCPPort, node.Version)
	}
	return t.start()
}

func (n *kadNodeImpl) invoke(packet any, addr *net.UDPAddr, observer *kadObserver, opcode byte, target *protocol.Hash, multi bool) bool {
	if n == nil || n.tracker == nil || addr == nil {
		return false
	}
	if _, err := n.tracker.writePacket(addr, packet); err != nil {
		return false
	}
	n.tracker.rpc.Invoke(&kadRPCTransaction{
		endpointKey: addr.String(),
		opcode:      opcode,
		target:      target,
		multi:       multi,
		observer:    observer,
	})
	return true
}

func (n *kadNodeImpl) tick() {
	if n == nil || n.tracker == nil {
		return
	}
	now := time.Now()
	shortTimed, expired := n.tracker.rpc.Tick(now)
	for _, tx := range shortTimed {
		if tx == nil || tx.observer == nil {
			continue
		}
		tx.observer.shortTimeout()
	}
	for _, tx := range expired {
		if tx == nil {
			continue
		}
		if tx.observer != nil {
			tx.observer.timeout()
			continue
		}
		node := n.tracker.nodes[tx.endpointKey]
		if node != nil {
			node.FailCount++
			n.tracker.table.NodeFailed(node)
		}
	}

	live, replacements := n.tracker.table.Size()
	if len(n.running) == 0 && n.initialBootstrapRequired && live > 0 && replacements < 10 {
		n.initialBootstrapRequired = false
		seeds := n.tracker.knownNodesLocked(false)
		addrs := make([]*net.UDPAddr, 0, len(seeds))
		for _, node := range seeds {
			if node != nil && node.Addr != nil {
				addrs = append(addrs, node.Addr)
			}
		}
		n.bootstrap(addrs)
	}
	if target := n.tracker.table.NeedRefresh(now); target != nil {
		n.tracker.lastRefresh = now
		n.refresh(*target)
	}
	if live >= 5 && (n.tracker.lastFirewalledCheck.IsZero() || now.Sub(n.tracker.lastFirewalledCheck) >= time.Hour) {
		n.tracker.lastFirewalledCheck = now
		n.firewalled()
	}
	if live == 0 && len(n.tracker.nodes) > 0 && n.tracker.table.NeedBootstrap(now) {
		n.tracker.lastBootstrap = now
		seeds := n.tracker.knownNodesLocked(false)
		addrs := make([]*net.UDPAddr, 0, len(seeds))
		for _, node := range seeds {
			if node != nil && node.Addr != nil {
				addrs = append(addrs, node.Addr)
			}
		}
		n.bootstrap(addrs)
	}
}

func (n *kadNodeImpl) processPing(addr *net.UDPAddr) {
	if n == nil || n.tracker == nil {
		return
	}
	_, _ = n.tracker.writePacket(addr, kadproto.Pong{UDPPort: uint16(n.tracker.ListenPort())})
}

func (n *kadNodeImpl) processHelloReq(addr *net.UDPAddr, hello kadproto.Hello) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.handleHello(addr, hello, true)
}

func (n *kadNodeImpl) processHelloRes(addr *net.UDPAddr, hello kadproto.Hello) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.rpc.Incoming(addr, kadproto.HelloResOp, nil)
	n.tracker.handleHello(addr, hello, false)
}

func (n *kadNodeImpl) processSearchNotesReq(addr *net.UDPAddr, req kadproto.SearchNotesReq) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.handleSearchNotesRequest(addr, req)
}

func (n *kadNodeImpl) processFindReq(addr *net.UDPAddr, req kadproto.Req) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.handleFindRequest(addr, req)
}

func (n *kadNodeImpl) processFindRes(addr *net.UDPAddr, res kadproto.Res) {
	if n == nil || n.tracker == nil {
		return
	}
	target := res.Target.Hash
	tx := n.tracker.rpc.Incoming(addr, kadproto.ResOp, &target)
	n.tracker.handleFindResponse(addr, res)
	if tx == nil || tx.observer == nil {
		return
	}
	for _, entry := range res.Results {
		tx.observer.traversal.traverse(udpAddrFromKad(entry.Endpoint), entry.ID, entry.Endpoint.TCPPort, entry.Version)
	}
	tx.observer.done()
}

func (n *kadNodeImpl) processBootstrapReq(addr *net.UDPAddr) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.handleBootstrapRequest(addr)
}

func (n *kadNodeImpl) processBootstrapRes(addr *net.UDPAddr, res kadproto.BootstrapRes) {
	if n == nil || n.tracker == nil {
		return
	}
	tx := n.tracker.rpc.Incoming(addr, kadproto.BootstrapResOp, nil)
	n.tracker.handleBootstrapResponse(addr, res)
	if tx == nil || tx.observer == nil {
		return
	}
	for _, contact := range res.Contacts {
		tx.observer.traversal.traverse(udpAddrFromKad(contact.Endpoint), contact.ID, contact.Endpoint.TCPPort, contact.Version)
	}
	tx.observer.done()
}

func (n *kadNodeImpl) processPublishKeysReq(addr *net.UDPAddr, req kadproto.PublishKeysReq) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.handlePublishKeysRequest(addr, req)
}

func (n *kadNodeImpl) processPublishSourcesReq(addr *net.UDPAddr, req kadproto.PublishSourcesReq) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.handlePublishSourcesRequest(addr, req)
}

func (n *kadNodeImpl) processPublishNotesReq(addr *net.UDPAddr, req kadproto.PublishNotesReq) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.handlePublishNotesRequest(addr, req)
}

func (n *kadNodeImpl) processPublishRes(addr *net.UDPAddr, res kadproto.PublishRes) {
	if n == nil || n.tracker == nil {
		return
	}
	target := res.FileID.Hash
	n.tracker.rpc.Incoming(addr, kadproto.PublishResOp, &target)
}

func (n *kadNodeImpl) processPublishNotesRes(addr *net.UDPAddr, res kadproto.PublishNotesRes) {
	if n == nil || n.tracker == nil {
		return
	}
	target := res.FileID.Hash
	n.tracker.rpc.Incoming(addr, kadproto.PublishNotesResOp, &target)
}

func (n *kadNodeImpl) processFirewalledReq(addr *net.UDPAddr, req kadproto.FirewalledReq) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.handleFirewalledRequest(addr, req)
}

func (n *kadNodeImpl) processLegacyFirewalledReq(addr *net.UDPAddr, req kadproto.LegacyFirewalledReq) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.handleFirewalledRequest(addr, kadproto.FirewalledReq{
		TCPPort: req.TCPPort,
		ID:      n.tracker.selfID,
	})
}

func (n *kadNodeImpl) processFirewalledRes(addr *net.UDPAddr, res kadproto.FirewalledRes) {
	if n == nil || n.tracker == nil {
		return
	}
	tx := n.tracker.rpc.Incoming(addr, kadproto.FirewalledResOp, nil)
	n.tracker.handleFirewalledResponse(addr, res)
	if tx == nil || tx.observer == nil {
		return
	}
	tx.observer.externalIP = res.IP
	tx.observer.done()
}

func (n *kadNodeImpl) processSearchKeysReq(addr *net.UDPAddr, req kadproto.SearchKeysReq) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.handleSearchKeysRequest(addr, req)
}

func (n *kadNodeImpl) processSearchSourcesReq(addr *net.UDPAddr, req kadproto.SearchSourcesReq) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.handleSearchSourcesRequest(addr, req)
}

func (n *kadNodeImpl) processSearchRes(addr *net.UDPAddr, res kadproto.SearchRes) {
	if n == nil || n.tracker == nil {
		return
	}
	target := res.Target.Hash
	tx := n.tracker.rpc.Incoming(addr, kadproto.SearchResOp, &target)
	if tx == nil || tx.observer == nil {
		return
	}
	tx.observer.entries = append(tx.observer.entries, res.Results...)
	tx.observer.processedResponses++
}

func (n *kadNodeImpl) processPong(addr *net.UDPAddr, pong kadproto.Pong) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.rpc.Incoming(addr, kadproto.PongOp, nil)
	n.tracker.mu.Lock()
	if node := n.tracker.nodes[addr.String()]; node != nil {
		n.tracker.confirmNodeLocked(node)
	}
	n.tracker.mu.Unlock()
	_ = pong
}

func (n *kadNodeImpl) processFirewalledUDP(addr *net.UDPAddr, packet kadproto.FirewalledUDP) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.mu.Lock()
	defer n.tracker.mu.Unlock()
	if node := n.tracker.nodes[addr.String()]; node != nil {
		n.tracker.confirmNodeLocked(node)
	}
	_ = packet
}

func (n *kadNodeImpl) processAddresses(addresses []uint32) {
	if n == nil || n.tracker == nil {
		return
	}
	n.tracker.mu.Lock()
	defer n.tracker.mu.Unlock()
	n.tracker.externalIPs = n.tracker.externalIPs[:0]
	for _, ip := range addresses {
		if ip != 0 {
			n.tracker.externalIPs = append(n.tracker.externalIPs, ip)
		}
	}
	if len(n.tracker.externalIPs) < 2 {
		return
	}
	localIP := uint32(0)
	if n.tracker.conn != nil {
		if udpAddr, ok := n.tracker.conn.LocalAddr().(*net.UDPAddr); ok {
			localIP = protocolIP(udpAddr.IP)
		}
	}
	matchCount := 0
	for _, ip := range n.tracker.externalIPs[:2] {
		if ip != 0 && ip == localIP {
			matchCount++
		}
	}
	n.tracker.firewalled = matchCount != 2
}
