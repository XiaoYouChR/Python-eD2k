package goed2k

import (
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/monkeyWie/goed2k/internal/logx"
	"github.com/monkeyWie/goed2k/protocol"
	kadproto "github.com/monkeyWie/goed2k/protocol/kad"
)

type DHTTracker struct {
	mu                  sync.Mutex
	conn                *net.UDPConn
	listenPort          int
	searchTimeout       time.Duration
	combiner            kadproto.PacketCombiner
	selfID              kadproto.ID
	node                *kadNodeImpl
	nodes               map[string]*KadRoutingNode
	table               *kadRoutingTable
	rpc                 *kadRPCManager
	sourceIndex         map[string]map[string]kadproto.SearchEntry
	keywordIndex        map[string]map[string]kadproto.SearchEntry
	notesIndex          map[string]map[string]kadproto.SearchEntry
	stopCh              chan struct{}
	startOnce           sync.Once
	closeOnce           sync.Once
	lastBootstrap       time.Time
	lastRefresh         time.Time
	lastFirewalledCheck time.Time
	firewalled          bool
	externalIPs         []uint32
	storagePoint        *net.UDPAddr
}

func NewDHTTracker(listenPort int, timeout time.Duration) *DHTTracker {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	selfID := kadproto.NewID(protocol.Invalid)
	if randomID, err := protocol.RandomHash(false); err == nil {
		selfID = kadproto.NewID(randomID)
	}
	tracker := &DHTTracker{
		listenPort:    listenPort,
		searchTimeout: timeout,
		selfID:        selfID,
		nodes:         make(map[string]*KadRoutingNode),
		table:         newKadRoutingTable(selfID, 10),
		rpc:           newKadRPCManager(),
		sourceIndex:   make(map[string]map[string]kadproto.SearchEntry),
		keywordIndex:  make(map[string]map[string]kadproto.SearchEntry),
		notesIndex:    make(map[string]map[string]kadproto.SearchEntry),
		stopCh:        make(chan struct{}),
		firewalled:    true,
		externalIPs:   make([]uint32, 0, 2),
	}
	tracker.node = newKadNodeImpl(tracker)
	return tracker
}

func (t *DHTTracker) Start() error {
	var err error
	t.startOnce.Do(func() {
		port := t.ListenPort()
		conn, listenErr := net.ListenUDP("udp", &net.UDPAddr{Port: port})
		if listenErr != nil {
			err = listenErr
			return
		}
		t.conn = conn
		if udpAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			t.setListenPort(udpAddr.Port)
		}
		go t.readLoop()
	})
	return err
}

func (t *DHTTracker) Close() {
	t.closeOnce.Do(func() {
		close(t.stopCh)
		if t.conn != nil {
			_ = t.conn.Close()
		}
	})
}

func (t *DHTTracker) AddNode(addr *net.UDPAddr) {
	if addr == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	key := addr.String()
	node := t.nodes[key]
	if node == nil {
		node = &KadRoutingNode{}
		t.nodes[key] = node
	}
	node.Addr = addr
	node.Seed = true
	node.LastSeen = time.Now()
}

func (t *DHTTracker) AddNodes(addrs ...*net.UDPAddr) {
	for _, addr := range addrs {
		t.AddNode(addr)
	}
}

func (t *DHTTracker) LoadNodesDat(path string) error {
	nodes, err := kadproto.LoadNodesDat(path)
	if err != nil {
		return err
	}
	return t.ApplyNodesDat(nodes)
}

func (t *DHTTracker) ApplyNodesDat(nodes *kadproto.NodesDat) error {
	if nodes == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	haveVerified := false
	loaded := make([]*KadRoutingNode, 0, len(nodes.Contacts))
	for _, entry := range nodes.Contacts {
		addr := &net.UDPAddr{
			IP:   net.IPv4(byte(entry.Endpoint.IP), byte(entry.Endpoint.IP>>8), byte(entry.Endpoint.IP>>16), byte(entry.Endpoint.IP>>24)),
			Port: int(entry.Endpoint.UDPPort),
		}
		node := t.addOrUpdateNodeLocked(entry.ID, addr, entry.Endpoint.TCPPort, entry.Version, true)
		if node == nil {
			continue
		}
		loaded = append(loaded, node)
		if nodes.BootstrapEdition == 1 {
			t.table.AddRouterNode(addr)
			continue
		}
		if entry.Verified {
			haveVerified = true
			t.table.NodeSeen(node)
		} else {
			t.table.HeardAbout(node)
		}
	}
	if nodes.BootstrapEdition == 0 && len(loaded) > 0 && !haveVerified {
		for _, node := range loaded {
			t.table.NodeSeen(node)
		}
	}
	return nil
}

