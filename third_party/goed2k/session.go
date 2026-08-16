package goed2k

import (
	"errors"
	"net"
	"os"
	"sync"

	"github.com/monkeyWie/goed2k/disk"
	"github.com/monkeyWie/goed2k/internal/logx"
	"github.com/monkeyWie/goed2k/protocol"
	kadproto "github.com/monkeyWie/goed2k/protocol/kad"
	serverproto "github.com/monkeyWie/goed2k/protocol/server"
)

type Session struct {
	mu                     sync.Mutex
	diskMu                 sync.Mutex
	searchMu               sync.Mutex
	transfers              map[protocol.Hash]*Transfer
	connections            []*PeerConnection
	callbacks              map[int32]protocol.Hash
	settings               Settings
	lastTick               int64
	accumulator            Statistics
	serverConnection       *ServerConnection
	serverConnections      map[string]*ServerConnection
	serverConnectionPolicy map[string]*ServerConnectionPolicy
	configuredServers      map[string]*net.TCPAddr
	clientID               int32
	tcpFlags               int32
	auxPort                int32
	diskTasks              []*diskTask
	diskResults            chan diskTaskResult
	listener               *net.TCPListener
	incomingConns          chan net.Conn
	dhtTracker             *DHTTracker
	upnp                   *upnpManager
	uploadQueue            *UploadQueue
	credits                *PeerCreditManager
	friendSlots            map[string]bool
	activeSearch           *searchTask
	nextSearchID           uint32
}

type diskTask struct {
	callable TransferCallable
	transfer *Transfer
	done     bool
	running  bool
}

type diskTaskResult struct {
	task   *diskTask
	result AsyncOperationResult
}

func NewSession(st Settings) *Session {
	session := &Session{
		transfers:              make(map[protocol.Hash]*Transfer),
		connections:            make([]*PeerConnection, 0),
		callbacks:              make(map[int32]protocol.Hash),
		settings:               st,
		lastTick:               CurrentTime(),
		accumulator:            NewStatistics(),
		serverConnections:      make(map[string]*ServerConnection),
		serverConnectionPolicy: make(map[string]*ServerConnectionPolicy),
		configuredServers:      make(map[string]*net.TCPAddr),
		diskTasks:              make([]*diskTask, 0),
		diskResults:            make(chan diskTaskResult, 128),
		incomingConns:          make(chan net.Conn, 32),
		credits:                NewPeerCreditManager(),
		friendSlots:            make(map[string]bool),
	}
	session.initUploadQueue()
	return session
}

func (s *Session) initUploadQueue() {
	if s.uploadQueue == nil {
		s.uploadQueue = NewUploadQueue(s)
	}
}

func (s *Session) GetCurrentTime() int64 {
	return s.lastTick
}

func (s *Session) ConfigureSession(st Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = st
}

func (s *Session) AddTransfer(hash protocol.Hash, size int64, file *os.File) (TransferHandle, error) {
	s.mu.Lock()
	if t, ok := s.transfers[hash]; ok {
		s.mu.Unlock()
		return NewTransferHandleWithTransfer(s, t), nil
	}
	s.mu.Unlock()

	t, err := NewTransfer(s, NewAddTransferParamsFromFile(hash, CurrentTimeMillis(), size, file, false))
	if err != nil {
		return NewTransferHandle(s), err
	}
	s.mu.Lock()
	if existing, ok := s.transfers[hash]; ok {
		s.mu.Unlock()
		return NewTransferHandleWithTransfer(s, existing), nil
	}
	s.transfers[hash] = t
	s.mu.Unlock()
	return NewTransferHandleWithTransfer(s, t), nil
}

