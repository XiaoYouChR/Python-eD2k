package goed2k

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/monkeyWie/goed2k/disk"
	"github.com/monkeyWie/goed2k/internal/logx"
	"github.com/monkeyWie/goed2k/protocol"
	kadproto "github.com/monkeyWie/goed2k/protocol/kad"
	serverproto "github.com/monkeyWie/goed2k/protocol/server"
)

var ErrClientStopped = errors.New("client stopped")

var remoteResourceHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after %d redirects", len(via))
		}
		return nil
	},
}

type Client struct {
	session           *Session
	tickInterval      time.Duration
	statusInterval    time.Duration
	autoSaveTick      time.Duration
	stopCh            chan struct{}
	doneCh            chan struct{}
	startOnce         sync.Once
	closeOnce         sync.Once
	started           bool
	serverAddr        string
	stateStore        ClientStateStore
	listenersMu       sync.Mutex
	listeners         map[int]chan ClientStatusEvent
	nextListenerID    int
	progressMu        sync.Mutex
	progressListeners map[int]chan TransferProgressEvent
	nextProgressID    int
	lastProgress      map[protocol.Hash]TransferProgressSnapshot
}

func NewClient(settings Settings) *Client {
	if settings.Logger != nil {
		logx.SetLogger(settings.Logger)
	}
	return &Client{
		session:        NewSession(settings),
		tickInterval:   100 * time.Millisecond,
		statusInterval: time.Second,
		autoSaveTick:   5 * time.Second,
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
}

func (c *Client) Session() *Session {
	return c.session
}

func (c *Client) SetDHTTracker(tracker *DHTTracker) {
	c.session.SetDHTTracker(tracker)
}

func (c *Client) GetDHTTracker() *DHTTracker {
	return c.session.GetDHTTracker()
}

func (c *Client) EnableDHT() *DHTTracker {
	if tracker := c.GetDHTTracker(); tracker != nil {
		return tracker
	}
	timeout := time.Duration(c.session.settings.DHTSearchTimeout) * time.Second
	tracker := NewDHTTracker(c.session.settings.UDPPort, timeout)
	c.SetDHTTracker(tracker)
	return tracker
}

func (c *Client) LoadDHTNodesDat(path ...string) error {
	if len(path) == 0 {
		return errors.New("nodes.dat path is empty")
	}
	var errs []error
	loaded := false
	for _, source := range path {
		for _, part := range strings.Split(source, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			nodes, err := c.loadDHTNodesDat(part)
			if err != nil {
				logx.Debug("load nodes.dat failed", "source", part, "err", err)
				errs = append(errs, fmt.Errorf("%s: %w", part, err))
				continue
			}
			if err := c.EnableDHT().ApplyNodesDat(nodes); err != nil {
				logx.Debug("apply nodes.dat failed", "source", part, "err", err)
				errs = append(errs, fmt.Errorf("%s: %w", part, err))
				continue
			}
			logx.Debug("nodes.dat loaded", "source", part, "entries", len(nodes.Contacts))
			loaded = true
		}
	}
	if loaded {
		return nil
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return errors.New("nodes.dat path is empty")
}

func (c *Client) AddDHTBootstrapNodes(nodes ...string) error {
	tracker := c.EnableDHT()
	for _, item := range nodes {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			addr, err := net.ResolveUDPAddr("udp", part)
			if err != nil {
				return err
			}
			tracker.AddNode(addr)
		}
	}
	return nil
}

func (c *Client) PublishDHTSource(hash protocol.Hash, endpoint protocol.Endpoint, size int64) bool {
	return c.EnableDHT().PublishSource(hash, endpoint, size)
}

func (c *Client) PublishDHTKeyword(keywordHash protocol.Hash, entries ...kadproto.SearchEntry) bool {
	return c.EnableDHT().PublishKeyword(keywordHash, entries...)
}

func (c *Client) PublishDHTNotes(fileHash protocol.Hash, entries ...kadproto.SearchEntry) bool {
	return c.EnableDHT().PublishNotes(fileHash, entries...)
}

func (c *Client) SearchDHTKeywords(keywordHash protocol.Hash, cb func([]kadproto.SearchEntry)) bool {
	return c.EnableDHT().SearchKeywords(keywordHash, cb)
}

func (c *Client) StartSearch(params SearchParams) (SearchHandle, error) {
	return c.session.StartSearch(params)
}

func (c *Client) StopSearch() error {
	return c.session.StopSearch(0)
}

func (c *Client) SearchSnapshot() SearchSnapshot {
	return c.session.SearchSnapshot()
}

func (c *Client) SetDHTStoragePoint(address string) error {
	if strings.TrimSpace(address) == "" {
		c.EnableDHT().SetStoragePoint(nil)
		return nil
	}
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}
	c.EnableDHT().SetStoragePoint(addr)
	return nil
}

