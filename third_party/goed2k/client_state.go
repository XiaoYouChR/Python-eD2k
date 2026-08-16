package goed2k

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/monkeyWie/goed2k/data"
	"github.com/monkeyWie/goed2k/disk"
	"github.com/monkeyWie/goed2k/protocol"
)

const clientStateVersion = 2

type ClientStateStore interface {
	Load() (*ClientState, error)
	Save(state *ClientState) error
}

type ClientState struct {
	Version       int                   `json:"version"`
	ServerAddress string                `json:"server_address,omitempty"`
	Transfers     []ClientTransferState `json:"transfers"`
	Credits       []ClientCreditState   `json:"credits,omitempty"`
	FriendSlots   []protocol.Hash       `json:"friend_slots,omitempty"`
	DHT           *ClientDHTState       `json:"dht,omitempty"`
}

type ClientTransferState struct {
	Hash       protocol.Hash                `json:"hash"`
	Size       int64                        `json:"size"`
	CreateTime int64                        `json:"create_time"`
	TargetPath string                       `json:"target_path"`
	Paused     bool                         `json:"paused"`
	UploadPrio UploadPriority               `json:"upload_prio,omitempty"`
	ResumeData *protocol.TransferResumeData `json:"resume_data,omitempty"`
}

type ClientDHTState struct {
	SelfID              protocol.Hash        `json:"self_id,omitempty"`
	Firewalled          bool                 `json:"firewalled"`
	LastBootstrap       int64                `json:"last_bootstrap,omitempty"`
	LastRefresh         int64                `json:"last_refresh,omitempty"`
	LastFirewalledCheck int64                `json:"last_firewalled_check,omitempty"`
	StoragePoint        string               `json:"storage_point,omitempty"`
	Nodes               []ClientDHTNodeState `json:"nodes,omitempty"`
	RouterNodes         []string             `json:"router_nodes,omitempty"`
}

type ClientDHTNodeState struct {
	ID        protocol.Hash `json:"id,omitempty"`
	Addr      string        `json:"addr"`
	TCPPort   uint16        `json:"tcp_port,omitempty"`
	Version   byte          `json:"version,omitempty"`
	Seed      bool          `json:"seed,omitempty"`
	HelloSent bool          `json:"hello_sent,omitempty"`
	Pinged    bool          `json:"pinged,omitempty"`
	FailCount int           `json:"fail_count,omitempty"`
	FirstSeen int64         `json:"first_seen,omitempty"`
	LastSeen  int64         `json:"last_seen,omitempty"`
}

type FileClientStateStore struct {
	path string
}

func NewFileClientStateStore(path string) *FileClientStateStore {
	return &FileClientStateStore{path: path}
}

func (s *FileClientStateStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *FileClientStateStore) Load() (*ClientState, error) {
	if s == nil || s.path == "" {
		return nil, errors.New("state path is empty")
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var state ClientState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	if state.Version == 0 {
		state.Version = clientStateVersion
	}
	return &state, nil
}

func (s *FileClientStateStore) Save(state *ClientState) error {
	if s == nil || s.path == "" {
		return errors.New("state path is empty")
	}
	if state == nil {
		state = &ClientState{Version: clientStateVersion}
	}
	if state.Version == 0 {
		state.Version = clientStateVersion
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func (c *Client) SetStateStore(store ClientStateStore) {
	c.stateStore = store
}

func (c *Client) StateStore() ClientStateStore {
	return c.stateStore
}

func (c *Client) SetStatePath(path string) {
	if path == "" {
		c.stateStore = nil
		return
	}
	c.stateStore = NewFileClientStateStore(path)
}

func (c *Client) StatePath() string {
	fileStore, ok := c.stateStore.(*FileClientStateStore)
	if !ok || fileStore == nil {
		return ""
	}
	return fileStore.Path()
}

func (c *Client) SaveState(path string) error {
	if path != "" {
		c.SetStatePath(path)
	}
	if c.stateStore == nil {
		return errors.New("state store is not configured")
	}
	state, err := c.snapshotState()
	if err != nil {
		return err
	}
	return c.stateStore.Save(state)
}

func (c *Client) LoadState(path string) error {
	if path != "" {
		c.SetStatePath(path)
	}
	if c.stateStore == nil {
		return errors.New("state store is not configured")
	}
	state, err := c.stateStore.Load()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return c.applyState(state)
}

func (c *Client) snapshotState() (*ClientState, error) {
	handles := c.session.GetTransfers()
	sort.Slice(handles, func(i, j int) bool {
		return handles[i].GetHash().String() < handles[j].GetHash().String()
	})
	state := &ClientState{
		Version:       clientStateVersion,
		ServerAddress: c.serverAddr,
		Transfers:     make([]ClientTransferState, 0, len(handles)),
		Credits:       c.session.Credits().Snapshot(),
		FriendSlots:   c.session.friendSlotSnapshot(),
	}
	if tracker := c.GetDHTTracker(); tracker != nil {
		state.DHT = tracker.SnapshotState()
	}
	for _, handle := range handles {
		if !handle.IsValid() {
			continue
		}
		path := handle.GetFilePath()
		if path == "" {
			continue
		}
		state.Transfers = append(state.Transfers, ClientTransferState{
			Hash:       handle.GetHash(),
			Size:       handle.GetSize(),
			CreateTime: handle.GetCreateTime(),
			TargetPath: path,
			Paused:     handle.IsPaused(),
			UploadPrio: handle.transfer.UploadPriority(),
			ResumeData: handle.GetResumeData(),
		})
	}
	return state, nil
}

func (c *Client) applyState(state *ClientState) error {
	if state == nil {
		return nil
	}
	if state.Version != 0 && state.Version != 1 && state.Version != clientStateVersion {
		return errors.New("unsupported state version")
	}
	c.serverAddr = state.ServerAddress
	c.session.Credits().ApplySnapshot(state.Credits)
	c.session.applyFriendSlotSnapshot(state.FriendSlots)
	if state.DHT != nil {
		if err := c.EnableDHT().ApplyState(state.DHT); err != nil {
			return err
		}
	}
	for _, record := range state.Transfers {
		if record.TargetPath == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(record.TargetPath), 0o755); err != nil {
			return err
		}
		atp := AddTransferParams{
			Hash:       record.Hash,
			CreateTime: record.CreateTime,
			Size:       record.Size,
			FilePath:   record.TargetPath,
			Paused:     record.Paused,
			ResumeData: cloneResumeData(record.ResumeData),
			Handler:    disk.NewDesktopFileHandler(record.TargetPath),
		}
		handle, err := c.session.AddTransferParams(atp)
		if err != nil {
			return err
		}
		if handle.IsValid() {
			handle.transfer.SetUploadPriority(record.UploadPrio)
		}
	}
	return nil
}

func cloneResumeData(src *protocol.TransferResumeData) *protocol.TransferResumeData {
	if src == nil {
		return nil
	}
	dst := &protocol.TransferResumeData{
		Hashes:           append([]protocol.Hash(nil), src.Hashes...),
		Pieces:           protocol.NewBitField(src.Pieces.Len()),
		DownloadedBlocks: append([]data.PieceBlock(nil), src.DownloadedBlocks...),
		Peers:            append([]protocol.Endpoint(nil), src.Peers...),
	}
	for i := 0; i < src.Pieces.Len(); i++ {
		if src.Pieces.GetBit(i) {
			dst.Pieces.SetBit(i)
		}
	}
	return dst
}