func (s *Session) AddTransferWithHandler(hash protocol.Hash, size int64, handler disk.FileHandler) (TransferHandle, error) {
	s.mu.Lock()
	if t, ok := s.transfers[hash]; ok {
		s.mu.Unlock()
		return NewTransferHandleWithTransfer(s, t), nil
	}
	s.mu.Unlock()

	t, err := NewTransfer(s, NewAddTransferParamsFromHandler(hash, CurrentTimeMillis(), size, handler, false))
	if err != nil {
		return NewTransferHandle(s), err
	}
	s.mu.Lock()
	if existing, ok := s.transfers[hash]; ok {
		s.mu.Unlock()
		return NewTransferHandleWithTransfer(s, existing), nil
	}
	s.transfers[hash] = t
	s.mu.Unlock()
	return NewTransferHandleWithTransfer(s, t), nil
}

func (s *Session) AddTransferParams(atp AddTransferParams) (TransferHandle, error) {
	s.mu.Lock()
	if t, ok := s.transfers[atp.Hash]; ok {
		s.mu.Unlock()
		return NewTransferHandleWithTransfer(s, t), nil
	}
	s.mu.Unlock()

	t, err := NewTransfer(s, atp)
	if err != nil {
		return NewTransferHandle(s), err
	}
	s.mu.Lock()
	if existing, ok := s.transfers[atp.Hash]; ok {
		s.mu.Unlock()
		return NewTransferHandleWithTransfer(s, existing), nil
	}
	s.transfers[atp.Hash] = t
	s.mu.Unlock()
	return NewTransferHandleWithTransfer(s, t), nil
}

func (s *Session) FindTransfer(hash protocol.Hash) TransferHandle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return NewTransferHandleWithTransfer(s, s.transfers[hash])
}

func (s *Session) LookupTransfer(hash protocol.Hash) *Transfer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transfers[hash]
}

func (s *Session) RemoveTransfer(hash protocol.Hash, deleteFile bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := s.transfers[hash]
	if t == nil {
		return nil
	}
	if err := t.Abort(deleteFile); err != nil {
		return err
	}
	delete(s.transfers, hash)
	return nil
}

func (s *Session) GetTransfers() []TransferHandle {
	s.mu.Lock()
	defer s.mu.Unlock()

	handles := make([]TransferHandle, 0, len(s.transfers))
	for _, t := range s.transfers {
		handles = append(handles, NewTransferHandleWithTransfer(s, t))
	}
	return handles
}

func (s *Session) CloseConnection(connection *PeerConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dst := s.connections[:0]
	for _, c := range s.connections {
		if c != connection {
			dst = append(dst, c)
		}
	}
	s.connections = dst
}

func (s *Session) GetUserAgent() protocol.Hash {
	return s.settings.UserAgent
}

func (s *Session) GetClientID() int32 {
	return s.clientID
}

func (s *Session) GetListenPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings.ListenPort
}

func (s *Session) GetUDPPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dhtTracker != nil {
		if port := s.dhtTracker.ListenPort(); port > 0 {
			return port
		}
	}
	return s.settings.UDPPort
}

func (s *Session) GetClientName() string {
	return s.settings.ClientName
}

func (s *Session) GetModName() string {
	return s.settings.ModName
}

func (s *Session) GetAppVersion() int {
	return s.settings.Version
}

func (s *Session) GetCompressionVersion() int {
	return s.settings.CompressionVersion
}

func (s *Session) GetModMajorVersion() int {
	return s.settings.ModMajor
}

func (s *Session) GetModMinorVersion() int {
	return s.settings.ModMinor
}

func (s *Session) GetModBuildVersion() int {
	return s.settings.ModBuild
}

func (s *Session) SendSourcesRequest(hash protocol.Hash, size int64) bool {
	sent := false
	for _, sc := range s.activeServerConnections() {
		if sc == nil || !sc.IsHandshakeCompleted() {
			continue
		}
		sc.SendFileSourcesRequest(hash, size)
		sent = true
	}
	return sent
}

