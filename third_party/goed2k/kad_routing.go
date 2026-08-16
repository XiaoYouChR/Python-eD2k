package goed2k

import (
	"net"
	"sort"
	"time"

	"github.com/monkeyWie/goed2k/protocol"
	kadproto "github.com/monkeyWie/goed2k/protocol/kad"
)

type KadRoutingNode struct {
	ID        kadproto.ID
	Addr      *net.UDPAddr
	TCPPort   uint16
	Version   byte
	Seed      bool
	HelloSent bool
	Pinged    bool
	FailCount int
	FirstSeen time.Time
	LastSeen  time.Time
}

func (n *KadRoutingNode) Key() string {
	if n == nil || n.Addr == nil {
		return ""
	}
	return n.Addr.String()
}

func (n *KadRoutingNode) KnownID() bool {
	if n == nil {
		return false
	}
	return !n.ID.Hash.Equal(protocol.Invalid)
}

func sortKadNodesByDistance(nodes []*KadRoutingNode, target kadproto.ID) {
	sort.Slice(nodes, func(i, j int) bool {
		return kadproto.DistanceCompare(nodes[i].ID, nodes[j].ID, target) < 0
	})
}

type kadRoutingBucket struct {
	live         []*KadRoutingNode
	replacements []*KadRoutingNode
	lastActive   time.Time
}

type kadRoutingTable struct {
	self            kadproto.ID
	bucketSize      int
	buckets         []kadRoutingBucket
	routerNodes     map[string]*net.UDPAddr
	lastBootstrap   time.Time
	lastRefresh     time.Time
	lastSelfRefresh time.Time
}

func newKadRoutingTable(self kadproto.ID, bucketSize int) *kadRoutingTable {
	if bucketSize <= 0 {
		bucketSize = 10
	}
	buckets := make([]kadRoutingBucket, 128)
	now := time.Now()
	for i := range buckets {
		buckets[i].lastActive = now
	}
	return &kadRoutingTable{
		self:        self,
		bucketSize:  bucketSize,
		buckets:     buckets,
		routerNodes: make(map[string]*net.UDPAddr),
	}
}

func (t *kadRoutingTable) AddRouterNode(addr *net.UDPAddr) {
	if t == nil || addr == nil {
		return
	}
	t.routerNodes[addr.String()] = normalizeUDPAddr(addr)
}

func (t *kadRoutingTable) RouterNodes() []*net.UDPAddr {
	if t == nil {
		return nil
	}
	res := make([]*net.UDPAddr, 0, len(t.routerNodes))
	for _, addr := range t.routerNodes {
		if addr != nil {
			res = append(res, addr)
		}
	}
	return res
}

func (t *kadRoutingTable) HeardAbout(node *KadRoutingNode) {
	t.addNode(node, false)
}

func (t *kadRoutingTable) NodeSeen(node *KadRoutingNode) {
	t.addNode(node, true)
}

func (t *kadRoutingTable) addNode(node *KadRoutingNode, confirmed bool) {
	if t == nil || node == nil || !node.KnownID() {
		return
	}
	index := kadBucketIndex(t.self, node.ID)
	bucket := &t.buckets[index]
	bucket.lastActive = time.Now()

	if existing, pos := findKadNode(bucket.live, node); pos >= 0 {
		existing.Addr = node.Addr
		existing.TCPPort = node.TCPPort
		existing.Version = node.Version
		existing.LastSeen = time.Now()
		if confirmed {
			existing.Pinged = true
			existing.FailCount = 0
		}
		moveKadNodeToEnd(bucket.live, pos)
		return
	}
	if existing, pos := findKadNode(bucket.replacements, node); pos >= 0 {
		existing.Addr = node.Addr
		existing.TCPPort = node.TCPPort
		existing.Version = node.Version
		existing.LastSeen = time.Now()
		if confirmed {
			existing.Pinged = true
			existing.FailCount = 0
			bucket.replacements = append(bucket.replacements[:pos], bucket.replacements[pos+1:]...)
			if len(bucket.live) < t.bucketSize {
				bucket.live = append(bucket.live, existing)
			} else {
				bucket.replacements = append(bucket.replacements, existing)
			}
		}
		return
	}

	node.LastSeen = time.Now()
	if node.FirstSeen.IsZero() {
		node.FirstSeen = node.LastSeen
	}
	if confirmed {
		node.Pinged = true
		node.FailCount = 0
	}
	if len(bucket.live) < t.bucketSize {
		bucket.live = append(bucket.live, node)
		return
	}
	if len(bucket.replacements) < t.bucketSize {
		bucket.replacements = append(bucket.replacements, node)
		return
	}
	bucket.replacements = append(bucket.replacements[1:], node)
}