func (t *DHTTracker) SearchSources(hash protocol.Hash, size int64, cb func([]kadproto.SearchEntry)) bool {
	if cb == nil || size <= 0 {
		return false
	}
	if err := t.Start(); err != nil {
		logx.Debug("dht source search start failed", "hash", hash.String(), "err", err)
		return false
	}
	t.mu.Lock()
	if len(t.nodes) == 0 && len(t.table.RouterNodes()) == 0 {
		t.mu.Unlock()
		logx.Debug("dht source search skipped: no bootstrap nodes", "hash", hash.String())
		return false
	}
	t.mu.Unlock()
	logx.Debug("dht source search started", "hash", hash.String(), "size", size)
	return t.node.searchSources(hash, size, cb)
}

func (t *DHTTracker) readLoop() {
	buffer := make([]byte, 8192)
	for {
		if t.conn == nil {
			return
		}
		_ = t.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, addr, err := t.conn.ReadFromUDP(buffer)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				t.node.tick()
				select {
				case <-t.stopCh:
					return
				default:
				}
				continue
			}
			if errorsIsClosed(err) {
				return
			}
			continue
		}
		addr = normalizeUDPAddr(addr)
		opcode, message, err := t.combiner.Unpack(buffer[:n])
		if err != nil {
			continue
		}
		switch opcode {
		case kadproto.SearchResOp:
			t.node.processSearchRes(addr, *(message.(*kadproto.SearchRes)))
		case kadproto.SearchSrcReqOp:
			t.node.processSearchSourcesReq(addr, *(message.(*kadproto.SearchSourcesReq)))
		case kadproto.SearchKeysReqOp:
			t.node.processSearchKeysReq(addr, *(message.(*kadproto.SearchKeysReq)))
		case kadproto.BootstrapResOp:
			t.node.processBootstrapRes(addr, *(message.(*kadproto.BootstrapRes)))
		case kadproto.ResOp:
			t.node.processFindRes(addr, *(message.(*kadproto.Res)))
		case kadproto.HelloReqOp:
			t.node.processHelloReq(addr, *(message.(*kadproto.Hello)))
		case kadproto.HelloResOp:
			t.node.processHelloRes(addr, *(message.(*kadproto.Hello)))
		case kadproto.BootstrapReqOp:
			t.node.processBootstrapReq(addr)
		case kadproto.ReqOp:
			t.node.processFindReq(addr, *(message.(*kadproto.Req)))
		case kadproto.PublishSourceReqOp:
			t.node.processPublishSourcesReq(addr, *(message.(*kadproto.PublishSourcesReq)))
		case kadproto.PublishKeysReqOp:
			t.node.processPublishKeysReq(addr, *(message.(*kadproto.PublishKeysReq)))
		case kadproto.PublishNotesReqOp:
			t.node.processPublishNotesReq(addr, *(message.(*kadproto.PublishNotesReq)))
		case kadproto.PublishResOp:
			t.node.processPublishRes(addr, *(message.(*kadproto.PublishRes)))
		case kadproto.PublishNotesResOp:
			t.node.processPublishNotesRes(addr, *(message.(*kadproto.PublishNotesRes)))
		case kadproto.PingOp:
			t.node.processPing(addr)
		case kadproto.PongOp:
			t.node.processPong(addr, *(message.(*kadproto.Pong)))
		case kadproto.FirewalledReqOp:
			t.node.processFirewalledReq(addr, *(message.(*kadproto.FirewalledReq)))
		case kadproto.LegacyFirewalledReqOp:
			legacy := *(message.(*kadproto.LegacyFirewalledReq))
			t.node.processLegacyFirewalledReq(addr, legacy)
		case kadproto.FirewalledResOp:
			t.node.processFirewalledRes(addr, *(message.(*kadproto.FirewalledRes)))
		case kadproto.FirewalledUdpOp:
			t.node.processFirewalledUDP(addr, *(message.(*kadproto.FirewalledUDP)))
		case kadproto.SearchNotesReqOp:
			t.node.processSearchNotesReq(addr, *(message.(*kadproto.SearchNotesReq)))
		case kadproto.SearchNotesResOp, kadproto.HelloResAckOp, kadproto.PublishResAckOp, kadproto.CallbackReqOp, kadproto.FindBuddyReqOp, kadproto.FindBuddyResOp:
			continue
		}
	}
}