func (s *Session) SendDHTSourcesRequest(hash protocol.Hash, size int64, transfer *Transfer) bool {
	if s.dhtTracker == nil || transfer == nil {
		return false
	}
	return s.dhtTracker.SearchSources(hash, size, func(results []kadproto.SearchEntry) {
		logx.Debug("dht source search result", "hash", hash.String(), "results", len(results))
		s.mu.Lock()
		current := s.transfers[hash]
		s.mu.Unlock()
		if current == nil || current != transfer {
			return
		}
		for _, entry := range results {
			endpoint, ok := entry.SourceEndpoint()
			if !ok || !endpoint.Defined() {
				continue
			}
			_ = current.AddPeer(endpoint, int(PeerDHT))
		}
	})
}

func (s *Session) RequestSourcesNow(transfer *Transfer) bool {
	if transfer == nil || transfer.IsPaused() || transfer.IsAborted() || transfer.IsFinished() {
		return false
	}
	transfer.ForceSourceDiscoveryNow()
	activePeers := transfer.ActiveConnections()
	knownPeers := transfer.policy.Size()
	serverSent := s.SendSourcesRequest(transfer.hash, transfer.size)
	sent := serverSent
	dhtSent := s.SendDHTSourcesRequest(transfer.hash, transfer.size, transfer)
	if dhtSent {
		sent = true
	}
	now := CurrentTime()
	transfer.nextSourcesRequest = now + transfer.nextServerSourcesInterval(activePeers, knownPeers, serverSent)
	transfer.nextDHTRequest = now + transfer.nextDHTSourcesInterval(activePeers, knownPeers, dhtSent)
	logx.Debug("source discovery triggered", "hash", transfer.hash.String(), "server", serverSent, "dht", dhtSent, "active_peers", activePeers, "known_peers", knownPeers)
	return sent
}

func (s *Session) ConnectNewPeers() {
	stepsSinceLastConnect := 0
	maxConnectionsPerSecond := s.settings.MaxConnectionsPerSecond
	transfers := s.snapshotTransfers()
	connections := s.snapshotConnections()
	numTransfers := len(transfers)
	enumerateCandidates := true
	if numTransfers > 0 && len(connections) < s.settings.SessionConnectionsLimit {
		for enumerateCandidates {
			for _, t := range transfers {
				if t.WantMorePeers() {
					connected, _ := t.TryConnectPeer(CurrentTime())
					if connected {
						maxConnectionsPerSecond--
						stepsSinceLastConnect = 0
					}
				}
				stepsSinceLastConnect++
				if stepsSinceLastConnect > numTransfers*2 {
					enumerateCandidates = false
					break
				}
				if maxConnectionsPerSecond == 0 {
					enumerateCandidates = false
					break
				}
			}
			if len(transfers) == 0 {
				break
			}
		}
	}
}

func (s *Session) Listen() error {
	if s.listener != nil {
		return nil
	}
	s.mu.Lock()
	port := s.settings.ListenPort
	s.mu.Unlock()
	if port < 0 {
		return nil
	}
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		return err
	}
	if addr, ok := listener.Addr().(*net.TCPAddr); ok {
		s.mu.Lock()
		s.settings.ListenPort = addr.Port
		s.mu.Unlock()
	}
	s.listener = listener
	go func(l *net.TCPListener) {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			select {
			case s.incomingConns <- conn:
			default:
				_ = conn.Close()
			}
		}
	}(listener)
	s.startUPnPMapping()
	return nil
}

func (s *Session) CloseListener() {
	s.stopUPnPMapping()
	if s.listener == nil {
		return
	}
	_ = s.listener.Close()
	s.listener = nil
}

func (s *Session) SyncDHTListenPort() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dhtTracker == nil {
		return
	}
	if port := s.dhtTracker.ListenPort(); port > 0 {
		s.settings.UDPPort = port
	}
}

func (s *Session) RefreshUPnPMapping() {
	s.stopUPnPMapping()
	s.startUPnPMapping()
}