func (c *Client) DHTStatus() DHTStatus {
	if tracker := c.GetDHTTracker(); tracker != nil {
		return tracker.Status()
	}
	return DHTStatus{}
}

func (c *Client) Start() error {
	var err error
	c.startOnce.Do(func() {
		if c.session.settings.EnableDHT && c.GetDHTTracker() == nil {
			c.EnableDHT()
		}
		err = c.session.Listen()
		if err != nil {
			return
		}
		if tracker := c.GetDHTTracker(); tracker != nil {
			if startErr := tracker.Start(); startErr != nil {
				err = startErr
				c.session.CloseListener()
				return
			}
			c.session.SyncDHTListenPort()
			if c.session.settings.EnableUPnP {
				c.session.RefreshUPnPMapping()
			}
		}
		c.started = true
		go c.loop()
		go c.statusLoop()
		c.emitStatusUpdate()
		c.emitTransferProgressUpdate(true)
	})
	return err
}

func (c *Client) Wait() error {
	if !c.started {
		return nil
	}
	ticker := time.NewTicker(c.tickInterval)
	defer ticker.Stop()
	for {
		handles := c.session.GetTransfers()
		if len(handles) == 0 {
			return nil
		}
		allFinished := true
		for _, handle := range handles {
			if !handle.IsFinished() {
				allFinished = false
				break
			}
		}
		if allFinished {
			return nil
		}
		select {
		case <-c.doneCh:
			return ErrClientStopped
		case <-ticker.C:
		}
	}
}

func (c *Client) Stop() error {
	var err error
	c.closeOnce.Do(func() {
		if c.started {
			close(c.stopCh)
			if tracker := c.GetDHTTracker(); tracker != nil {
				tracker.Close()
			}
			c.session.DisconnectFrom()
			c.session.CloseListener()
			select {
			case <-c.doneCh:
			case <-time.After(2 * time.Second):
			}
		} else {
			if tracker := c.GetDHTTracker(); tracker != nil {
				tracker.Close()
			}
			c.session.DisconnectFrom()
			c.session.CloseListener()
		}
		if c.stateStore != nil {
			err = c.SaveState("")
		}
	})
	return err
}

func (c *Client) Close() {
	_ = c.Stop()
}

func (c *Client) Connect(serverAddr string) error {
	logx.Debug("connect server", "server", serverAddr)
	c.serverAddr = serverAddr
	addr, err := net.ResolveTCPAddr("tcp", serverAddr)
	if err != nil {
		return err
	}
	return c.session.ConnectTo(serverAddr, addr)
}

func (c *Client) ConnectServers(serverAddrs ...string) error {
	normalized := make([]string, 0, len(serverAddrs))
	seen := make(map[string]struct{}, len(serverAddrs))
	for _, item := range serverAddrs {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			normalized = append(normalized, part)
		}
	}
	if len(normalized) == 0 {
		return errors.New("no server address provided")
	}
	c.serverAddr = strings.Join(normalized, ",")
	for _, serverAddr := range normalized {
		addr, err := net.ResolveTCPAddr("tcp", serverAddr)
		if err != nil {
			return err
		}
		if err := c.session.ConnectTo(serverAddr, addr); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) LoadServerMet(path string) ([]serverproto.ServerMetEntry, error) {
	met, err := c.loadServerMet(path)
	if err != nil {
		return nil, err
	}
	logx.Debug("server.met loaded", "source", path, "servers", len(met.Servers))
	entries := make([]serverproto.ServerMetEntry, len(met.Servers))
	copy(entries, met.Servers)
	return entries, nil
}