func (t *DHTTracker) handleBootstrapResponse(addr *net.UDPAddr, res kadproto.BootstrapRes) {
	t.mu.Lock()
	node := t.addOrUpdateNodeLocked(res.ID, addr, res.TCPPort, res.Version, false)
	t.confirmNodeLocked(node)
	for _, contact := range res.Contacts {
		contactNode := t.addOrUpdateNodeLocked(contact.ID, udpAddrFromKad(contact.Endpoint), contact.Endpoint.TCPPort, contact.Version, false)
		t.maybeSendHelloLocked(contactNode)
	}
	t.mu.Unlock()
}

func (t *DHTTracker) handleFindResponse(addr *net.UDPAddr, res kadproto.Res) {
	t.mu.Lock()
	for _, entry := range res.Results {
		entryNode := t.addOrUpdateNodeLocked(entry.ID, udpAddrFromKad(entry.Endpoint), entry.Endpoint.TCPPort, entry.Version, false)
		t.maybeSendHelloLocked(entryNode)
	}
	if node := t.nodes[addr.String()]; node != nil {
		node.LastSeen = time.Now()
		t.confirmNodeLocked(node)
	}
	t.mu.Unlock()
}

func (t *DHTTracker) handleHello(addr *net.UDPAddr, hello kadproto.Hello, reply bool) {
	t.mu.Lock()
	node := t.addOrUpdateNodeLocked(hello.ID, addr, hello.TCPPort, hello.Version, false)
	if node != nil {
		node.HelloSent = true
		t.confirmNodeLocked(node)
	}
	t.mu.Unlock()
	if reply {
		t.writePacket(addr, kadproto.Hello{
			ID:      t.selfID,
			TCPPort: uint16(t.ListenPort()),
			Version: kadproto.KademliaVersion,
		}, kadproto.HelloResOp)
	}
}

func (t *DHTTracker) handleBootstrapRequest(addr *net.UDPAddr) {
	t.mu.Lock()
	contacts := t.closestEntriesLocked(t.selfID, 20)
	t.mu.Unlock()
	_, _ = t.writePacketWithOpcode(addr, kadproto.BootstrapRes{
		ID:       t.selfID,
		TCPPort:  uint16(t.ListenPort()),
		Version:  kadproto.KademliaVersion,
		Contacts: contacts,
	})
}

func (t *DHTTracker) handleFindRequest(addr *net.UDPAddr, req kadproto.Req) {
	t.mu.Lock()
	contacts := t.closestEntriesLocked(req.Target, 20)
	t.mu.Unlock()
	_, _ = t.writePacketWithOpcode(addr, kadproto.Res{
		Target:  req.Target,
		Results: contacts,
	})
}

func (t *DHTTracker) handleSearchSourcesRequest(addr *net.UDPAddr, req kadproto.SearchSourcesReq) {
	t.mu.Lock()
	results := t.searchEntriesLocked(req.Target.Hash)
	t.mu.Unlock()
	if len(results) == 0 {
		return
	}
	t.sendSearchResults(addr, req.Target, results)
}

func (t *DHTTracker) handleSearchKeysRequest(addr *net.UDPAddr, req kadproto.SearchKeysReq) {
	t.mu.Lock()
	results := t.keywordEntriesLocked(req.Target.Hash)
	t.mu.Unlock()
	if len(results) == 0 {
		return
	}
	t.sendSearchResults(addr, req.Target, results)
}

func (t *DHTTracker) handlePublishSourcesRequest(addr *net.UDPAddr, req kadproto.PublishSourcesReq) {
	t.mu.Lock()
	if addr != nil {
		t.addSourceIPLocked(&req.Source, addr)
	}
	t.storeSourceLocked(req.FileID.Hash, req.Source)
	t.mu.Unlock()
	if addr != nil && t.storagePoint != nil {
		_, _ = t.writePacket(t.storagePoint, req)
	}
	_, _ = t.writePacket(addr, kadproto.PublishRes{
		FileID: req.FileID,
		Count:  1,
	})
}