func (s *Session) SecondTick(currentSessionTime, tickIntervalMS int64) {
	s.PumpIO()
	for _, t := range s.snapshotTransfers() {
		t.SecondTick(&s.accumulator, tickIntervalMS)
	}
	for _, sc := range s.activeServerConnections() {
		if sc != nil {
			sc.SecondTick(tickIntervalMS)
		}
	}
	if s.settings.ReconnectToServer {
		s.connectConfiguredServers(currentSessionTime)
	}
	if s.uploadQueue != nil {
		s.uploadQueue.Process()
	}
	s.tickSearches(currentSessionTime)
	s.processDiskTasks()
	s.accumulator.SecondTick(tickIntervalMS)
	s.ConnectNewPeers()
}

func (s *Session) PumpIO() {
	s.acceptIncomingConnections()
	for _, sc := range s.activeServerConnections() {
		if sc == nil {
			continue
		}
		if sc.IsDisconnecting() {
			if !sc.IsDisconnectHandled() {
				sc.OnDisconnect(sc.DisconnectCode())
				sc.MarkDisconnectHandled()
			}
			continue
		}
		if err := sc.FlushOutgoing(); err != nil {
			debugPeerf("server %s flush error: %v", sc.GetIdentifier(), err)
		}
		if err := sc.DoRead(); err != nil {
			debugPeerf("server %s read error: %v", sc.GetIdentifier(), err)
		}
		if err := sc.ProcessIncoming(); err != nil {
			debugPeerf("server %s process error: %v", sc.GetIdentifier(), err)
		}
		if sc.IsDisconnecting() && !sc.IsDisconnectHandled() {
			sc.OnDisconnect(sc.DisconnectCode())
			sc.MarkDisconnectHandled()
		}
	}
	for _, conn := range s.snapshotConnections() {
		if conn == nil {
			continue
		}
		if conn.IsDisconnecting() {
			if !conn.IsDisconnectHandled() {
				conn.OnDisconnect(conn.DisconnectCode())
				conn.MarkDisconnectHandled()
			}
			continue
		}
		if err := conn.FlushOutgoing(); err != nil {
			debugPeerf("peer %s flush error: %v", conn.Endpoint().String(), err)
		}
		if err := conn.DoRead(); err != nil {
			debugPeerf("peer %s read error: %v", conn.Endpoint().String(), err)
		}
		if conn.transferringData {
			conn.ReceivePendingData()
		} else {
			if err := conn.ProcessIncoming(); err != nil {
				debugPeerf("peer %s process error: %v", conn.Endpoint().String(), err)
			}
		}
		if conn.IsDisconnecting() && !conn.IsDisconnectHandled() {
			conn.OnDisconnect(conn.DisconnectCode())
			conn.MarkDisconnectHandled()
		}
	}
}

func (s *Session) acceptIncomingConnections() {
	for {
		select {
		case conn := <-s.incomingConns:
			if conn == nil {
				continue
			}
			s.mu.Lock()
			s.connections = append(s.connections, NewIncomingPeerConnection(s, conn))
			s.mu.Unlock()
		default:
			return
		}
	}
}

func (s *Session) ConnectTo(identifier string, address *net.TCPAddr) error {
	if identifier == "" || address == nil {
		return NewError(InternalError)
	}
	s.mu.Lock()
	s.configuredServers[identifier] = address
	policy := s.ensureServerConnectionPolicy(identifier)
	policy.SetConnectCandidate(identifier, address, CurrentTime())
	if existing := s.serverConnections[identifier]; existing != nil {
		if existing.GetAddress() != nil && existing.GetAddress().String() == address.String() && !existing.IsDisconnecting() {
			s.promotePrimaryServer(existing)
			s.mu.Unlock()
			return nil
		}
		existing.Close(NoError)
	}
	sc := NewServerConnection(identifier, address, s)
	s.serverConnections[identifier] = sc
	s.promotePrimaryServer(sc)
	s.mu.Unlock()
	return sc.Connect()
}