func (c *Client) ConnectServerMet(path string) error {
	met, err := c.loadServerMet(path)
	if err != nil {
		return err
	}
	return c.ConnectServers(met.Addresses()...)
}

func (c *Client) ConnectServerLink(linkValue string) error {
	link, err := ParseEMuleLink(linkValue)
	if err != nil {
		return err
	}
	switch link.Type {
	case LinkServer:
		return c.ConnectServers(net.JoinHostPort(link.StringValue, fmt.Sprintf("%d", link.NumberValue)))
	case LinkServers:
		return c.ConnectServerMet(link.StringValue)
	default:
		return errors.New("unsupported server link type")
	}
}

func (c *Client) SetAutoSaveInterval(interval time.Duration) {
	c.autoSaveTick = interval
}

func (c *Client) ServerAddress() string {
	return c.serverAddr
}

func (c *Client) ConnectSavedServer() error {
	if c.serverAddr == "" {
		return errors.New("saved server address is empty")
	}
	return c.ConnectServers(c.serverAddr)
}

func (c *Client) AddLink(linkValue, outputDir string) (TransferHandle, string, error) {
	link, err := ParseEMuleLink(linkValue)
	if err != nil {
		return TransferHandle{}, "", err
	}
	if link.Type != LinkFile {
		return TransferHandle{}, "", errors.New("unsupported link type")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return TransferHandle{}, "", err
	}
	targetPath := filepath.Join(outputDir, link.StringValue)
	handler := disk.NewDesktopFileHandler(targetPath)
	handle, err := c.session.AddTransferWithHandler(link.Hash, link.NumberValue, handler)
	if err == nil {
		logx.Debug("transfer added", "file", link.StringValue, "hash", link.Hash.String(), "size", link.NumberValue, "path", targetPath)
		if handle.transfer != nil {
			requested := c.session.RequestSourcesNow(handle.transfer)
			logx.Debug("initial source discovery requested", "hash", link.Hash.String(), "requested", requested)
		}
		_ = c.saveStateIfConfigured()
		c.emitStatusUpdate()
		c.emitTransferProgressUpdate(true)
	}
	return handle, targetPath, err
}

func (c *Client) loadServerMet(source string) (*serverproto.ServerMet, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, errors.New("server.met source is empty")
	}
	if strings.HasPrefix(strings.ToLower(source), "ed2k://") {
		link, err := ParseEMuleLink(source)
		if err != nil {
			return nil, err
		}
		if link.Type != LinkServers {
			return nil, errors.New("ed2k link is not a serverlist link")
		}
		source = link.StringValue
	}

	parsedURL, err := url.Parse(source)
	if err == nil && parsedURL.Scheme != "" {
		switch parsedURL.Scheme {
		case "file":
			return serverproto.LoadServerMet(parsedURL.Path)
		case "http", "https":
			data, err := fetchRemoteResource(source)
			if err != nil {
				return nil, err
			}
			return serverproto.ParseServerMet(data)
		}
	}

	return serverproto.LoadServerMet(source)
}

func (c *Client) loadDHTNodesDat(source string) (*kadproto.NodesDat, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, errors.New("nodes.dat source is empty")
	}
	parsedURL, err := url.Parse(source)
	if err == nil && parsedURL.Scheme != "" {
		switch parsedURL.Scheme {
		case "file":
			return kadproto.LoadNodesDat(parsedURL.Path)
		case "http", "https":
			data, err := fetchRemoteResource(source)
			if err != nil {
				return nil, err
			}
			return kadproto.ParseNodesDat(data)
		}
	}
	return kadproto.LoadNodesDat(source)
}