func (t *DHTTracker) handlePublishKeysRequest(addr *net.UDPAddr, req kadproto.PublishKeysReq) {
	t.mu.Lock()
	for i, source := range req.Sources {
		if addr != nil {
			t.addSourceIPLocked(&source, addr)
			req.Sources[i] = source
		}
		t.storeKeywordLocked(req.KeywordID.Hash, source)
	}
	t.mu.Unlock()
	if addr != nil && t.storagePoint != nil {
		_, _ = t.writePacket(t.storagePoint, req)
	}
	_, _ = t.writePacket(addr, kadproto.PublishRes{
		FileID: req.KeywordID,
		Count:  1,
	})
}

func (t *DHTTracker) handlePublishNotesRequest(addr *net.UDPAddr, req kadproto.PublishNotesReq) {
	t.mu.Lock()
	for i, note := range req.Notes {
		if addr != nil {
			t.addSourceIPLocked(&note, addr)
			req.Notes[i] = note
		}
		t.storeNotesLocked(req.FileID.Hash, note)
	}
	t.mu.Unlock()
	if addr != nil && t.storagePoint != nil {
		_, _ = t.writePacket(t.storagePoint, req)
	}
	_, _ = t.writePacket(addr, kadproto.PublishNotesRes{
		PublishRes: kadproto.PublishRes{
			FileID: req.FileID,
			Count:  1,
		},
	})
}

func (t *DHTTracker) handleFirewalledRequest(addr *net.UDPAddr, req kadproto.FirewalledReq) {
	_ = req
	_, _ = t.writePacket(addr, kadproto.FirewalledRes{
		IP: protocolIP(addr.IP),
	})
}

func (t *DHTTracker) handleFirewalledResponse(addr *net.UDPAddr, res kadproto.FirewalledRes) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if node := t.nodes[addr.String()]; node != nil {
		t.confirmNodeLocked(node)
	}
	if res.IP != 0 && len(t.externalIPs) < 2 {
		t.externalIPs = append(t.externalIPs, res.IP)
	}
	if len(t.externalIPs) >= 2 {
		matchCount := 0
		localIP := uint32(0)
		if t.conn != nil {
			if udpAddr, ok := t.conn.LocalAddr().(*net.UDPAddr); ok {
				localIP = protocolIP(udpAddr.IP)
			}
		}
		for _, ip := range t.externalIPs[:2] {
			if ip != 0 && ip == localIP {
				matchCount++
			}
		}
		t.firewalled = matchCount != 2
		t.externalIPs = t.externalIPs[:0]
	}
}

func (t *DHTTracker) handleSearchNotesRequest(addr *net.UDPAddr, req kadproto.SearchNotesReq) {
	t.mu.Lock()
	results := t.notesEntriesLocked(req.Target.Hash)
	t.mu.Unlock()
	if len(results) == 0 {
		return
	}
	t.sendSearchResults(addr, req.Target, results)
}

func (t *DHTTracker) maybeSendHelloLocked(node *KadRoutingNode) {
	if node == nil || node.Addr == nil || node.HelloSent {
		return
	}
	node.HelloSent = true
	t.writePacketLocked(node.Addr, kadproto.Hello{
		ID:      t.selfID,
		TCPPort: uint16(t.ListenPort()),
		Version: kadproto.KademliaVersion,
	}, kadproto.HelloReqOp)
	t.rpc.Invoke(&kadRPCTransaction{endpointKey: node.Addr.String(), opcode: kadproto.HelloResOp})
}