func (s *Session) DisconnectFrom() {
	s.mu.Lock()
	for identifier, policy := range s.serverConnectionPolicy {
		if policy != nil {
			policy.RemoveConnectCandidates()
		}
		delete(s.serverConnectionPolicy, identifier)
	}
	for identifier, sc := range s.serverConnections {
		if sc != nil {
			sc.Close(NoError)
		}
		delete(s.serverConnections, identifier)
	}
	for identifier := range s.configuredServers {
		delete(s.configuredServers, identifier)
	}
	s.serverConnection = nil
	s.clientID = 0
	s.tcpFlags = 0
	s.auxPort = 0
	s.mu.Unlock()
}

func (s *Session) ConnectedServerID() string {
	if s.serverConnection != nil && s.serverConnection.IsHandshakeCompleted() {
		return s.serverConnection.GetIdentifier()
	}
	return ""
}

func (s *Session) ConnectedServerIDs() []string {
	ids := make([]string, 0, len(s.serverConnections))
	for identifier, sc := range s.serverConnections {
		if sc != nil && sc.IsHandshakeCompleted() {
			ids = append(ids, identifier)
		}
	}
	return ids
}

func (s *Session) OnServerConnectionClosed(sc *ServerConnection, ec BaseErrorCode) {
	s.mu.Lock()
	if sc != nil {
		delete(s.serverConnections, sc.GetIdentifier())
		if s.settings.ReconnectToServer && ec.Code() != NoError.Code() {
			policy := s.ensureServerConnectionPolicy(sc.GetIdentifier())
			policy.SetServerConnectionFailed(sc.GetIdentifier(), sc.GetAddress(), CurrentTime())
		}
		if s.serverConnection == sc {
			s.serverConnection = nil
		}
	}
	s.promotePrimaryServer(nil)
	s.refreshPrimaryServerIdentity()
	s.mu.Unlock()
}

func (s *Session) SubmitDiskTask(task TransferCallable) {
	if task == nil {
		return
	}
	entry := &diskTask{callable: task, transfer: task.Transfer()}
	s.diskMu.Lock()
	s.diskTasks = append(s.diskTasks, entry)
	s.diskMu.Unlock()
}

func (s *Session) StartSearch(params SearchParams) (SearchHandle, error) {
	params = normalizeSearchParams(params)
	if params.Query == "" {
		return SearchHandle{}, NewError(IllegalArgument)
	}

	s.searchMu.Lock()
	if s.activeSearch != nil && s.activeSearch.state == SearchStateRunning {
		s.searchMu.Unlock()
		return SearchHandle{}, errors.New("search already in progress")
	}
	s.nextSearchID++
	searchID := s.nextSearchID
	task := newSearchTask(searchID, params, CurrentTime())
	s.activeSearch = task
	s.searchMu.Unlock()

	started := false
	if params.Scope&SearchScopeServer != 0 {
		if s.sendSearchRequest(task, params) {
			started = true
		}
	}
	if params.Scope&SearchScopeDHT != 0 {
		if s.startDHTSearch(task, params) {
			started = true
		}
	}
	if !started {
		task.fail(errors.New("no search backends available"))
		return SearchHandle{session: s, id: searchID}, nil
	}
	return SearchHandle{session: s, id: searchID}, nil
}

func (s *Session) SearchSnapshot() SearchSnapshot {
	s.searchMu.Lock()
	task := s.activeSearch
	s.searchMu.Unlock()
	if task == nil {
		return SearchSnapshot{}
	}
	return task.snapshot()
}

func (s *Session) StopSearch(id uint32) error {
	s.searchMu.Lock()
	task := s.activeSearch
	if task == nil || (id != 0 && task.id != id) {
		s.searchMu.Unlock()
		return nil
	}
	s.activeSearch = nil
	s.searchMu.Unlock()
	task.stop()
	return nil
}