func (t *kadRoutingTable) NodeFailed(node *KadRoutingNode) {
	if t == nil || node == nil || !node.KnownID() {
		return
	}
	index := kadBucketIndex(t.self, node.ID)
	bucket := &t.buckets[index]
	if existing, pos := findKadNode(bucket.live, node); pos >= 0 {
		if !existing.Pinged {
			bucket.live = append(bucket.live[:pos], bucket.live[pos+1:]...)
			return
		}
		existing.FailCount++
		if len(bucket.replacements) > 0 {
			repl := bucket.replacements[0]
			bucket.replacements = bucket.replacements[1:]
			bucket.live[pos] = repl
			return
		}
		if existing.FailCount >= 20 {
			bucket.live = append(bucket.live[:pos], bucket.live[pos+1:]...)
		}
		return
	}
	if _, pos := findKadNode(bucket.replacements, node); pos >= 0 {
		bucket.replacements = append(bucket.replacements[:pos], bucket.replacements[pos+1:]...)
	}
}

func (t *kadRoutingTable) FindClosest(target kadproto.ID, limit int, includeUnconfirmed bool) []*KadRoutingNode {
	if t == nil {
		return nil
	}
	nodes := make([]*KadRoutingNode, 0, len(t.buckets)*t.bucketSize)
	for i := range t.buckets {
		for _, node := range t.buckets[i].live {
			if node == nil {
				continue
			}
			if !includeUnconfirmed && !node.Pinged {
				continue
			}
			nodes = append(nodes, node)
		}
	}
	sortKadNodesByDistance(nodes, target)
	if limit > 0 && len(nodes) > limit {
		nodes = nodes[:limit]
	}
	return nodes
}

func (t *kadRoutingTable) TouchBucket(target kadproto.ID) {
	if t == nil {
		return
	}
	t.buckets[kadBucketIndex(t.self, target)].lastActive = time.Now()
}

func (t *kadRoutingTable) NeedBootstrap(now time.Time) bool {
	if t == nil {
		return false
	}
	live, replacements := t.Size()
	if live > 0 || replacements >= t.bucketSize {
		return false
	}
	if t.lastBootstrap.IsZero() || now.Sub(t.lastBootstrap) >= 30*time.Second {
		t.lastBootstrap = now
		return true
	}
	return false
}

func (t *kadRoutingTable) NeedRefresh(now time.Time) *kadproto.ID {
	if t == nil {
		return nil
	}
	if t.lastSelfRefresh.IsZero() || now.Sub(t.lastSelfRefresh) >= 15*time.Minute {
		t.lastSelfRefresh = now
		target := t.self
		return &target
	}
	index := -1
	var oldest time.Time
	for i := range t.buckets {
		last := t.buckets[i].lastActive
		if now.Sub(last) < 15*time.Minute {
			continue
		}
		if index < 0 || last.Before(oldest) {
			index = i
			oldest = last
		}
	}
	if index < 0 {
		return nil
	}
	if !t.lastRefresh.IsZero() && now.Sub(t.lastRefresh) < 45*time.Second {
		return nil
	}
	t.lastRefresh = now
	target := randomKadIDWithinBucket(index, t.self)
	return &target
}

func (t *kadRoutingTable) Size() (live int, replacements int) {
	if t == nil {
		return 0, 0
	}
	for i := range t.buckets {
		live += len(t.buckets[i].live)
		replacements += len(t.buckets[i].replacements)
	}
	return live, replacements
}

func kadBucketIndex(self, other kadproto.ID) int {
	for i := 0; i < 16; i++ {
		x := self.Hash.At(i) ^ other.Hash.At(i)
		if x == 0 {
			continue
		}
		for bit := 0; bit < 8; bit++ {
			mask := byte(0x80 >> bit)
			if x&mask != 0 {
				return i*8 + bit
			}
		}
	}
	return 127
}

func randomKadIDWithinBucket(index int, self kadproto.ID) kadproto.ID {
	hash, err := protocol.RandomHash(false)
	if err != nil {
		return self
	}
	id := kadproto.NewID(hash)
	byteIndex := index / 8
	bitIndex := index % 8
	for i := 0; i < byteIndex; i++ {
		id.Hash.Set(i, self.Hash.At(i))
	}
	mask := byte(0x80 >> bitIndex)
	id.Hash.Set(byteIndex, (self.Hash.At(byteIndex)&^mask)|(^self.Hash.At(byteIndex)&mask))
	return id
}

func findKadNode(nodes []*KadRoutingNode, needle *KadRoutingNode) (*KadRoutingNode, int) {
	for i, node := range nodes {
		if node == nil || needle == nil {
			continue
		}
		if node.KnownID() && needle.KnownID() && node.ID.Hash.Equal(needle.ID.Hash) {
			return node, i
		}
		if node.Addr != nil && needle.Addr != nil && node.Addr.String() == needle.Addr.String() {
			return node, i
		}
	}
	return nil, -1
}

func moveKadNodeToEnd(nodes []*KadRoutingNode, index int) {
	if index < 0 || index >= len(nodes) {
		return
	}
	node := nodes[index]
	copy(nodes[index:], nodes[index+1:])
	nodes[len(nodes)-1] = node
}
