package goed2k

import (
	"bytes"
	"compress/zlib"
	"io"
	"math"
	"net"
	"path/filepath"

	"github.com/monkeyWie/goed2k/data"
	"github.com/monkeyWie/goed2k/internal/logx"
	"github.com/monkeyWie/goed2k/protocol"
	clientproto "github.com/monkeyWie/goed2k/protocol/client"
)

func debugPeerf(format string, args ...any) {
	logx.Debug(formatMessage(format, args...))
}

const MaxOutgoingBufferSize = 102*2 + 8

type PeerSpeed int

const (
	PeerSpeedSlow PeerSpeed = iota
	PeerSpeedMedium
	PeerSpeedFast
)

type UploadState int

const (
	UploadStateNone UploadState = iota
	UploadStateOnQueue
	UploadStateUploading
	UploadStateConnecting
)

type MiscOptions struct {
	AICHVersion         int
	UnicodeSupport      int
	UDPVer              int
	DataCompVer         int
	SupportSecIdent     int
	SourceExchange1Ver  int
	ExtendedRequestsVer int
	AcceptCommentVer    int
	NoViewSharedFiles   int
	MultiPacket         int
	SupportsPreview     int
}

func (m MiscOptions) IntValue() int {
	return (m.AICHVersion << ((4 * 7) + 1)) |
		(m.UnicodeSupport << 4 * 7) |
		(m.UDPVer << 4 * 6) |
		(m.DataCompVer << 4 * 5) |
		(m.SupportSecIdent << 4 * 4) |
		(m.SourceExchange1Ver << 4 * 3) |
		(m.ExtendedRequestsVer << 4 * 2) |
		(m.AcceptCommentVer << 4) |
		(m.NoViewSharedFiles << 2) |
		(m.MultiPacket << 1) |
		m.SupportsPreview
}

func (m *MiscOptions) Assign(value int) {
	m.AICHVersion = (value >> (4*7 + 1)) & 0x07
	m.UnicodeSupport = (value >> 4 * 7) & 0x01
	m.UDPVer = (value >> 4 * 6) & 0x0f
	m.DataCompVer = (value >> 4 * 5) & 0x0f
	m.SupportSecIdent = (value >> 4 * 4) & 0x0f
	m.SourceExchange1Ver = (value >> 4 * 3) & 0x0f
	m.ExtendedRequestsVer = (value >> 4 * 2) & 0x0f
	m.AcceptCommentVer = (value >> 4) & 0x0f
	m.NoViewSharedFiles = (value >> 2) & 0x01
	m.MultiPacket = (value >> 1) & 0x01
	m.SupportsPreview = value & 0x01
}

type MiscOptions2 struct {
	Value int
}

const (
	largeFileOffset = 4
	multipOffset    = 5
	srcExtOffset    = 10
	captchaOffset   = 11
)

func (m MiscOptions2) SupportCaptcha() bool        { return ((m.Value >> captchaOffset) & 0x01) == 1 }
func (m MiscOptions2) SupportSourceExt2() bool     { return ((m.Value >> srcExtOffset) & 0x01) == 1 }
func (m MiscOptions2) SupportExtMultipacket() bool { return ((m.Value >> multipOffset) & 0x01) == 1 }
func (m MiscOptions2) SupportLargeFiles() bool     { return ((m.Value >> largeFileOffset) & 0x01) == 0 }
func (m *MiscOptions2) SetCaptcha()                { m.Value |= 1 << captchaOffset }
func (m *MiscOptions2) SetSourceExt2()             { m.Value |= 1 << srcExtOffset }
func (m *MiscOptions2) SetExtMultipacket()         { m.Value |= 1 << multipOffset }
func (m *MiscOptions2) SetLargeFiles()             { m.Value |= 1 << largeFileOffset }
func (m *MiscOptions2) Assign(value int)           { m.Value = value }

type RemotePeerInfo struct {
	Point      protocol.Endpoint
	ModName    string
	Version    int
	ModVersion string
	ModNumber  int
	Misc1      MiscOptions
	Misc2      MiscOptions2
}

type PendingBlock struct {
	Block      data.PieceBlock
	DataSize   int64
	CreateTime int64
	Received   int64
	Buffer     []byte
}

type RequestedUploadBlock struct {
	Begin       int64
	End         int64
	Transferred int64
}

func NewPendingBlock(block data.PieceBlock, totalSize int64) PendingBlock {
	return PendingBlock{
		Block:      block,
		DataSize:   int64(block.Size(totalSize)),
		CreateTime: CurrentTime(),
	}
}

type PeerConnection struct {
	Connection
	remotePeerInfo    RemotePeerInfo
	remoteHash        protocol.Hash
	transfer          *Transfer
	remotePieces      protocol.BitField
	speed             PeerSpeed
	peerInfo          *Peer
	failed            bool
	transferringData  bool
	recvReq           data.PeerRequest
	recvReqCompressed bool
	recvPos           int
	endpoint          protocol.Endpoint
	combiner          protocol.PacketCombiner
	downloadQueue     []PendingBlock
	uploadState       UploadState
	uploadQueueRank   uint16
	uploadWaitStart   int64
	uploadStartTime   int64
	uploadSessionBase int64
	lastUploadRequest int64
	uploadBlocks      []RequestedUploadBlock
	uploadDone        []RequestedUploadBlock
	uploadAddNext     bool
	friendSlot        bool
}