func (t *DHTTracker) knownNodesLocked(requireID bool) []*KadRoutingNode {
	nodes := make([]*KadRoutingNode, 0, len(t.nodes))
	for _, node := range t.nodes {
		if node == nil || node.Addr == nil {
			continue
		}
		if requireID && !node.KnownID() {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (t *DHTTracker) PublishSource(hash protocol.Hash, endpoint protocol.Endpoint, size int64) bool {
	if !endpoint.Defined() {
		return false
	}
	if err := t.Start(); err != nil {
		return false
	}
	entry := kadproto.SearchEntry{
		ID: kadproto.NewID(hash),
		Tags: []kadproto.Tag{
			{Type: kadproto.TagTypeUint8, ID: kadproto.TagSourceType, UInt64: 1},
			{Type: kadproto.TagTypeUint32, ID: kadproto.TagSourceIP, UInt64: uint64(uint32(endpoint.IP()))},
			{Type: kadproto.TagTypeUint16, ID: kadproto.TagSourcePort, UInt64: uint64(uint16(endpoint.Port()))},
		},
	}
	if size > 0 {
		entry.Tags = append(entry.Tags, kadproto.Tag{Type: kadproto.TagTypeUint64, ID: 0xD3, UInt64: uint64(size)})
	}
	t.mu.Lock()
	t.storeSourceLocked(hash, entry)
	nodes := t.closestNodesLocked(kadproto.NewID(hash), 5, true)
	t.mu.Unlock()
	if len(nodes) == 0 {
		return false
	}
	req := kadproto.PublishSourcesReq{FileID: kadproto.NewID(hash), Source: entry}
	for _, node := range nodes {
		if node == nil || node.Addr == nil {
			continue
		}
		_, _ = t.writePacket(node.Addr, req)
	}
	return true
}

func (t *DHTTracker) PublishKeyword(keywordHash protocol.Hash, entries ...kadproto.SearchEntry) bool {
	if len(entries) == 0 {
		return false
	}
	if err := t.Start(); err != nil {
		return false
	}
	t.mu.Lock()
	for _, entry := range entries {
		t.storeKeywordLocked(keywordHash, entry)
	}
	nodes := t.closestNodesLocked(kadproto.NewID(keywordHash), 5, true)
	t.mu.Unlock()
	if len(nodes) == 0 {
		return false
	}
	req := kadproto.PublishKeysReq{KeywordID: kadproto.NewID(keywordHash), Sources: entries}
	for _, node := range nodes {
		if node == nil || node.Addr == nil {
			continue
		}
		_, _ = t.writePacket(node.Addr, req)
	}
	return true
}

func (t *DHTTracker) PublishNotes(fileHash protocol.Hash, entries ...kadproto.SearchEntry) bool {
	if len(entries) == 0 {
		return false
	}
	if err := t.Start(); err != nil {
		return false
	}
	t.mu.Lock()
	for _, entry := range entries {
		t.storeNotesLocked(fileHash, entry)
	}
	nodes := t.closestNodesLocked(kadproto.NewID(fileHash), 5, true)
	t.mu.Unlock()
	if len(nodes) == 0 {
		return false
	}
	req := kadproto.PublishNotesReq{FileID: kadproto.NewID(fileHash), Notes: entries}
	for _, node := range nodes {
		if node == nil || node.Addr == nil {
			continue
		}
		_, _ = t.writePacket(node.Addr, req)
	}
	return true
}

func (t *DHTTracker) SearchKeywords(hash protocol.Hash, cb func([]kadproto.SearchEntry)) bool {
	if cb == nil {
		return false
	}
	if err := t.Start(); err != nil {
		return false
	}
	t.mu.Lock()
	if len(t.nodes) == 0 && len(t.table.RouterNodes()) == 0 {
		t.mu.Unlock()
		return false
	}
	t.mu.Unlock()
	return t.node.searchKeywords(hash, cb)
}

func (t *DHTTracker) SetStoragePoint(addr *net.UDPAddr) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.storagePoint = normalizeUDPAddr(addr)
}

func (t *DHTTracker) IsFirewalled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.firewalled
}

func (t *DHTTracker) ListenPort() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.listenPort
}

func (t *DHTTracker) setListenPort(port int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.listenPort = port
}

func (t *DHTTracker) Status() DHTStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	live, replacements := t.table.Size()
	status := DHTStatus{
		Firewalled:       t.firewalled,
		LiveNodes:        live,
		ReplacementNodes: replacements,
		RouterNodes:      len(t.table.RouterNodes()),
		KnownNodes:       len(t.nodes),
		ListenPort:       t.listenPort,
	}
	if t.node != nil {
		status.RunningTraversals = len(t.node.running)
		status.InitialBootstrap = t.node.initialBootstrapRequired
	}
	status.Bootstrapped = live > 0
	if t.storagePoint != nil {
		status.StoragePoint = t.storagePoint.String()
	}
	return status
}