func (s *Session) sendSearchRequest(task *searchTask, params SearchParams) bool {
	searchPacket := serverproto.SearchRequest{
		Query:              params.Query,
		MinSize:            params.MinSize,
		MaxSize:            params.MaxSize,
		MinSources:         params.MinSources,
		MinCompleteSources: params.MinCompleteSources,
		FileType:           params.FileType,
		Extension:          params.Extension,
	}
	sent := false
	for _, sc := range s.activeServerConnections() {
		if sc == nil || !sc.IsHandshakeCompleted() {
			continue
		}
		sc.SendSearchRequest(&searchPacket)
		sent = true
	}
	if sent {
		task.mu.Lock()
		task.serverBusy = true
		task.updatedAt = CurrentTime()
		task.deadlineAt = CurrentTime() + Seconds(int64(s.settings.ServerSearchTimeout))
		task.mu.Unlock()
	}
	return sent
}

func (s *Session) startDHTSearch(task *searchTask, params SearchParams) bool {
	if s.dhtTracker == nil {
		return false
	}
	keyword := pickKadKeyword(params.Query)
	if keyword == "" {
		return false
	}
	task.mu.Lock()
	task.kadKeyword = keyword
	task.dhtBusy = true
	task.updatedAt = CurrentTime()
	task.mu.Unlock()
	keywordHash, err := protocol.HashFromData([]byte(keyword))
	if err != nil {
		task.finishDHT()
		return false
	}

	return s.dhtTracker.SearchKeywords(keywordHash, func(entries []kadproto.SearchEntry) {
		s.searchMu.Lock()
		current := s.activeSearch
		s.searchMu.Unlock()
		if current == nil || current != task {
			return
		}
		for _, entry := range entries {
			result := makeSearchResultFromKAD(entry)
			if !matchesSearchFilters(result, params) {
				continue
			}
			task.mergeResult(result)
		}
		task.finishDHT()
	})
}

func (s *Session) OnServerSearchResult(sc *ServerConnection, result *serverproto.SearchResult) {
	if result == nil {
		return
	}
	s.searchMu.Lock()
	task := s.activeSearch
	s.searchMu.Unlock()
	if task == nil || task.state != SearchStateRunning {
		return
	}
	params := task.params
	for _, entry := range result.Results {
		searchResult := makeSearchResultFromServer(entry)
		if !matchesSearchFilters(searchResult, params) {
			continue
		}
		task.mergeResult(searchResult)
	}
	if result.MoreResults && sc != nil {
		sc.SendSearchMore()
		task.setDeadline(CurrentTime() + Seconds(int64(s.settings.ServerSearchTimeout)))
		return
	}
	task.setDeadline(CurrentTime() + Seconds(2))
}

func (s *Session) tickSearches(now int64) {
	s.searchMu.Lock()
	task := s.activeSearch
	s.searchMu.Unlock()
	if task == nil {
		return
	}
	task.onTick(now)
}

func matchesSearchFilters(result SearchResult, params SearchParams) bool {
	if params.MinSize > 0 && result.FileSize > 0 && result.FileSize < params.MinSize {
		return false
	}
	if params.MaxSize > 0 && result.FileSize > params.MaxSize {
		return false
	}
	if params.MinSources > 0 && result.Sources < params.MinSources {
		return false
	}
	if params.MinCompleteSources > 0 && result.CompleteSources < params.MinCompleteSources {
		return false
	}
	return true
}

func (s *Session) RemoveDiskTask(transfer *Transfer) {
	if transfer == nil {
		return
	}
	s.diskMu.Lock()
	defer s.diskMu.Unlock()
	dst := s.diskTasks[:0]
	for _, task := range s.diskTasks {
		if task.transfer == transfer {
			task.done = true
			continue
		}
		dst = append(dst, task)
	}
	s.diskTasks = dst
}

func (s *Session) processDiskTasks() {
	var nextTask *diskTask
	s.diskMu.Lock()
	for _, task := range s.diskTasks {
		if task == nil || task.done {
			continue
		}
		if !task.running {
			task.running = true
			nextTask = task
		}
		break
	}
	s.diskMu.Unlock()
	if nextTask != nil {
		go func(t *diskTask) {
			res := t.callable.Call()
			s.diskResults <- diskTaskResult{task: t, result: res}
		}(nextTask)
	}
	for {
		select {
		case result := <-s.diskResults:
			if result.task == nil || result.task.done {
				continue
			}
			s.diskMu.Lock()
			result.task.done = true
			dst := s.diskTasks[:0]
			for _, task := range s.diskTasks {
				if task != result.task {
					dst = append(dst, task)
				}
			}
			s.diskTasks = dst
			s.diskMu.Unlock()
			if result.result != nil {
				result.result.OnCompleted()
			}
		default:
			return
		}
	}
}

