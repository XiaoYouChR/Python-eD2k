package goed2k

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	internalupnp "github.com/monkeyWie/goed2k/internal/upnp"
)

func TestUPnPPortMapperMapAndClose(t *testing.T) {
	previousDiscover := discoverUPnPDevices
	defer func() {
		discoverUPnPDevices = previousDiscover
	}()

	deviceA := &fakeUPnPDevice{id: "gw-a"}
	deviceB := &fakeUPnPDevice{id: "gw-b"}
	discoverUPnPDevices = func(timeout time.Duration) []internalupnp.Device {
		if timeout <= 0 {
			t.Fatalf("expected positive timeout, got %s", timeout)
		}
		return []internalupnp.Device{deviceA, deviceB}
	}

	settings := NewSettings()
	settings.EnableUPnP = true
	settings.EnableDHT = true
	settings.ListenPort = 4661
	settings.UDPPort = 4662

	mapper := newUPnPPortMapper()
	if err := mapper.MapContext(context.Background(), settings); err != nil {
		t.Fatalf("map ports: %v", err)
	}
	if err := mapper.Close(); err != nil {
		t.Fatalf("close mapper: %v", err)
	}

	wantAdds := []string{"add:TCP:4661", "add:UDP:4662"}
	if got := deviceA.actions; len(got) != len(wantAdds) || got[0] != wantAdds[0] || got[1] != wantAdds[1] {
		t.Fatalf("unexpected gateway A add actions: %#v", got)
	}
	if got := deviceB.actions; len(got) != len(wantAdds) || got[0] != wantAdds[0] || got[1] != wantAdds[1] {
		t.Fatalf("unexpected gateway B add actions: %#v", got)
	}
	wantDeletes := []string{"delete:TCP:4661", "delete:UDP:4662"}
	if got := deviceA.deleteActions; len(got) != len(wantDeletes) || got[0] != wantDeletes[0] || got[1] != wantDeletes[1] {
		t.Fatalf("unexpected gateway A delete actions: %#v", got)
	}
	if got := deviceB.deleteActions; len(got) != len(wantDeletes) || got[0] != wantDeletes[0] || got[1] != wantDeletes[1] {
		t.Fatalf("unexpected gateway B delete actions: %#v", got)
	}
}

func TestUPnPPortMapperReturnsErrorWhenNoGatewayDiscovered(t *testing.T) {
	previousDiscover := discoverUPnPDevices
	defer func() {
		discoverUPnPDevices = previousDiscover
	}()
	discoverUPnPDevices = func(time.Duration) []internalupnp.Device { return nil }

	settings := NewSettings()
	settings.EnableUPnP = true
	settings.ListenPort = 4661

	err := newUPnPPortMapper().MapContext(context.Background(), settings)
	if err == nil {
		t.Fatal("expected discover error")
	}
}

func TestSessionListenIgnoresUPnPFailureAndClosesMapper(t *testing.T) {
	previousFactory := newSessionPortMapper
	defer func() {
		newSessionPortMapper = previousFactory
	}()

	fake := &fakeSessionMapper{
		mapCalled:   make(chan struct{}),
		closeCalled: make(chan struct{}),
		mapErr:      errors.New("no upnp device"),
	}
	newSessionPortMapper = func() sessionPortMapper {
		return fake
	}

	settings := NewSettings()
	settings.EnableUPnP = true
	settings.ListenPort = freeTCPPort(t)

	session := NewSession(settings)
	if err := session.Listen(); err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	select {
	case <-fake.mapCalled:
	case <-time.After(time.Second):
		t.Fatal("expected upnp mapper to be invoked")
	}

	session.CloseListener()

	select {
	case <-fake.closeCalled:
	case <-time.After(time.Second):
		t.Fatal("expected upnp mapper close to be invoked")
	}
}

func TestDesiredUPnPPortMappings(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 4661
	settings.UDPPort = 4662
	settings.EnableDHT = true

	mappings := desiredUPnPPortMappings(settings)
	if len(mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(mappings))
	}
	if mappings[0].protocol != internalupnp.TCP || mappings[0].port != 4661 {
		t.Fatalf("unexpected tcp mapping: %+v", mappings[0])
	}
	if mappings[1].protocol != internalupnp.UDP || mappings[1].port != 4662 {
		t.Fatalf("unexpected udp mapping: %+v", mappings[1])
	}
}

type fakeUPnPDevice struct {
	id            string
	actions       []string
	deleteActions []string
}

func (d *fakeUPnPDevice) ID() string {
	return d.id
}

func (d *fakeUPnPDevice) GetLocalIPAddress() net.IP {
	return net.IPv4(192, 168, 1, 2)
}

func (d *fakeUPnPDevice) AddPortMapping(protocol internalupnp.Protocol, internalPort, externalPort int, description string, duration time.Duration) (int, error) {
	d.actions = append(d.actions, "add:"+string(protocol)+":"+strconv.Itoa(externalPort))
	return externalPort, nil
}

func (d *fakeUPnPDevice) DeletePortMapping(protocol internalupnp.Protocol, externalPort int) error {
	d.deleteActions = append(d.deleteActions, "delete:"+string(protocol)+":"+strconv.Itoa(externalPort))
	return nil
}

func (d *fakeUPnPDevice) GetExternalIPAddress() (net.IP, error) {
	return net.IPv4(1, 2, 3, 4), nil
}

type fakeSessionMapper struct {
	mapCalled   chan struct{}
	closeCalled chan struct{}
	mapErr      error
}

func (m *fakeSessionMapper) MapContext(context.Context, Settings) error {
	close(m.mapCalled)
	return m.mapErr
}

func (m *fakeSessionMapper) Close() error {
	close(m.closeCalled)
	return nil
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate tcp port: %v", err)
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("unexpected listener addr type")
	}
	return addr.Port
}