func (t *DHTTracker) SnapshotState() *ClientDHTState {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := &ClientDHTState{
		SelfID:              t.selfID.Hash,
		Firewalled:          t.firewalled,
		LastBootstrap:       timeToMillis(t.lastBootstrap),
		LastRefresh:         timeToMillis(t.lastRefresh),
		LastFirewalledCheck: timeToMillis(t.lastFirewalledCheck),
		Nodes:               make([]ClientDHTNodeState, 0, len(t.nodes)),
		RouterNodes:         make([]string, 0, len(t.table.RouterNodes())),
	}
	if t.storagePoint != nil {
		state.StoragePoint = t.storagePoint.String()
	}
	for _, addr := range t.table.RouterNodes() {
		if addr != nil {
			state.RouterNodes = append(state.RouterNodes, addr.String())
		}
	}
	for _, node := range t.nodes {
		if node == nil || node.Addr == nil {
			continue
		}
		state.Nodes = append(state.Nodes, ClientDHTNodeState{
			ID:        node.ID.Hash,
			Addr:      node.Addr.String(),
			TCPPort:   node.TCPPort,
			Version:   node.Version,
			Seed:      node.Seed,
			HelloSent: node.HelloSent,
			Pinged:    node.Pinged,
			FailCount: node.FailCount,
			FirstSeen: timeToMillis(node.FirstSeen),
			LastSeen:  timeToMillis(node.LastSeen),
		})
	}
	return state
}

func (t *DHTTracker) ApplyState(state *ClientDHTState) error {
	if state == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !state.SelfID.Equal(protocol.Invalid) {
		t.selfID = kadproto.NewID(state.SelfID)
	}
	t.table = newKadRoutingTable(t.selfID, 10)
	t.nodes = make(map[string]*KadRoutingNode, len(state.Nodes))
	t.firewalled = state.Firewalled
	t.lastBootstrap = millisToTime(state.LastBootstrap)
	t.lastRefresh = millisToTime(state.LastRefresh)
	t.lastFirewalledCheck = millisToTime(state.LastFirewalledCheck)
	t.table.lastBootstrap = t.lastBootstrap
	t.table.lastRefresh = t.lastRefresh
	t.storagePoint = nil
	if state.StoragePoint != "" {
		if addr, err := net.ResolveUDPAddr("udp", state.StoragePoint); err == nil {
			t.storagePoint = normalizeUDPAddr(addr)
		}
	}
	for _, router := range state.RouterNodes {
		addr, err := net.ResolveUDPAddr("udp", router)
		if err != nil {
			continue
		}
		t.table.AddRouterNode(addr)
	}
	for _, record := range state.Nodes {
		addr, err := net.ResolveUDPAddr("udp", record.Addr)
		if err != nil {
			continue
		}
		node := &KadRoutingNode{
			ID:        kadproto.NewID(record.ID),
			Addr:      normalizeUDPAddr(addr),
			TCPPort:   record.TCPPort,
			Version:   record.Version,
			Seed:      record.Seed,
			HelloSent: record.HelloSent,
			Pinged:    record.Pinged,
			FailCount: record.FailCount,
			FirstSeen: millisToTime(record.FirstSeen),
			LastSeen:  millisToTime(record.LastSeen),
		}
		t.nodes[node.Addr.String()] = node
		if node.Pinged {
			t.table.NodeSeen(node)
		} else {
			t.table.HeardAbout(node)
		}
	}
	if t.node != nil {
		live, _ := t.table.Size()
		t.node.initialBootstrapRequired = live == 0
	}
	return nil
}

func (t *DHTTracker) closestNodesLocked(target kadproto.ID, limit int, requireID bool) []*KadRoutingNode {
	var nodes []*KadRoutingNode
	if requireID {
		nodes = t.table.FindClosest(target, limit, true)
	} else {
		nodes = t.knownNodesLocked(false)
	}
	if len(nodes) == 0 {
		return nil
	}
	if !requireID {
		sortKadNodesByDistance(nodes, target)
	}
	if limit > 0 && len(nodes) > limit {
		nodes = nodes[:limit]
	}
	return nodes
}

func (t *DHTTracker) closestEntriesLocked(target kadproto.ID, limit int) []kadproto.Entry {
	nodes := t.closestNodesLocked(target, limit, true)
	res := make([]kadproto.Entry, 0, len(nodes))
	for _, node := range nodes {
		if node == nil || node.Addr == nil {
			continue
		}
		res = append(res, kadproto.Entry{
			ID: node.ID,
			Endpoint: kadproto.Endpoint{
				IP:      protocolIP(node.Addr.IP),
				UDPPort: uint16(node.Addr.Port),
				TCPPort: node.TCPPort,
			},
			Version: node.Version,
		})
	}
	return res
}

