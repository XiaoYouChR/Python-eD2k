package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Endpoint struct {
	ip   int32
	port int
}

func EndpointFromInet(addr *net.TCPAddr) Endpoint {
	if addr == nil || addr.IP == nil {
		return Endpoint{}
	}
	return Endpoint{
		ip:   packToNetworkByteOrder(addr.IP.To4()),
		port: addr.Port,
	}
}

func EndpointFromString(addr string, port int) (Endpoint, error) {
	ip, err := string2IP(addr)
	if err != nil {
		return Endpoint{}, err
	}
	return Endpoint{ip: ip, port: port}, nil
}

func NewEndpoint(ip int32, port int) Endpoint {
	return Endpoint{ip: ip, port: port}
}

func (e *Endpoint) Assign(ip int32, port int) Endpoint {
	e.ip = ip
	e.port = port
	return *e
}

func (e *Endpoint) AssignEndpoint(point Endpoint) Endpoint {
	e.ip = point.ip
	e.port = point.port
	return *e
}

func (e Endpoint) Defined() bool {
	return e.ip != 0 && e.port != 0
}

func (e Endpoint) String() string {
	return fmt.Sprintf("%s:%d", ip2String(e.ip), e.port)
}

func (e Endpoint) ToTCPAddr() (*net.TCPAddr, error) {
	ip := int2Address(e.ip)
	if ip == nil {
		return nil, errors.New("illegal argument")
	}
	return &net.TCPAddr{IP: ip, Port: e.port}, nil
}

func (e Endpoint) Compare(other Endpoint) int {
	if e.ip > other.ip {
		return 1
	}
	if e.ip < other.ip {
		return -1
	}
	if e.port > other.port {
		return 1
	}
	if e.port < other.port {
		return -1
	}
	return 0
}

func (e Endpoint) Equal(other Endpoint) bool {
	return e.Compare(other) == 0
}

func (e Endpoint) HashCode() int32 {
	return e.ip + int32(e.port)
}

func (e Endpoint) IsZero() bool {
	return e.ip == 0 && e.port == 0
}

func (e Endpoint) MarshalJSON() ([]byte, error) {
	if e.IsZero() {
		return json.Marshal("")
	}
	return json.Marshal(e.String())
}

func (e *Endpoint) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if strings.TrimSpace(value) == "" {
		*e = Endpoint{}
		return nil
	}
	host, port, err := splitHostPortText(value)
	if err != nil {
		return err
	}
	endpoint, err := EndpointFromString(host, port)
	if err != nil {
		return err
	}
	*e = endpoint
	return nil
}

func (e *Endpoint) SetIP(ip int32) {
	e.ip = ip
}

func (e *Endpoint) SetPort(port int) {
	e.port = port
}

func (e Endpoint) IP() int32 {
	return e.ip
}

func (e Endpoint) Port() int {
	return e.port
}

func ip2String(ip int32) string {
	return fmt.Sprintf("%d.%d.%d.%d", byte(ip), byte(ip>>8), byte(ip>>16), byte(ip>>24))
}

func string2IP(s string) (int32, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return 0, errors.New("illegal argument")
	}
	raw := make([]int, 4)
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 || value > 255 {
			return 0, errors.New("illegal argument")
		}
		raw[i] = value
	}
	return int32(
		uint32(raw[0]) |
			(uint32(raw[1]) << 8) |
			(uint32(raw[2]) << 16) |
			(uint32(raw[3]) << 24),
	), nil
}

func splitHostPortText(value string) (string, int, error) {
	host, portString, err := net.SplitHostPort(value)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func int2Address(ip int32) net.IP {
	return net.IPv4(byte(ip), byte(ip>>8), byte(ip>>16), byte(ip>>24)).To4()
}

func packToNetworkByteOrder(order []byte) int32 {
	if len(order) != 4 {
		return 0
	}
	return int32(order[3])<<24 | int32(order[2])<<16 | int32(order[1])<<8 | int32(order[0])
}