func (s *Session) diskTaskCount() int {
	s.diskMu.Lock()
	defer s.diskMu.Unlock()
	return len(s.diskTasks)
}

func (s *Session) SetDHTTracker(tracker *DHTTracker) {
	s.dhtTracker = tracker
}

func (s *Session) GetDHTTracker() *DHTTracker {
	return s.dhtTracker
}

func (s *Session) UploadQueue() *UploadQueue {
	s.initUploadQueue()
	return s.uploadQueue
}

func (s *Session) Credits() *PeerCreditManager {
	if s == nil {
		return nil
	}
	if s.credits == nil {
		s.credits = NewPeerCreditManager()
	}
	return s.credits
}

func (s *Session) SetFriendSlot(hash protocol.Hash, enabled bool) {
	if s == nil || hash.Equal(protocol.Invalid) {
		return
	}
	key := hash.String()
	if enabled {
		s.friendSlots[key] = true
		return
	}
	delete(s.friendSlots, key)
}

func (s *Session) IsFriendSlot(hash protocol.Hash) bool {
	if s == nil || hash.Equal(protocol.Invalid) {
		return false
	}
	return s.friendSlots[hash.String()]
}

func (s *Session) friendSlotSnapshot() []protocol.Hash {
	if s == nil {
		return nil
	}
	out := make([]protocol.Hash, 0, len(s.friendSlots))
	for key, enabled := range s.friendSlots {
		if !enabled {
			continue
		}
		hash, err := protocol.HashFromString(key)
		if err != nil {
			continue
		}
		out = append(out, hash)
	}
	return out
}

func (s *Session) applyFriendSlotSnapshot(values []protocol.Hash) {
	if s == nil {
		return
	}
	s.friendSlots = make(map[string]bool, len(values))
	for _, hash := range values {
		if hash.Equal(protocol.Invalid) {
			continue
		}
		s.friendSlots[hash.String()] = true
	}
}

func (s *Session) OnServerIDChange(sc *ServerConnection, clientID, tcpFlags, auxPort int32) {
	if sc == nil {
		return
	}
	var kick []*Transfer
	var offerFiles []serverproto.OfferFile
	s.mu.Lock()
	if s.serverConnection == nil || !s.serverConnection.IsHandshakeCompleted() {
		s.serverConnection = sc
	}
	if s.serverConnection == sc || s.clientID == 0 {
		s.clientID = clientID
		s.tcpFlags = tcpFlags
		s.auxPort = auxPort
	}
	for _, transfer := range s.transfers {
		if transfer == nil || transfer.IsPaused() || transfer.IsAborted() {
			continue
		}
		if transfer.IsFinished() {
			offerFiles = append(offerFiles, serverproto.OfferFile{
				Hash: transfer.GetHash(),
				Name: transfer.FileName(),
				Size: transfer.Size(),
			})
			continue
		}
		kick = append(kick, transfer)
	}
	s.mu.Unlock()
	if len(offerFiles) > 0 {
		packet := serverproto.NewOfferFiles(clientID, s.GetListenPort(), offerFiles)
		sc.SendOfferFiles(&packet)
	}
	for _, transfer := range kick {
		s.RequestSourcesNow(transfer)
	}
}

func (s *Session) PublishTransferToServer(t *Transfer) {
	if s == nil || t == nil {
		return
	}
	s.mu.Lock()
	sc := s.serverConnection
	clientID := s.clientID
	s.mu.Unlock()
	if sc == nil || !sc.IsHandshakeCompleted() || clientID == 0 || t.IsPaused() || t.IsAborted() || !t.IsFinished() {
		return
	}
	packet := serverproto.NewOfferFiles(clientID, s.GetListenPort(), []serverproto.OfferFile{{
		Hash: t.GetHash(),
		Name: t.FileName(),
		Size: t.Size(),
	}})
	sc.SendOfferFiles(&packet)
}