func (t *DHTTracker) addOrUpdateNode(id kadproto.ID, addr *net.UDPAddr, tcpPort uint16, version byte, seed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.addOrUpdateNodeLocked(id, addr, tcpPort, version, seed)
}

func (t *DHTTracker) addOrUpdateNodeLocked(id kadproto.ID, addr *net.UDPAddr, tcpPort uint16, version byte, seed bool) *KadRoutingNode {
	addr = normalizeUDPAddr(addr)
	if addr == nil {
		return nil
	}
	key := addr.String()
	node := t.nodes[key]
	if node == nil {
		node = &KadRoutingNode{Addr: addr}
		t.nodes[key] = node
	}
	node.Addr = addr
	if !id.Hash.Equal(protocol.Invalid) {
		node.ID = id
	}
	if tcpPort != 0 {
		node.TCPPort = tcpPort
	}
	if version != 0 {
		node.Version = version
	}
	node.Seed = node.Seed || seed
	node.LastSeen = time.Now()
	if node.FirstSeen.IsZero() {
		node.FirstSeen = node.LastSeen
	}
	if node.KnownID() {
		t.table.HeardAbout(node)
	}
	return node
}

func (t *DHTTracker) storeSourceLocked(hash protocol.Hash, entry kadproto.SearchEntry) {
	key := hash.String()
	bucket := t.sourceIndex[key]
	if bucket == nil {
		bucket = make(map[string]kadproto.SearchEntry)
		t.sourceIndex[key] = bucket
	}
	entryKey := entry.ID.Hash.String()
	if endpoint, ok := entry.SourceEndpoint(); ok && endpoint.Defined() {
		entryKey = endpoint.String()
	}
	bucket[entryKey] = entry
}

func (t *DHTTracker) storeKeywordLocked(hash protocol.Hash, entry kadproto.SearchEntry) {
	key := hash.String()
	bucket := t.keywordIndex[key]
	if bucket == nil {
		bucket = make(map[string]kadproto.SearchEntry)
		t.keywordIndex[key] = bucket
	}
	entryKey := entry.ID.Hash.String()
	if name, ok := entry.StringTag(0x01); ok && name != "" {
		entryKey = entryKey + ":" + name
	}
	bucket[entryKey] = entry
}

func (t *DHTTracker) storeNotesLocked(hash protocol.Hash, entry kadproto.SearchEntry) {
	key := hash.String()
	bucket := t.notesIndex[key]
	if bucket == nil {
		bucket = make(map[string]kadproto.SearchEntry)
		t.notesIndex[key] = bucket
	}
	entryKey := entry.ID.Hash.String()
	if endpoint, ok := entry.SourceEndpoint(); ok && endpoint.Defined() {
		entryKey = endpoint.String()
	}
	bucket[entryKey] = entry
}

func (t *DHTTracker) searchEntriesLocked(hash protocol.Hash) []kadproto.SearchEntry {
	bucket := t.sourceIndex[hash.String()]
	if len(bucket) == 0 {
		return nil
	}
	results := make([]kadproto.SearchEntry, 0, len(bucket))
	for _, entry := range bucket {
		results = append(results, entry)
	}
	return results
}

func (t *DHTTracker) keywordEntriesLocked(hash protocol.Hash) []kadproto.SearchEntry {
	bucket := t.keywordIndex[hash.String()]
	if len(bucket) == 0 {
		return nil
	}
	results := make([]kadproto.SearchEntry, 0, len(bucket))
	for _, entry := range bucket {
		results = append(results, entry)
	}
	return results
}

func (t *DHTTracker) notesEntriesLocked(hash protocol.Hash) []kadproto.SearchEntry {
	bucket := t.notesIndex[hash.String()]
	if len(bucket) == 0 {
		return nil
	}
	results := make([]kadproto.SearchEntry, 0, len(bucket))
	for _, entry := range bucket {
		results = append(results, entry)
	}
	return results
}

func (t *DHTTracker) writePacket(addr *net.UDPAddr, packet any, extra ...byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.writePacketWithOpcodeLocked(addr, packet, extra...)
}

func (t *DHTTracker) writePacketLocked(addr *net.UDPAddr, packet any, extra ...byte) {
	_, _ = t.writePacketWithOpcodeLocked(addr, packet, extra...)
}