func NewPeerConnection(session *Session, point protocol.Endpoint, transfer *Transfer, peerInfo *Peer) *PeerConnection {
	return &PeerConnection{
		Connection:     NewConnection(session),
		transfer:       transfer,
		speed:          PeerSpeedSlow,
		peerInfo:       peerInfo,
		endpoint:       point,
		remotePeerInfo: RemotePeerInfo{},
		combiner:       clientproto.NewPacketCombiner(),
		downloadQueue:  make([]PendingBlock, 0),
		uploadBlocks:   make([]RequestedUploadBlock, 0),
		uploadDone:     make([]RequestedUploadBlock, 0),
	}
}

func NewIncomingPeerConnection(session *Session, conn net.Conn) *PeerConnection {
	pc := &PeerConnection{
		Connection:     NewConnection(session),
		speed:          PeerSpeedSlow,
		remotePeerInfo: RemotePeerInfo{},
		combiner:       clientproto.NewPacketCombiner(),
		downloadQueue:  make([]PendingBlock, 0),
		uploadBlocks:   make([]RequestedUploadBlock, 0),
		uploadDone:     make([]RequestedUploadBlock, 0),
	}
	pc.socket = conn
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		pc.endpoint = protocol.EndpointFromInet(tcpAddr)
	}
	return pc
}

func (p *PeerConnection) HasEndpoint() bool {
	return p.endpoint.Defined()
}

func (p *PeerConnection) Connect() error {
	addr, err := p.endpoint.ToTCPAddr()
	if err != nil {
		return err
	}
	if err := p.Connection.Connect(addr); err != nil {
		return err
	}
	p.OnConnect()
	return nil
}

func (p *PeerConnection) OnDisconnect(ec BaseErrorCode) {
	debugPeerf("peer %s disconnect code=%d", p.endpoint.String(), ec.Code())
	if ec.Code() != NoError.Code() && ec.Code() != TransferPaused.Code() {
		p.failed = true
	}
	if q := p.session.UploadQueue(); q != nil {
		q.RemoveFromUploadQueue(p)
	}
	if p.transfer != nil {
		transfer := p.transfer
		p.AbortAllRequests()
		transfer.AddStats(p.Statistics())
		transfer.RemovePeerConnection(p)
		p.transfer = nil
	}
	p.session.CloseConnection(p)
}

func (p *PeerConnection) SecondTick(tickIntervalMS int64) {
	if p.IsDisconnecting() {
		return
	}
	p.Connection.SecondTick(tickIntervalMS)
	now := CurrentTime()
	if now-p.lastReceive > int64(p.session.settings.PeerConnectionTimeout)*1000 {
		p.Close(ConnectionTimeout)
		return
	}
	if p.hasStalledDownloadRequest(now) {
		p.Close(ConnectionTimeout)
	}
}

func (p *PeerConnection) Endpoint() protocol.Endpoint {
	return p.endpoint
}

func (p *PeerConnection) UploadState() UploadState {
	return p.uploadState
}

func (p *PeerConnection) SetUploadState(state UploadState) {
	p.uploadState = state
	if state != UploadStateUploading {
		p.uploadStartTime = 0
	}
}

func (p *PeerConnection) UploadQueueRank() uint16 {
	return p.uploadQueueRank
}

func (p *PeerConnection) SetUploadQueueRank(rank uint16) {
	p.uploadQueueRank = rank
}

func (p *PeerConnection) UploadWaitStart() int64 {
	return p.uploadWaitStart
}

func (p *PeerConnection) SetUploadWaitStart(ts int64) {
	p.uploadWaitStart = ts
}

func (p *PeerConnection) ClearUploadWaitStart() {
	p.uploadWaitStart = 0
}

func (p *PeerConnection) SetUploadStartTime(ts int64) {
	p.uploadStartTime = ts
}

func (p *PeerConnection) UploadStartDelay() int64 {
	if p.uploadStartTime == 0 {
		return 0
	}
	return CurrentTime() - p.uploadStartTime
}

func (p *PeerConnection) ResetUploadSession() {
	p.uploadSessionBase = p.Statistics().TotalUpload()
}

func (p *PeerConnection) UploadSession() int64 {
	total := p.Statistics().TotalUpload()
	if total < p.uploadSessionBase {
		return 0
	}
	return total - p.uploadSessionBase
}

func (p *PeerConnection) UploadAddNextConnect() bool {
	return p.uploadAddNext
}

func (p *PeerConnection) SetUploadAddNextConnect(v bool) {
	p.uploadAddNext = v
}

func (p *PeerConnection) FriendSlot() bool {
	return p.friendSlot
}

func (p *PeerConnection) SetFriendSlot(v bool) {
	p.friendSlot = v
}

func (p *PeerConnection) IsUploadConnected() bool {
	return p != nil && p.socket != nil && !p.IsDisconnecting()
}

func (p *PeerConnection) IsUploadLowID() bool {
	if p == nil {
		return false
	}
	if p.peerInfo != nil {
		return !p.peerInfo.Connectable
	}
	return false
}

