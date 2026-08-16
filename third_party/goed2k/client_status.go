package goed2k

import (
	"path/filepath"
	"sort"

	"github.com/monkeyWie/goed2k/protocol"
)

type ServerSnapshot struct {
	Identifier                   string
	Address                      string
	Configured                   bool
	Connected                    bool
	HandshakeCompleted           bool
	Primary                      bool
	Disconnecting                bool
	ClientID                     int32
	TCPFlags                     int32
	AuxPort                      int32
	MillisecondsSinceLastReceive int64
	DownloadRate                 int
	UploadRate                   int
}

func (s ServerSnapshot) IDClass() string {
	if s.ClientID == 0 {
		return "UNKNOWN"
	}
	if IsLowID(s.ClientID) {
		return "LOW_ID"
	}
	return "HIGH_ID"
}

type TransferSnapshot struct {
	Hash        protocol.Hash
	FileName    string
	FilePath    string
	CreateTime  int64
	Size        int64
	ActivePeers int
	Status      TransferStatus
	Peers       []PeerInfo
	Pieces      []PieceSnapshot
}

func (t TransferSnapshot) ED2KLink() string {
	if t.FileName == "" || t.Size <= 0 || t.Hash.Equal(protocol.Invalid) {
		return ""
	}
	return FormatLink(t.FileName, t.Size, t.Hash)
}

type ClientPeerSnapshot struct {
	TransferHash protocol.Hash
	FileName     string
	FilePath     string
	Peer         PeerInfo
}

type ClientStatus struct {
	Servers       []ServerSnapshot
	Peers         []ClientPeerSnapshot
	Transfers     []TransferSnapshot
	TotalDone     int64
	TotalReceived int64
	TotalWanted   int64
	Upload        int64
	DownloadRate  int
	UploadRate    int
}

func (c *Client) Status() ClientStatus {
	transfers := c.TransferSnapshots()
	peers := make([]ClientPeerSnapshot, 0)
	status := ClientStatus{
		Servers:   c.ServerStatuses(),
		Transfers: transfers,
	}
	for _, transfer := range transfers {
		status.TotalDone += transfer.Status.TotalDone
		status.TotalReceived += transfer.Status.TotalReceived
		status.TotalWanted += transfer.Status.TotalWanted
		status.Upload += transfer.Status.Upload
		status.DownloadRate += transfer.Status.DownloadRate
		status.UploadRate += transfer.Status.UploadRate
		for _, peer := range transfer.Peers {
			peers = append(peers, ClientPeerSnapshot{
				TransferHash: transfer.Hash,
				FileName:     transfer.FileName,
				FilePath:     transfer.FilePath,
				Peer:         peer,
			})
		}
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].TransferHash.Compare(peers[j].TransferHash) != 0 {
			return peers[i].TransferHash.Compare(peers[j].TransferHash) < 0
		}
		return peers[i].Peer.Endpoint.String() < peers[j].Peer.Endpoint.String()
	})
	status.Peers = peers
	return status
}

func (c *Client) ServerStatuses() []ServerSnapshot {
	return c.session.ServerSnapshots()
}

func (c *Client) TransferSnapshots() []TransferSnapshot {
	handles := c.Transfers()
	snapshots := make([]TransferSnapshot, 0, len(handles))
	for _, handle := range handles {
		if !handle.IsValid() {
			continue
		}
		filePath := handle.GetFilePath()
		fileName := filepath.Base(filePath)
		if fileName == "." || fileName == "" {
			fileName = handle.GetHash().String()
		}
		snapshots = append(snapshots, TransferSnapshot{
			Hash:        handle.GetHash(),
			FileName:    fileName,
			FilePath:    filePath,
			CreateTime:  handle.GetCreateTime(),
			Size:        handle.GetSize(),
			ActivePeers: handle.ActiveConnections(),
			Status:      handle.GetStatus(),
			Peers:       handle.GetPeersInfo(),
			Pieces:      handle.PieceSnapshots(),
		})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].CreateTime != snapshots[j].CreateTime {
			return snapshots[i].CreateTime < snapshots[j].CreateTime
		}
		return snapshots[i].Hash.Compare(snapshots[j].Hash) < 0
	})
	return snapshots
}

func (c *Client) PeerStatuses() []ClientPeerSnapshot {
	return c.Status().Peers
}

func (s *Session) ServerSnapshots() []ServerSnapshot {
	s.mu.Lock()
	configured := make(map[string]string, len(s.configuredServers))
	for identifier, address := range s.configuredServers {
		if address == nil {
			configured[identifier] = ""
			continue
		}
		configured[identifier] = address.String()
	}
	servers := make(map[string]*ServerConnection, len(s.serverConnections))
	for identifier, connection := range s.serverConnections {
		servers[identifier] = connection
	}
	primary := s.serverConnection
	s.mu.Unlock()

	identifiers := make([]string, 0, len(configured)+len(servers))
	seen := make(map[string]struct{}, len(configured)+len(servers))
	for identifier := range configured {
		seen[identifier] = struct{}{}
		identifiers = append(identifiers, identifier)
	}
	for identifier := range servers {
		if _, ok := seen[identifier]; ok {
			continue
		}
		seen[identifier] = struct{}{}
		identifiers = append(identifiers, identifier)
	}
	sort.Strings(identifiers)

	snapshots := make([]ServerSnapshot, 0, len(identifiers))
	for _, identifier := range identifiers {
		sc := servers[identifier]
		snapshot := ServerSnapshot{
			Identifier: identifier,
			Configured: false,
		}
		if address, ok := configured[identifier]; ok {
			snapshot.Address = address
			snapshot.Configured = true
		}
		if sc != nil {
			stats := sc.Statistics()
			if snapshot.Address == "" && sc.GetAddress() != nil {
				snapshot.Address = sc.GetAddress().String()
			}
			snapshot.Connected = !sc.IsDisconnecting()
			snapshot.HandshakeCompleted = sc.IsHandshakeCompleted()
			snapshot.Primary = sc == primary
			snapshot.Disconnecting = sc.IsDisconnecting()
			snapshot.ClientID = sc.ClientID()
			snapshot.TCPFlags = sc.TCPFlags()
			snapshot.AuxPort = sc.AuxPort()
			snapshot.MillisecondsSinceLastReceive = sc.MillisecondsSinceLastReceive()
			snapshot.DownloadRate = int(stats.DownloadRate())
			snapshot.UploadRate = int(stats.UploadRate())
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}