func fetchRemoteResource(source string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "goed2k/1.0")
	resp, err := remoteResourceHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	logx.Debug("fetched remote resource", "source", source, "status", resp.StatusCode, "final_url", resp.Request.URL.String())
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download failed: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Client) AddTransfer(atp AddTransferParams) (TransferHandle, error) {
	handle, err := c.session.AddTransferParams(atp)
	if err == nil {
		_ = c.saveStateIfConfigured()
		c.emitStatusUpdate()
		c.emitTransferProgressUpdate(true)
	}
	return handle, err
}

func (c *Client) FindTransfer(hash protocol.Hash) TransferHandle {
	return c.session.FindTransfer(hash)
}

func (c *Client) Transfers() []TransferHandle {
	return c.session.GetTransfers()
}

func (c *Client) PauseTransfer(hash protocol.Hash) error {
	handle := c.FindTransfer(hash)
	if !handle.IsValid() {
		return errors.New("transfer not found")
	}
	handle.Pause()
	if err := c.saveStateIfConfigured(); err != nil {
		return err
	}
	c.emitStatusUpdate()
	c.emitTransferProgressUpdate(true)
	return nil
}

func (c *Client) ResumeTransfer(hash protocol.Hash) error {
	handle := c.FindTransfer(hash)
	if !handle.IsValid() {
		return errors.New("transfer not found")
	}
	handle.Resume()
	if err := c.saveStateIfConfigured(); err != nil {
		return err
	}
	c.emitStatusUpdate()
	c.emitTransferProgressUpdate(true)
	return nil
}

func (c *Client) RemoveTransfer(hash protocol.Hash, deleteFile bool) error {
	if err := c.session.RemoveTransfer(hash, deleteFile); err != nil {
		return err
	}
	if err := c.saveStateIfConfigured(); err != nil {
		return err
	}
	c.emitStatusUpdate()
	c.emitTransferProgressUpdate(true)
	return nil
}

func (c *Client) SetTransferUploadPriority(hash protocol.Hash, priority UploadPriority) error {
	handle := c.FindTransfer(hash)
	if !handle.IsValid() {
		return errors.New("transfer not found")
	}
	handle.transfer.SetUploadPriority(priority)
	return c.saveStateIfConfigured()
}

func (c *Client) SuspendUpload(hash protocol.Hash, terminate bool) uint16 {
	removed := c.session.UploadQueue().SuspendUpload(hash, terminate)
	_ = c.saveStateIfConfigured()
	return removed
}

func (c *Client) ResumeUpload(hash protocol.Hash) {
	c.session.UploadQueue().ResumeUpload(hash)
	_ = c.saveStateIfConfigured()
}

func (c *Client) SetFriendSlot(hash protocol.Hash, enabled bool) {
	c.session.SetFriendSlot(hash, enabled)
	_ = c.saveStateIfConfigured()
}

func (c *Client) loop() {
	defer close(c.doneCh)
	ticker := time.NewTicker(c.tickInterval)
	defer ticker.Stop()
	lastTick := time.Now()
	lastSave := lastTick
	for {
		select {
		case now := <-ticker.C:
			elapsed := now.Sub(lastTick)
			lastTick = now
			UpdateCachedTime()
			c.session.SecondTick(CurrentTime(), elapsed.Milliseconds())
			if c.stateStore != nil && c.autoSaveTick > 0 && now.Sub(lastSave) >= c.autoSaveTick {
				_ = c.SaveState("")
				lastSave = now
			}
		case <-c.stopCh:
			return
		}
	}
}

func (c *Client) statusLoop() {
	ticker := time.NewTicker(c.statusInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.emitStatusUpdate()
			c.emitTransferProgressUpdate(false)
		case <-c.stopCh:
			return
		}
	}
}

func (c *Client) saveStateIfConfigured() error {
	if c.stateStore == nil {
		return nil
	}
	return c.SaveState("")
}
