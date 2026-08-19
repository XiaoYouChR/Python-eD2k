package goed2k

import (
	"bytes"
	"context"
	"io"
	"net"
	"time"

	"github.com/monkeyWie/goed2k/internal/logx"
	"github.com/monkeyWie/goed2k/protocol"
)

const (
	connectionIODeadline     = 250 * time.Microsecond
	connectionDialTimeout    = 5 * time.Second
	connectionReadBufferSize = 32 * 1024
	connectionReadBudget     = 512 * 1024
)

type connectionDialResult struct {
	socket net.Conn
	err    error
}

type connectionDialFunc func(ctx context.Context, network, address string) (net.Conn, error)

var defaultConnectionDialContext connectionDialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, address)
}

type Connection struct {
	socket        net.Conn
	session       *Session
	stat          Statistics
	lastReceive   int64
	disconnecting bool
	closeCode     BaseErrorCode
	closeHandled  bool
	outgoing      []queuedPacket
	incoming      [][]byte
	header        protocol.PacketHeader
	headerBuffer  []byte
	bodyBuffer    []byte
	connecting    bool
	dialCancel    context.CancelFunc
	dialResult    <-chan connectionDialResult
	dialContext   connectionDialFunc
}

type queuedPacket struct {
	data          []byte
	protocolBytes int64
	payloadBytes  int64
}

func NewConnection(session *Session) Connection {
	return Connection{
		session:      session,
		stat:         NewStatistics(),
		lastReceive:  CurrentTime(),
		closeCode:    NoError,
		outgoing:     make([]queuedPacket, 0),
		incoming:     make([][]byte, 0),
		headerBuffer: make([]byte, 0, protocol.PacketHeaderSize),
		bodyBuffer:   nil,
		dialContext:  defaultConnectionDialContext,
	}
}

func (c *Connection) DoRead() error {
	if c.socket == nil {
		return nil
	}
	tmp := make([]byte, connectionReadBufferSize)
	_ = c.socket.SetReadDeadline(time.Now().Add(connectionIODeadline))
	for received := 0; received < connectionReadBudget; {
		n, err := c.socket.Read(tmp)
		if n > 0 {
			logx.Debug("socket read", "endpoint", c.Endpoint().String(), "bytes", n)
			c.lastReceive = CurrentTime()
			c.stat.ReceiveBytes(int64(n), 0)
			c.incoming = append(c.incoming, bytes.Clone(tmp[:n]))
			received += n
		}
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return nil
			}
			if err == io.EOF {
				c.Close(EndOfStream)
				return nil
			}
			c.Close(IOException)
			return err
		}
		if n == 0 {
			return nil
		}
	}
	return nil
}

func (c *Connection) Connect(address net.Addr) error {
	tcpAddr, ok := address.(*net.TCPAddr)
	if !ok || tcpAddr == nil {
		return NewError(IllegalArgument)
	}
	if c.socket != nil || c.connecting {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectionDialTimeout)
	result := make(chan connectionDialResult)
	c.connecting = true
	c.dialCancel = cancel
	c.dialResult = result
	dial := c.dialContext
	if dial == nil {
		dial = defaultConnectionDialContext
	}
	go func() {
		conn, err := dial(ctx, "tcp", tcpAddr.String())
		select {
		case result <- connectionDialResult{socket: conn, err: err}:
		case <-ctx.Done():
			if conn != nil {
				_ = conn.Close()
			}
		}
	}()
	return nil
}

func (c *Connection) PollConnect() error {
	if !c.connecting || c.dialResult == nil {
		return nil
	}
	select {
	case result := <-c.dialResult:
		c.connecting = false
		c.dialResult = nil
		if c.dialCancel != nil {
			c.dialCancel()
			c.dialCancel = nil
		}
		if c.disconnecting {
			if result.socket != nil {
				_ = result.socket.Close()
			}
			return nil
		}
		if result.err != nil {
			c.Close(IOException)
			return result.err
		}
		c.socket = result.socket
		c.lastReceive = CurrentTime()
	default:
	}
	return nil
}

func (c *Connection) IsConnecting() bool {
	return c.connecting
}

func (c *Connection) Close(ec BaseErrorCode) {
	if c.disconnecting {
		return
	}
	if c.dialCancel != nil {
		c.dialCancel()
		c.dialCancel = nil
	}
	c.connecting = false
	c.dialResult = nil
	if c.socket != nil {
		_ = c.socket.Close()
		c.socket = nil
	}
	c.disconnecting = true
	c.closeCode = ec
}

func (c *Connection) SecondTick(tickIntervalMS int64) {
	c.stat.SecondTick(tickIntervalMS)
}

func (c *Connection) Statistics() Statistics {
	return c.stat
}

func (c *Connection) IsDisconnecting() bool {
	return c.disconnecting
}

func (c *Connection) DisconnectCode() BaseErrorCode {
	return c.closeCode
}

func (c *Connection) IsDisconnectHandled() bool {
	return c.closeHandled
}

func (c *Connection) MarkDisconnectHandled() {
	c.closeHandled = true
}

func (c *Connection) MillisecondsSinceLastReceive() int64 {
	return CurrentTime() - c.lastReceive
}

func (c *Connection) Endpoint() protocol.Endpoint {
	return protocol.Endpoint{}
}

func (c *Connection) QueuePacket(packet []byte) {
	c.QueuePacketWithStats(packet, int64(len(packet)), 0)
}