func (p *PeerConnection) PrepareHelloAnswer() clientproto.HelloAnswer {
	const clientSoftwareAMule = 3
	mo := MiscOptions{
		UnicodeSupport:     1,
		DataCompVer:        p.session.GetCompressionVersion(),
		SourceExchange1Ver: 0,
		NoViewSharedFiles:  1,
	}
	var mo2 MiscOptions2
	mo2.SetCaptcha()
	mo2.SetLargeFiles()
	mo2.SetSourceExt2()
	return clientproto.HelloAnswer{
		Hash:  p.session.GetUserAgent(),
		Point: protocol.NewEndpoint(p.session.GetClientID(), p.session.GetListenPort()),
		Properties: protocol.TagList{
			protocol.NewStringTag(0x01, p.session.GetClientName()),
			protocol.NewStringTag(0x55, p.session.GetModName()),
			protocol.NewUInt32Tag(0x11, uint32(p.session.GetAppVersion())),
			protocol.NewUInt32Tag(0xF9, 0),
			protocol.NewUInt32Tag(0xFB, uint32((clientSoftwareAMule<<24)|((p.session.GetModMajorVersion()&0x7f)<<17)|((p.session.GetModMinorVersion()&0x7f)<<10)|((p.session.GetModBuildVersion()&0x7f)<<7))),
			protocol.NewUInt32Tag(0xFA, uint32(mo.IntValue())),
			protocol.NewUInt32Tag(0xFE, uint32(mo2.Value)),
		},
		ServerPoint: protocol.Endpoint{},
	}
}

func (p *PeerConnection) PrepareHello() clientproto.Hello {
	return clientproto.Hello{
		HashLength:  16,
		HelloAnswer: p.PrepareHelloAnswer(),
	}
}

