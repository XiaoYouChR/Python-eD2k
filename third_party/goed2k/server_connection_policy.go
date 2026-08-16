package goed2k

import "net"

type ServerConnectionCandidate struct {
	Identifier string
	Address    *net.TCPAddr
}

type ServerConnectionPolicy struct {
	reconnectSecondsTimeout int64
	maxReconnects           int
	iteration               int
	identifier              string
	address                 *net.TCPAddr
	nextConnectTime         int64
}

func NewServerConnectionPolicy(reconnectSecondsTimeout int64, maxReconnects int) ServerConnectionPolicy {
	p := ServerConnectionPolicy{
		reconnectSecondsTimeout: reconnectSecondsTimeout,
		maxReconnects:           maxReconnects,
	}
	p.RemoveConnectCandidates()
	return p
}

func (p *ServerConnectionPolicy) SetConnectCandidate(identifier string, address *net.TCPAddr, currentSessionTime int64) {
	p.iteration = 0
	p.identifier = identifier
	p.address = address
	p.nextConnectTime = currentSessionTime
}

func (p *ServerConnectionPolicy) SetServerConnectionFailed(identifier string, address *net.TCPAddr, currentSessionTime int64) {
	if p.identifier == "" || p.identifier != identifier {
		p.iteration = 0
		p.identifier = identifier
		p.address = address
	} else {
		p.iteration++
	}
	if p.HasCandidate() && p.HasIterations() {
		p.nextConnectTime = currentSessionTime + int64(p.iteration)*p.reconnectSecondsTimeout*1000
	} else {
		p.nextConnectTime = -1
	}
}

func (p ServerConnectionPolicy) HasCandidate() bool {
	return p.identifier != "" && p.address != nil
}

func (p ServerConnectionPolicy) HasIterations() bool {
	return p.iteration < p.maxReconnects
}

func (p *ServerConnectionPolicy) RemoveConnectCandidates() {
	p.iteration = p.maxReconnects
	p.identifier = ""
	p.address = nil
	p.nextConnectTime = -1
}

func (p ServerConnectionPolicy) GetConnectCandidate(currentSessionTime int64) *ServerConnectionCandidate {
	if p.nextConnectTime != -1 && p.nextConnectTime <= currentSessionTime {
		return &ServerConnectionCandidate{
			Identifier: p.identifier,
			Address:    p.address,
		}
	}
	return nil
}