func (c *Connection) QueuePacketWithStats(packet []byte, protocolBytes, payloadBytes int64) {
	if len(packet) == 0 {
		return
	}
	buf := bytes.Clone(packet)
	c.outgoing = append(c.outgoing, queuedPacket{
		data:          buf,
		protocolBytes: protocolBytes,
		payloadBytes:  payloadBytes,
	})
}

func (c *Connection) PendingPackets() [][]byte {
	out := make([][]byte, len(c.outgoing))
	for i := range c.outgoing {
		out[i] = bytes.Clone(c.outgoing[i].data)
	}
	return out
}

func (c *Connection) PopOutgoing() []byte {
	if len(c.outgoing) == 0 {
		return nil
	}
	packet := c.outgoing[0].data
	c.outgoing = c.outgoing[1:]
	return packet
}

func (c *Connection) AppendIncoming(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	c.incoming = append(c.incoming, bytes.Clone(chunk))
}

func (c *Connection) prependIncoming(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	c.incoming = append([][]byte{bytes.Clone(chunk)}, c.incoming...)
}

func (c *Connection) IncomingChunks() [][]byte {
	out := make([][]byte, len(c.incoming))
	copy(out, c.incoming)
	return out
}

func (c *Connection) IncomingBytes() int {
	total := 0
	for _, chunk := range c.incoming {
		total += len(chunk)
	}
	return total
}

func (c *Connection) DrainIncoming() []byte {
	if len(c.incoming) == 0 {
		return nil
	}
	total := 0
	for _, chunk := range c.incoming {
		total += len(chunk)
	}
	buf := make([]byte, 0, total)
	for _, chunk := range c.incoming {
		buf = append(buf, chunk...)
	}
	c.incoming = c.incoming[:0]
	return buf
}

func (c *Connection) ConsumeIncoming(limit int) []byte {
	if limit <= 0 {
		return nil
	}
	raw := c.DrainIncoming()
	if len(raw) == 0 {
		return nil
	}
	if len(raw) <= limit {
		return raw
	}
	chunk := bytes.Clone(raw[:limit])
	c.prependIncoming(raw[limit:])
	return chunk
}

func (c *Connection) ReadFrames() ([]protocol.PacketHeader, [][]byte, error) {
	return c.ReadFramesWithCombiner(nil)
}

func (c *Connection) ReadFramesWithCombiner(combiner *protocol.PacketCombiner) ([]protocol.PacketHeader, [][]byte, error) {
	raw := c.DrainIncoming()
	if len(raw) == 0 {
		return nil, nil, nil
	}
	reader := bytes.NewReader(raw)
	headers := make([]protocol.PacketHeader, 0)
	bodies := make([][]byte, 0)
	for reader.Len() >= protocol.PacketHeaderSize {
		frameStart := len(raw) - reader.Len()
		var header protocol.PacketHeader
		if err := header.Get(reader); err != nil {
			logx.Debug("read frame header error", "raw_len", len(raw), "err", err)
			return headers, bodies, err
		}
		bodySize := int(header.SizePacket())
		serviceSize := bodySize
		if combiner != nil {
			serviceSize = combiner.ServiceSize(header)
		}
		if bodySize < 0 || serviceSize < 0 || serviceSize > bodySize {
			logx.Debug("read frame invalid header", "header", header.String(), "body_size", bodySize, "service_size", serviceSize, "remaining", reader.Len())
			return headers, bodies, io.ErrUnexpectedEOF
		}
		if reader.Len() < serviceSize {
			logx.Debug("read frame incomplete", "header", header.String(), "body_size", bodySize, "service_size", serviceSize, "remaining", reader.Len(), "raw_len", len(raw))
			c.prependIncoming(raw[frameStart:])
			break
		}
		body, err := protocol.ReadBytes(reader, serviceSize)
		if err != nil {
			logx.Debug("read frame body error", "header", header.String(), "body_size", bodySize, "service_size", serviceSize, "remaining", reader.Len(), "err", err)
			return headers, bodies, err
		}
		headers = append(headers, header)
		bodies = append(bodies, body)
		if bodySize > serviceSize {
			c.prependIncoming(raw[len(raw)-reader.Len():])
			return headers, bodies, nil
		}
	}
	return headers, bodies, nil
}

func (c *Connection) DecodeFrames(combiner *protocol.PacketCombiner) ([]protocol.PacketHeader, []protocol.Serializable, error) {
	if combiner == nil {
		return nil, nil, nil
	}
	headers, bodies, err := c.ReadFrames()
	if err != nil {
		return nil, nil, err
	}
	packets := make([]protocol.Serializable, 0, len(headers))
	for i, header := range headers {
		packet, err := combiner.Unpack(header, bodies[i])
		if err != nil {
			return headers[:i], packets, err
		}
		packets = append(packets, packet)
	}
	return headers, packets, nil
}

func (c *Connection) FlushOutgoing() error {
	if c.socket == nil {
		return nil
	}
	if len(c.outgoing) == 0 {
		return nil
	}
	packet := &c.outgoing[0]
	_ = c.socket.SetWriteDeadline(time.Now().Add(connectionIODeadline))
	n, err := c.socket.Write(packet.data)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil
		}
		c.Close(IOException)
		return err
	}
	if n >= len(packet.data) {
		c.stat.SendBytes(packet.protocolBytes, packet.payloadBytes)
		c.outgoing = c.outgoing[1:]
		return nil
	}
	packet.data = bytes.Clone(packet.data[n:])
	return nil
}