func (t *DHTTracker) writePacketWithOpcode(addr *net.UDPAddr, packet any, extra ...byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.writePacketWithOpcodeLocked(addr, packet, extra...)
}

func (t *DHTTracker) writePacketWithOpcodeLocked(addr *net.UDPAddr, packet any, extra ...byte) (int, error) {
	if t.conn == nil || addr == nil {
		return 0, errors.New("dht tracker socket is not ready")
	}
	raw, err := t.combiner.Pack(packet, extra...)
	if err != nil {
		return 0, err
	}
	return t.conn.WriteToUDP(raw, addr)
}

func (t *DHTTracker) confirmNodeLocked(node *KadRoutingNode) {
	if node == nil || !node.KnownID() {
		return
	}
	node.Pinged = true
	node.FailCount = 0
	node.LastSeen = time.Now()
	t.table.NodeSeen(node)
}

func errorsIsClosed(err error) bool {
	return err != nil && strings.Contains(err.Error(), "closed network connection")
}

func normalizeUDPAddr(addr *net.UDPAddr) *net.UDPAddr {
	if addr == nil {
		return nil
	}
	ip4 := addr.IP.To4()
	if ip4 == nil {
		return addr
	}
	return &net.UDPAddr{IP: ip4, Port: addr.Port}
}

func udpAddrFromKad(endpoint kadproto.Endpoint) *net.UDPAddr {
	ip := net.IPv4(byte(endpoint.IP), byte(endpoint.IP>>8), byte(endpoint.IP>>16), byte(endpoint.IP>>24))
	return &net.UDPAddr{IP: ip, Port: int(endpoint.UDPPort)}
}

func protocolIP(ip net.IP) uint32 {
	ip4 := ip.To4()
	if len(ip4) != 4 {
		return 0
	}
	return uint32(ip4[0]) | uint32(ip4[1])<<8 | uint32(ip4[2])<<16 | uint32(ip4[3])<<24
}

func timeToMillis(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func millisToTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}

func (t *DHTTracker) sendSearchResults(addr *net.UDPAddr, target kadproto.ID, results []kadproto.SearchEntry) {
	if addr == nil || len(results) == 0 {
		return
	}
	const maxEntriesPerPacket = 50
	const outputLimit = 8128
	packet := kadproto.SearchRes{Source: t.selfID, Target: target, Results: make([]kadproto.SearchEntry, 0, maxEntriesPerPacket)}
	bytesAllocated := 0
	for _, entry := range results {
		entrySize := kadSearchEntrySize(entry)
		if len(packet.Results) >= maxEntriesPerPacket || bytesAllocated+entrySize+32 > outputLimit {
			_, _ = t.writePacket(addr, packet)
			packet = kadproto.SearchRes{Source: t.selfID, Target: target, Results: make([]kadproto.SearchEntry, 0, maxEntriesPerPacket)}
			bytesAllocated = 0
		}
		packet.Results = append(packet.Results, entry)
		bytesAllocated += entrySize
	}
	if len(packet.Results) > 0 {
		_, _ = t.writePacket(addr, packet)
	}
}

func kadSearchEntrySize(entry kadproto.SearchEntry) int {
	size := 16 + 1
	for _, tag := range entry.Tags {
		size += 2
		switch tag.Type {
		case kadproto.TagTypeUint8:
			size++
		case kadproto.TagTypeUint16:
			size += 2
		case kadproto.TagTypeUint32:
			size += 4
		case kadproto.TagTypeUint64:
			size += 8
		case kadproto.TagTypeString:
			size += 2 + len(tag.String)
		default:
			if tag.Type >= kadproto.TagTypeStr1 && tag.Type <= kadproto.TagTypeStr1+15 {
				size += len(tag.String)
			}
		}
	}
	return size
}

func (t *DHTTracker) addSourceIPLocked(entry *kadproto.SearchEntry, addr *net.UDPAddr) {
	if entry == nil || addr == nil {
		return
	}
	for _, tag := range entry.Tags {
		if tag.ID == kadproto.TagSourceIP {
			return
		}
	}
	entry.Tags = append([]kadproto.Tag{{Type: kadproto.TagTypeUint32, ID: kadproto.TagSourceIP, UInt64: uint64(protocolIP(addr.IP))}}, entry.Tags...)
}