func (s *Session) ensureServerConnectionPolicy(identifier string) *ServerConnectionPolicy {
	if identifier == "" {
		return nil
	}
	if policy := s.serverConnectionPolicy[identifier]; policy != nil {
		return policy
	}
	policy := NewServerConnectionPolicy(5, 5)
	s.serverConnectionPolicy[identifier] = &policy
	return &policy
}

func (s *Session) connectConfiguredServers(currentSessionTime int64) {
	s.mu.Lock()
	configured := make(map[string]*net.TCPAddr, len(s.configuredServers))
	for identifier, address := range s.configuredServers {
		configured[identifier] = address
	}
	s.mu.Unlock()
	for identifier, address := range configured {
		if identifier == "" || address == nil {
			continue
		}
		s.mu.Lock()
		existing := s.serverConnections[identifier]
		s.mu.Unlock()
		if existing != nil && !existing.IsDisconnecting() {
			continue
		}
		s.mu.Lock()
		policy := s.ensureServerConnectionPolicy(identifier)
		candidate := policy.GetConnectCandidate(currentSessionTime)
		s.mu.Unlock()
		if candidate == nil || candidate.Identifier == "" || candidate.Address == nil {
			continue
		}
		sc := NewServerConnection(candidate.Identifier, candidate.Address, s)
		s.mu.Lock()
		s.serverConnections[identifier] = sc
		s.promotePrimaryServer(sc)
		s.mu.Unlock()
		_ = sc.Connect()
	}
}

func (s *Session) activeServerConnections() []*ServerConnection {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]*ServerConnection, 0, len(s.serverConnections)+1)
	seen := make(map[*ServerConnection]struct{}, len(s.serverConnections)+1)
	if s.serverConnection != nil {
		res = append(res, s.serverConnection)
		seen[s.serverConnection] = struct{}{}
	}
	for _, sc := range s.serverConnections {
		if sc == nil {
			continue
		}
		if _, ok := seen[sc]; ok {
			continue
		}
		res = append(res, sc)
		seen[sc] = struct{}{}
	}
	return res
}

func (s *Session) snapshotTransfers() []*Transfer {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Transfer, 0, len(s.transfers))
	for _, t := range s.transfers {
		out = append(out, t)
	}
	return out
}

func (s *Session) snapshotConnections() []*PeerConnection {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*PeerConnection, len(s.connections))
	copy(out, s.connections)
	return out
}

func (s *Session) promotePrimaryServer(sc *ServerConnection) {
	if sc != nil {
		if s.serverConnection == nil || s.serverConnection.IsDisconnecting() || !s.serverConnection.IsHandshakeCompleted() {
			s.serverConnection = sc
		}
		return
	}
	if s.serverConnection != nil && !s.serverConnection.IsDisconnecting() {
		return
	}
	for _, candidate := range s.serverConnections {
		if candidate != nil && !candidate.IsDisconnecting() {
			s.serverConnection = candidate
			return
		}
	}
	s.serverConnection = nil
}

func (s *Session) refreshPrimaryServerIdentity() {
	if s.serverConnection != nil && s.serverConnection.IsHandshakeCompleted() {
		s.clientID = s.serverConnection.ClientID()
		s.tcpFlags = s.serverConnection.TCPFlags()
		s.auxPort = s.serverConnection.AuxPort()
		return
	}
	for _, sc := range s.serverConnections {
		if sc != nil && sc.IsHandshakeCompleted() {
			s.serverConnection = sc
			s.clientID = sc.ClientID()
			s.tcpFlags = sc.TCPFlags()
			s.auxPort = sc.AuxPort()
			return
		}
	}
	s.clientID = 0
	s.tcpFlags = 0
	s.auxPort = 0
}
