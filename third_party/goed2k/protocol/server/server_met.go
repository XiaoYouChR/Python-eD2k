package server

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/monkeyWie/goed2k/protocol"
)

const (
	serverMetHeader               = 0x0E
	serverMetHeaderWithLargeFiles = 0x0F

	serverTagName        = 0x01
	serverTagDescription = 0x0B
	serverTagPreference  = 0x0E

	serverMetEntryMinBytes = 6 + 4
)

type ServerMet struct {
	Header  byte
	Servers []ServerMetEntry
}

type ServerMetEntry struct {
	Endpoint protocol.Endpoint
	Tags     protocol.TagList
}

func NewServerMet() ServerMet {
	return ServerMet{Header: serverMetHeader}
}

func NewServerMetEntryFromIP(ip string, port int, name, description string) (ServerMetEntry, error) {
	endpoint, err := protocol.EndpointFromString(ip, port)
	if err != nil {
		return ServerMetEntry{}, err
	}
	entry := ServerMetEntry{
		Endpoint: endpoint,
		Tags: protocol.TagList{
			protocol.NewStringTag(serverTagName, name),
		},
	}
	if description != "" {
		entry.Tags = append(entry.Tags, protocol.NewStringTag(serverTagDescription, description))
	}
	return entry, nil
}

func NewServerMetEntryFromHost(host string, port int, name, description string) (ServerMetEntry, error) {
	host = strings.TrimSpace(host)
	if host == "" || port == 0 || strings.TrimSpace(name) == "" {
		return ServerMetEntry{}, errors.New("invalid server.met entry")
	}
	entry := ServerMetEntry{
		Endpoint: protocol.NewEndpoint(0, port),
		Tags: protocol.TagList{
			protocol.NewStringTag(serverTagPreference, host),
			protocol.NewStringTag(serverTagName, name),
		},
	}
	if description != "" {
		entry.Tags = append(entry.Tags, protocol.NewStringTag(serverTagDescription, description))
	}
	return entry, nil
}

func LoadServerMet(path string) (*ServerMet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseServerMet(data)
}

func ParseServerMet(data []byte) (*ServerMet, error) {
	reader := bytes.NewReader(data)
	var met ServerMet
	if err := met.Get(reader); err != nil {
		return nil, err
	}
	return &met, nil
}

func (m *ServerMet) Get(src *bytes.Reader) error {
	header, err := src.ReadByte()
	if err != nil {
		return err
	}
	switch header {
	case serverMetHeader, serverMetHeaderWithLargeFiles:
		m.Header = header
	default:
		m.Header = serverMetHeader
	}
	count, err := protocol.ReadUInt32(src)
	if err != nil {
		return err
	}
	maxCount := uint32(src.Len() / serverMetEntryMinBytes)
	if count > maxCount {
		return fmt.Errorf("server.met declares %d servers, but payload can contain at most %d", count, maxCount)
	}
	m.Servers = make([]ServerMetEntry, int(count))
	for idx := range m.Servers {
		if err := m.Servers[idx].Get(src); err != nil {
			return err
		}
	}
	return nil
}

func (m ServerMet) Put(dst *bytes.Buffer) error {
	header := m.Header
	if header != serverMetHeader && header != serverMetHeaderWithLargeFiles {
		header = serverMetHeader
	}
	if err := dst.WriteByte(header); err != nil {
		return err
	}
	if err := protocol.WriteUInt32(dst, uint32(len(m.Servers))); err != nil {
		return err
	}
	for _, entry := range m.Servers {
		if err := entry.Put(dst); err != nil {
			return err
		}
	}
	return nil
}

func (m ServerMet) BytesCount() int {
	size := 1 + 4
	for _, entry := range m.Servers {
		size += entry.BytesCount()
	}
	return size
}

func (m *ServerMet) AddServer(entry ServerMetEntry) {
	m.Servers = append(m.Servers, entry)
}

func (m ServerMet) Addresses() []string {
	result := make([]string, 0, len(m.Servers))
	for _, entry := range m.Servers {
		if addr := entry.Address(); addr != "" {
			result = append(result, addr)
		}
	}
	return result
}

func (e *ServerMetEntry) Get(src *bytes.Reader) error {
	endpoint, err := protocol.ReadEndpoint(src)
	if err != nil {
		return err
	}
	e.Endpoint = endpoint
	return e.Tags.Get(src)
}

func (e ServerMetEntry) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteEndpoint(dst, e.Endpoint); err != nil {
		return err
	}
	return e.Tags.Put(dst)
}

func (e ServerMetEntry) BytesCount() int {
	return 6 + e.Tags.BytesCount()
}

func (e ServerMetEntry) Name() string {
	for _, tag := range e.Tags {
		if tag.ID == serverTagName {
			return tag.String
		}
	}
	return ""
}

func (e ServerMetEntry) Description() string {
	for _, tag := range e.Tags {
		if tag.ID == serverTagDescription {
			return tag.String
		}
	}
	return ""
}

func (e ServerMetEntry) Host() string {
	if e.Endpoint.IP() != 0 {
		return endpointIPString(e.Endpoint)
	}
	for _, tag := range e.Tags {
		if tag.ID == serverTagPreference {
			return strings.TrimSpace(tag.String)
		}
	}
	return ""
}

func (e ServerMetEntry) Port() int {
	return e.Endpoint.Port()
}

func (e ServerMetEntry) Address() string {
	host := e.Host()
	if host == "" || e.Port() == 0 {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(e.Port()))
}

func endpointIPString(endpoint protocol.Endpoint) string {
	ip := net.IPv4(byte(endpoint.IP()), byte(endpoint.IP()>>8), byte(endpoint.IP()>>16), byte(endpoint.IP()>>24)).To4()
	if ip == nil {
		return ""
	}
	return ip.String()
}