func (p *PeerConnection) OnConnect() {
	packet := p.PrepareHello()
	if raw, err := p.combiner.Pack("client.Hello", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendExtHelloAnswer() {
	packet := clientproto.ExtHelloAnswer{
		ExtendedHandshake: clientproto.ExtendedHandshake{
			Version:         0x10,
			ProtocolVersion: 0x01,
			Properties: protocol.TagList{
				protocol.NewUInt32Tag(0x20, 0),
				protocol.NewUInt32Tag(0x21, 0),
				protocol.NewUInt32Tag(0x22, 0),
				protocol.NewUInt32Tag(0x23, 0),
				protocol.NewUInt32Tag(0x24, 0),
				protocol.NewUInt32Tag(0x25, 0),
				protocol.NewUInt32Tag(0x26, 0x03),
				protocol.NewUInt32Tag(0x27, 0),
				protocol.NewUInt32Tag(0x55, uint32(p.session.settings.Version)),
			},
		},
	}
	if raw, err := p.combiner.Pack("client.ExtHelloAnswer", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendFileRequest(hash protocol.Hash) {
	debugPeerf("peer %s -> FileRequest", p.endpoint.String())
	packet := clientproto.FileRequest{Hash: hash}
	if raw, err := p.combiner.Pack("client.FileRequest", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendFileAnswer(transfer *Transfer) {
	if transfer == nil {
		return
	}
	packet := clientproto.FileAnswer{
		Hash: transfer.GetHash(),
		Name: protocol.ByteContainer16FromString(filepath.Base(transfer.FileName())),
	}
	if raw, err := p.combiner.Pack("client.FileAnswer", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendFileStatusRequest(hash protocol.Hash) {
	debugPeerf("peer %s -> FileStatusRequest", p.endpoint.String())
	packet := clientproto.FileStatusRequest{Hash: hash}
	if raw, err := p.combiner.Pack("client.FileStatusRequest", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendFileStatusAnswer(transfer *Transfer) {
	if transfer == nil {
		return
	}
	packet := clientproto.FileStatusAnswer{
		Hash:     transfer.GetHash(),
		BitField: transfer.AvailablePieces(),
	}
	if raw, err := p.combiner.Pack("client.FileStatusAnswer", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendHashSetRequest(hash protocol.Hash) {
	debugPeerf("peer %s -> HashSetRequest", p.endpoint.String())
	packet := clientproto.HashSetRequest{Hash: hash}
	if raw, err := p.combiner.Pack("client.HashSetRequest", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendHashSetAnswer(transfer *Transfer) {
	if transfer == nil {
		return
	}
	packet := clientproto.HashSetAnswer{
		Hash:  transfer.GetHash(),
		Parts: transfer.UploadHashSet(),
	}
	if len(packet.Parts) == 0 {
		return
	}
	if raw, err := p.combiner.Pack("client.HashSetAnswer", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendStartUpload(hash protocol.Hash) {
	debugPeerf("peer %s -> StartUpload", p.endpoint.String())
	packet := clientproto.StartUpload{Hash: hash}
	if raw, err := p.combiner.Pack("client.StartUpload", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendAcceptUpload() {
	packet := clientproto.AcceptUpload{}
	if raw, err := p.combiner.Pack("client.AcceptUpload", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendQueueRanking(rank uint16) {
	packet := clientproto.QueueRanking{Rank: rank}
	if raw, err := p.combiner.Pack("client.QueueRanking", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendOutOfParts() {
	packet := clientproto.OutOfParts{}
	if raw, err := p.combiner.Pack("client.OutOfParts", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendCancelTransfer() {
	packet := clientproto.CancelTransfer{}
	if raw, err := p.combiner.Pack("client.CancelTransfer", &packet); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendRequestParts32(packet *clientproto.RequestParts32) {
	if packet == nil {
		return
	}
	if raw, err := p.combiner.Pack("client.RequestParts32", packet); err == nil {
		debugPeerf("peer %s request32 raw=% X", p.endpoint.String(), raw)
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendRequestParts64(packet *clientproto.RequestParts64) {
	if packet == nil {
		return
	}
	if raw, err := p.combiner.Pack("client.RequestParts64", packet); err == nil {
		debugPeerf("peer %s request64 raw=% X", p.endpoint.String(), raw)
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) SendPart(begin, end int64, payload []byte) error {
	if p.transfer == nil || begin < 0 || end <= begin || len(payload) != int(end-begin) {
		return NewError(IllegalArgument)
	}
	if p.transfer.Size() <= math.MaxUint32 {
		packet := clientproto.SendingPart32{
			Hash:        p.transfer.GetHash(),
			BeginOffset: uint32(begin),
			EndOffset:   uint32(end),
		}
		raw, protoSize, err := p.combiner.PackPayload("client.SendingPart32", &packet, payload)
		if err != nil {
			return err
		}
		p.QueuePacketWithStats(raw, int64(protoSize), int64(len(payload)))
		p.session.Credits().AddUploaded(p.remoteHash, int64(len(payload)))
		return nil
	}
	packet := clientproto.SendingPart64{
		Hash:        p.transfer.GetHash(),
		BeginOffset: uint64(begin),
		EndOffset:   uint64(end),
	}
	raw, protoSize, err := p.combiner.PackPayload("client.SendingPart64", &packet, payload)
	if err != nil {
		return err
	}
	p.QueuePacketWithStats(raw, int64(protoSize), int64(len(payload)))
	p.session.Credits().AddUploaded(p.remoteHash, int64(len(payload)))
	return nil
}

func (p *PeerConnection) AddUploadRequest(req data.PeerRequest) {
	if p.uploadState != UploadStateUploading {
		return
	}
	block := RequestedUploadBlock{
		Begin: req.Range().Left,
		End:   req.Range().Right,
	}
	for _, item := range p.uploadDone {
		if item.Begin == block.Begin && item.End == block.End {
			return
		}
	}
	for _, item := range p.uploadBlocks {
		if item.Begin == block.Begin && item.End == block.End {
			return
		}
	}
	p.uploadBlocks = append(p.uploadBlocks, block)
}

func (p *PeerConnection) ClearUploadBlockRequests() {
	p.uploadBlocks = p.uploadBlocks[:0]
	p.uploadDone = p.uploadDone[:0]
}

func (p *PeerConnection) SendBlockData() {
	if p.uploadState != UploadStateUploading {
		return
	}
	if q := p.session.UploadQueue(); q != nil && q.CheckForTimeOver(p) {
		q.RemoveFromUploadQueue(p)
		p.SendOutOfPartReqsAndAddToWaitingQueue()
		return
	}
	if len(p.uploadBlocks) == 0 || p.transfer == nil {
		return
	}
	current := p.uploadBlocks[0]
	p.uploadBlocks = p.uploadBlocks[1:]
	if !p.transfer.CanUploadRange(current.Begin, current.End) {
		p.SendOutOfPartReqsAndAddToWaitingQueue()
		return
	}
	payload, err := p.transfer.ReadRange(current.Begin, current.End)
	if err != nil {
		if q := p.session.UploadQueue(); q != nil {
			q.RemoveFromUploadQueue(p)
		}
		return
	}
	togo := len(payload)
	if togo == 0 {
		return
	}
	packetSize := togo
	if togo > 10240 {
		packetSize = togo / (togo / 10240)
	}
	offset := current.Begin
	for togo > 0 {
		if togo < packetSize*2 {
			packetSize = togo
		}
		chunk := payload[len(payload)-togo : len(payload)-togo+packetSize]
		if err := p.SendPart(offset, offset+int64(packetSize), chunk); err != nil {
			if q := p.session.UploadQueue(); q != nil {
				q.RemoveFromUploadQueue(p)
			}
			return
		}
		offset += int64(packetSize)
		togo -= packetSize
	}
	current.Transferred = current.End - current.Begin
	p.uploadDone = append(p.uploadDone, current)
}

func (p *PeerConnection) SendOutOfPartReqsAndAddToWaitingQueue() {
	p.SendOutOfParts()
	p.ClearUploadBlockRequests()
	if q := p.session.UploadQueue(); q != nil {
		q.AddClientToQueue(p)
	}
}

func (p *PeerConnection) HandleHelloAnswer(value *clientproto.HelloAnswer) {
	if value == nil {
		return
	}
	p.remoteHash = value.Hash
	p.friendSlot = p.session.IsFriendSlot(value.Hash)
	if value.Point.Defined() {
		p.endpoint.AssignEndpoint(value.Point)
	}
	debugPeerf("peer %s <- HelloAnswer", p.endpoint.String())
	if p.transfer != nil {
		p.SendFileRequest(p.transfer.GetHash())
	}
}

func (p *PeerConnection) HandleExtHello(_ *clientproto.ExtHello) {
	debugPeerf("peer %s <- ExtHello", p.endpoint.String())
	p.SendExtHelloAnswer()
}

func (p *PeerConnection) HandleClientHello(value *clientproto.Hello) {
	if value == nil {
		return
	}
	p.remoteHash = value.Hash
	p.friendSlot = p.session.IsFriendSlot(value.Hash)
	if value.Point.Defined() {
		p.endpoint.AssignEndpoint(value.Point)
	}
	debugPeerf("peer %s <- Hello", p.endpoint.String())
	answer := p.PrepareHelloAnswer()
	if raw, err := p.combiner.Pack("client.HelloAnswer", &answer); err == nil {
		p.QueuePacket(raw)
	}
}

func (p *PeerConnection) attachTransferByHash(hash protocol.Hash) *Transfer {
	if p.transfer != nil && p.transfer.GetHash().Equal(hash) {
		return p.transfer
	}
	transfer := p.session.LookupTransfer(hash)
	if transfer == nil || !transfer.CanUpload() {
		return nil
	}
	if err := transfer.AttachIncomingPeer(p); err != nil {
		return nil
	}
	return transfer
}

func (p *PeerConnection) HandleClientFileRequest(value *clientproto.FileRequest) {
	if value == nil {
		return
	}
	transfer := p.attachTransferByHash(value.Hash)
	if transfer == nil {
		p.Close(NoTransfer)
		return
	}
	p.SendFileAnswer(transfer)
}

func (p *PeerConnection) HandleClientFileStatusRequest(value *clientproto.FileStatusRequest) {
	if value == nil {
		return
	}
	transfer := p.attachTransferByHash(value.Hash)
	if transfer == nil {
		packet := clientproto.NoFileStatus{}
		if raw, err := p.combiner.Pack("client.NoFileStatus", &packet); err == nil {
			p.QueuePacket(raw)
		}
		return
	}
	p.SendFileStatusAnswer(transfer)
}

func (p *PeerConnection) HandleClientHashSetRequest(value *clientproto.HashSetRequest) {
	if value == nil {
		return
	}
	transfer := p.attachTransferByHash(value.Hash)
	if transfer == nil {
		p.Close(NoTransfer)
		return
	}
	if len(transfer.UploadHashSet()) == 0 {
		p.Close(WrongHashSet)
		return
	}
	p.SendHashSetAnswer(transfer)
}

func (p *PeerConnection) HandleClientStartUpload(value *clientproto.StartUpload) {
	if value == nil {
		return
	}
	transfer := p.attachTransferByHash(value.Hash)
	if transfer == nil || !transfer.CanUpload() {
		p.SendQueueRanking(1)
		return
	}
	p.session.UploadQueue().AddClientToQueue(p)
}

func (p *PeerConnection) HandleClientCancelTransfer() {
	if q := p.session.UploadQueue(); q != nil {
		q.RemoveFromUploadQueue(p)
	}
}

func (p *PeerConnection) handleUploadRange(begin, end int64) error {
	if p.transfer == nil || !p.transfer.CanUploadRange(begin, end) {
		p.SendOutOfParts()
		return nil
	}
	payload, err := p.transfer.ReadRange(begin, end)
	if err != nil {
		return err
	}
	return p.SendPart(begin, end, payload)
}

func (p *PeerConnection) HandleClientRequestParts32(value *clientproto.RequestParts32) error {
	if value == nil {
		return nil
	}
	transfer := p.attachTransferByHash(value.Hash)
	if transfer == nil {
		p.SendOutOfParts()
		return nil
	}
	for i := 0; i < value.CurrentFree; i++ {
		begin := int64(value.BeginOffset[i])
		end := int64(value.EndOffset[i])
		if end <= begin {
			continue
		}
		reqs, err := data.MakePeerRequests(begin, end, transfer.Size())
		if err != nil {
			return err
		}
		for _, req := range reqs {
			p.AddUploadRequest(req)
		}
	}
	return nil
}

func (p *PeerConnection) HandleClientRequestParts64(value *clientproto.RequestParts64) error {
	if value == nil {
		return nil
	}
	transfer := p.attachTransferByHash(value.Hash)
	if transfer == nil {
		p.SendOutOfParts()
		return nil
	}
	for i := 0; i < value.CurrentFree; i++ {
		begin := int64(value.BeginOffset[i])
		end := int64(value.EndOffset[i])
		if end <= begin {
			continue
		}
		reqs, err := data.MakePeerRequests(begin, end, transfer.Size())
		if err != nil {
			return err
		}
		for _, req := range reqs {
			p.AddUploadRequest(req)
		}
	}
	return nil
}

func (p *PeerConnection) HandleFileAnswer(value *clientproto.FileAnswer) {
	debugPeerf("peer %s <- FileAnswer", p.endpoint.String())
	if p.transfer != nil && value.Hash.Equal(p.transfer.GetHash()) {
		p.SendFileStatusRequest(p.transfer.GetHash())
	} else {
		p.Close(NoTransfer)
	}
}

func (p *PeerConnection) HandleFileStatusAnswer(value *clientproto.FileStatusAnswer) {
	debugPeerf("peer %s <- FileStatusAnswer pieces=%d have=%d first0=%t first1=%t first2=%t",
		p.endpoint.String(),
		p.remotePieces.Len(),
		p.remotePieces.Count(),
		p.remotePieces.GetBit(0),
		p.remotePieces.GetBit(1),
		p.remotePieces.GetBit(2))
	if p.transfer != nil && p.transfer.Size() > 9728000 {
		p.SendHashSetRequest(p.transfer.GetHash())
	} else if p.transfer != nil {
		if p.transfer.GetHash().Equal(value.Hash) {
			p.transfer.SetHashSet(value.Hash, []protocol.Hash{value.Hash})
			p.SendStartUpload(p.transfer.GetHash())
		} else {
			p.Close(HashMismatch)
		}
	}
}

func (p *PeerConnection) ProcessIncoming() error {
	headers, bodies, err := p.ReadFramesWithCombiner(&p.combiner)
	if err != nil {
		return err
	}
	for i, header := range headers {
		packet, err := p.combiner.Unpack(header, bodies[i])
		if err != nil {
			debugPeerf("peer %s unpack error header=%s body=%d err=%v", p.endpoint.String(), header.String(), len(bodies[i]), err)
			return err
		}
		switch value := packet.(type) {
		case *clientproto.HelloAnswer:
			p.HandleHelloAnswer(value)
		case *clientproto.Hello:
			p.HandleClientHello(value)
		case *clientproto.ExtHello:
			p.HandleExtHello(value)
		case *clientproto.FileRequest:
			p.HandleClientFileRequest(value)
		case *clientproto.FileAnswer:
			p.HandleFileAnswer(value)
		case *clientproto.FileStatusRequest:
			p.HandleClientFileStatusRequest(value)
		case *clientproto.FileStatusAnswer:
			p.remotePieces = value.BitField
			p.HandleFileStatusAnswer(value)
		case *clientproto.HashSetRequest:
			p.HandleClientHashSetRequest(value)
		case *clientproto.HashSetAnswer:
			debugPeerf("peer %s <- HashSetAnswer parts=%d", p.endpoint.String(), len(value.Parts))
			if p.transfer != nil {
				if p.transfer.GetHash().Equal(value.Hash) &&
					p.transfer.GetHash().Equal(protocol.HashFromHashSet(value.Parts)) &&
					p.transfer.picker.GetPieceCount() == len(value.Parts) {
					p.transfer.SetHashSet(value.Hash, value.Parts)
					p.SendStartUpload(p.transfer.GetHash())
				} else {
					p.Close(WrongHashSet)
				}
			}
		case *clientproto.AcceptUpload:
			debugPeerf("peer %s <- AcceptUpload", p.endpoint.String())
			p.RequestBlocks()
		case *clientproto.StartUpload:
			p.HandleClientStartUpload(value)
		case *clientproto.RequestParts32:
			if err := p.HandleClientRequestParts32(value); err != nil {
				return err
			}
		case *clientproto.RequestParts64:
			if err := p.HandleClientRequestParts64(value); err != nil {
				return err
			}
		case *clientproto.CancelTransfer:
			p.HandleClientCancelTransfer()
		case *clientproto.QueueRanking:
			debugPeerf("peer %s <- QueueRanking", p.endpoint.String())
			p.Close(QueueRanking)
		case *clientproto.SendingPart32:
			debugPeerf("peer %s <- SendingPart32 %d..%d", p.endpoint.String(), value.BeginOffset, value.EndOffset)
			if req, err := data.MakePeerRequest(int64(value.BeginOffset), int64(value.EndOffset)); err == nil {
				p.ReceiveData(req, false)
			}
		case *clientproto.SendingPart64:
			debugPeerf("peer %s <- SendingPart64 %d..%d", p.endpoint.String(), value.BeginOffset, value.EndOffset)
			if req, err := data.MakePeerRequest(int64(value.BeginOffset), int64(value.EndOffset)); err == nil {
				p.ReceiveData(req, false)
			}
		case *clientproto.CompressedPart32:
			debugPeerf("peer %s <- CompressedPart32 %d len=%d", p.endpoint.String(), value.BeginOffset, value.CompressedLength)
			p.ReceiveCompressedData(header, int64(value.BeginOffset), int64(value.CompressedLength), value.BytesCount())
		case *clientproto.CompressedPart64:
			debugPeerf("peer %s <- CompressedPart64 %d len=%d", p.endpoint.String(), value.BeginOffset, value.CompressedLength)
			p.ReceiveCompressedData(header, int64(value.BeginOffset), int64(value.CompressedLength), value.BytesCount())
		case *clientproto.NoFileStatus:
			p.Close(NoTransfer)
		case *clientproto.OutOfParts:
			p.Close(OutOfParts)
		}
	}
	return nil
}

func (p *PeerConnection) SetPeer(peer *Peer) {
	p.peerInfo = peer
}

func (p *PeerConnection) SetTransfer(transfer *Transfer) {
	p.transfer = transfer
}

func (p *PeerConnection) GetInfo() PeerInfo {
	stats := p.Statistics()
	info := PeerInfo{
		DownloadSpeed:        int(stats.DownloadRate()),
		PayloadDownloadSpeed: int(stats.DownloadPayloadRate()),
		UploadSpeed:          int(stats.UploadRate()),
		PayloadUploadSpeed:   int(stats.UploadPayloadRate()),
		RemotePieces:         p.remotePieces,
		Endpoint:             p.endpoint,
		SourceFlag:           0,
	}
	if p.peerInfo != nil {
		info.FailCount = p.peerInfo.FailCount
		info.SourceFlag = p.peerInfo.SourceFlag
	}
	info.ModName = p.remotePeerInfo.ModName
	info.Version = p.remotePeerInfo.Version
	info.ModVersion = p.remotePeerInfo.ModNumber
	info.StrModVersion = p.remotePeerInfo.ModVersion
	return info
}

func (p *PeerConnection) GetPeer() *Peer {
	return p.peerInfo
}

func (p *PeerConnection) Speed() PeerSpeed {
	if p.transfer != nil {
		downloadRate := p.Statistics().DownloadPayloadRate()
		transferDownloadRate := p.transfer.stat.DownloadPayloadRate()
		if downloadRate > 512 && downloadRate > transferDownloadRate/16 {
			p.speed = PeerSpeedFast
		} else if downloadRate > 4096 && downloadRate > transferDownloadRate/64 {
			p.speed = PeerSpeedMedium
		} else if downloadRate < transferDownloadRate/15 && p.speed == PeerSpeedFast {
			p.speed = PeerSpeedMedium
		} else {
			p.speed = PeerSpeedSlow
		}
	}
	return p.speed
}

func (p *PeerConnection) IsRequesting(block data.PieceBlock) bool {
	return p.GetDownloading(block) != nil
}

func (p *PeerConnection) GetDownloading(block data.PieceBlock) *PendingBlock {
	for i := range p.downloadQueue {
		if p.downloadQueue[i].Block.Compare(block) == 0 {
			return &p.downloadQueue[i]
		}
	}
	return nil
}

func (p *PeerConnection) RequestBlocks() {
	if p.transfer == nil || p.transferringData || len(p.downloadQueue) != 0 {
		return
	}
	blocks := make([]data.PieceBlock, 0)
	p.transfer.picker.PickPiecesWithAvailability(&blocks, RequestQueueSize, p.GetPeer(), p.Speed(), &p.remotePieces)
	use32 := p.transfer.Size() <= math.MaxUint32
	req32 := &clientproto.RequestParts32{Hash: p.transfer.GetHash()}
	req64 := &clientproto.RequestParts64{Hash: p.transfer.GetHash()}
	for len(blocks) > 0 && len(p.downloadQueue) < RequestQueueSize {
		block := blocks[0]
		blocks = blocks[1:]
		p.downloadQueue = append(p.downloadQueue, NewPendingBlock(block, p.transfer.Size()))
		if use32 {
			req32.AppendRange(block.Range(p.transfer.Size()))
		} else {
			req64.AppendRange(block.Range(p.transfer.Size()))
		}
	}
	if use32 && req32.CurrentFree > 0 {
		debugPeerf("peer %s -> RequestParts32 blocks=%d ranges=[%d..%d][%d..%d][%d..%d]",
			p.endpoint.String(),
			req32.CurrentFree,
			req32.BeginOffset[0], req32.EndOffset[0],
			req32.BeginOffset[1], req32.EndOffset[1],
			req32.BeginOffset[2], req32.EndOffset[2])
		p.SendRequestParts32(req32)
	} else if !use32 && req64.CurrentFree > 0 {
		debugPeerf("peer %s -> RequestParts64 blocks=%d ranges=[%d..%d][%d..%d][%d..%d]",
			p.endpoint.String(),
			req64.CurrentFree,
			req64.BeginOffset[0], req64.EndOffset[0],
			req64.BeginOffset[1], req64.EndOffset[1],
			req64.BeginOffset[2], req64.EndOffset[2])
		p.SendRequestParts64(req64)
	} else {
		debugPeerf("peer %s no blocks to request, closing", p.endpoint.String())
		p.Close(NoError)
	}
}

func (p *PeerConnection) AbortAllRequests() {
	if p.transfer != nil {
		for _, pb := range p.downloadQueue {
			p.transfer.picker.AbortDownload(pb.Block, p.GetPeer())
		}
	}
	p.downloadQueue = nil
}

func (p *PeerConnection) hasStalledDownloadRequest(now int64) bool {
	if p.transfer == nil || p.transferringData || len(p.downloadQueue) == 0 {
		return false
	}
	timeout := int64(p.session.settings.PeerConnectionTimeout) * 500
	if timeout < Seconds(10) {
		timeout = Seconds(10)
	}
	if p.MillisecondsSinceLastReceive() < timeout/2 {
		return false
	}
	for _, pb := range p.downloadQueue {
		if now-pb.CreateTime >= timeout {
			return true
		}
	}
	return false
}

func (p *PeerConnection) ReceiveCompressedData(header protocol.PacketHeader, offset, compressedLength int64, payloadSize int) {
	block := data.MakePieceBlock(offset)
	pb := p.GetDownloading(block)
	if pb != nil && len(pb.Buffer) == 0 {
		pb.DataSize = compressedLength
	}
	beginOffset := offset
	if pb != nil {
		beginOffset = int64(pb.Block.Range(p.transfer.Size()).Left) + pb.Received
	}
	endOffset := beginOffset + int64(header.SizePacket()) - int64(payloadSize)
	if req, err := data.MakePeerRequest(beginOffset, endOffset); err == nil {
		p.ReceiveData(req, true)
	}
}

func (p *PeerConnection) ReceiveData(req data.PeerRequest, compressed bool) {
	p.transferringData = true
	p.recvReq = req
	p.recvPos = 0
	p.recvReqCompressed = compressed
	p.ReceivePendingData()
}

func (p *PeerConnection) ReceivePendingData() {
	if !p.transferringData || p.transfer == nil {
		return
	}
	block := data.NewPieceBlock(p.recvReq.Piece, int(p.recvReq.Start/BlockSize))
	pb := p.GetDownloading(block)
	if pb == nil {
		p.skipPendingData()
		return
	}
	if pb.Buffer == nil {
		pb.Buffer = make([]byte, block.Size(p.transfer.Size()))
	}
	remaining := int(p.recvReq.Length) - p.recvPos
	if remaining <= 0 {
		p.transferringData = false
		return
	}
	payload := p.ConsumeIncoming(remaining)
	if len(payload) == 0 {
		return
	}
	p.session.Credits().AddDownloaded(p.remoteHash, int64(len(payload)))
	offset := int(p.recvReq.InBlockOffset()) + p.recvPos
	copy(pb.Buffer[offset:], payload)
	pb.Received += int64(len(payload))
	p.recvPos += len(payload)
	if p.recvPos < int(p.recvReq.Length) {
		return
	}
	if p.CompleteBlock(pb) {
		block := pb.Block
		buffer := pb.Buffer
		wasFinished := p.transfer.picker.IsPieceFinished(p.recvReq.Piece)
		wasDownloading := p.transfer.picker.MarkAsWriting(block)
		p.removePending(block)
		if wasDownloading {
			p.asyncWrite(block, buffer, p.transfer)
			if p.transfer.picker.IsPieceFinished(p.recvReq.Piece) && !wasFinished {
				p.transfer.QueuePieceHash(block.PieceIndex)
			}
		}
		p.transferringData = false
		p.continueBufferedIncoming()
		p.RequestBlocks()
		return
	}
	p.transferringData = false
	p.continueBufferedIncoming()
}

func (p *PeerConnection) skipPendingData() {
	remaining := int(p.recvReq.Length) - p.recvPos
	if remaining <= 0 {
		p.transferringData = false
		p.continueBufferedIncoming()
		p.RequestBlocks()
		return
	}
	chunk := p.ConsumeIncoming(remaining)
	p.recvPos += len(chunk)
	if p.recvPos >= int(p.recvReq.Length) {
		p.transferringData = false
		p.continueBufferedIncoming()
		p.RequestBlocks()
	}
}

func (p *PeerConnection) continueBufferedIncoming() {
	for !p.transferringData && len(p.incoming) != 0 {
		before := p.IncomingBytes()
		if err := p.ProcessIncoming(); err != nil {
			p.Close(IOException)
			return
		}
		after := p.IncomingBytes()
		if len(p.incoming) == 0 || after >= before {
			return
		}
	}
}

func (p *PeerConnection) CompleteBlock(pb *PendingBlock) bool {
	if pb == nil {
		return false
	}
	if pb.Received < pb.DataSize {
		return false
	}
	if p.recvReqCompressed {
		reader, err := zlib.NewReader(bytes.NewReader(pb.Buffer[:pb.Received]))
		if err == nil {
			defer reader.Close()
			raw, err := io.ReadAll(reader)
			if err == nil {
				pb.Buffer = raw
			}
		}
	}
	return true
}

func (p *PeerConnection) UploadScore() uint32 {
	if p == nil || p.uploadState == UploadStateUploading {
		return 0
	}
	if p.FriendSlot() && !p.IsUploadLowID() {
		return 0x0FFFFFFF
	}
	waitStart := p.UploadWaitStart()
	if waitStart == 0 {
		return 0
	}
	base := float64(CurrentTime()-waitStart) / 1000.0
	base *= p.session.Credits().ScoreRatio(p.remoteHash)
	if p.transfer != nil {
		base *= p.transfer.UploadPriority().ScoreFactor()
	}
	if base < 0 {
		return 0
	}
	return uint32(base)
}

func (p *PeerConnection) removePending(block data.PieceBlock) {
	dst := p.downloadQueue[:0]
	for _, pb := range p.downloadQueue {
		if pb.Block.Compare(block) != 0 {
			dst = append(dst, pb)
		}
	}
	p.downloadQueue = dst
}

func (p *PeerConnection) asyncWrite(block data.PieceBlock, buffer []byte, transfer *Transfer) {
	transfer.picker.MarkAsWriting(block)
	p.session.SubmitDiskTask(NewAsyncWrite(block, buffer, transfer))
}
