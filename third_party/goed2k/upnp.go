package goed2k

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/monkeyWie/goed2k/internal/logx"
	internalupnp "github.com/monkeyWie/goed2k/internal/upnp"
)

const defaultUPnPDiscoverTimeout = 3 * time.Second

type sessionPortMapper interface {
	MapContext(context.Context, Settings) error
	Close() error
}

var newSessionPortMapper = func() sessionPortMapper {
	return newUPnPPortMapper()
}

var discoverUPnPDevices = func(timeout time.Duration) []internalupnp.Device {
	return internalupnp.Discover(0, timeout)
}

type upnpManager struct {
	mapper sessionPortMapper
	cancel context.CancelFunc
	done   chan struct{}
}

func (m *upnpManager) stop() {
	if m == nil {
		return
	}
	if m.cancel != nil {
		m.cancel()
	}
	if m.done != nil {
		<-m.done
	}
	if m.mapper != nil {
		_ = m.mapper.Close()
	}
}

type upnpPortMapper struct {
	mu       sync.Mutex
	mappings []mappedUPnPPort
}

type mappedUPnPPort struct {
	device   internalupnp.Device
	protocol internalupnp.Protocol
	port     int
}

type upnpPortMapping struct {
	protocol internalupnp.Protocol
	port     int
	name     string
}

func newUPnPPortMapper() *upnpPortMapper {
	return &upnpPortMapper{}
}

func (m *upnpPortMapper) MapContext(ctx context.Context, settings Settings) error {
	mappings := desiredUPnPPortMappings(settings)
	if len(mappings) == 0 {
		return nil
	}

	m.mu.Lock()
	if len(m.mappings) > 0 {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	timeout := defaultUPnPDiscoverTimeout
	if ctx != nil {
		if deadline, ok := ctx.Deadline(); ok {
			if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
				timeout = remaining
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	devices := discoverUPnPDevices(timeout)
	if len(devices) == 0 {
		return errors.New("no upnp gateway discovered")
	}

	mapped := make([]mappedUPnPPort, 0, len(devices)*len(mappings))
	for _, device := range devices {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		for _, mapping := range mappings {
			if _, err := device.AddPortMapping(mapping.protocol, mapping.port, mapping.port, mapping.name, 0); err != nil {
				logx.Warn("upnp port mapping failed", "gateway", device.ID(), "protocol", mapping.protocol, "port", mapping.port, "err", err)
				continue
			}
			mapped = append(mapped, mappedUPnPPort{
				device:   device,
				protocol: mapping.protocol,
				port:     mapping.port,
			})
			logx.Info("upnp port mapped", "gateway", device.ID(), "protocol", mapping.protocol, "port", mapping.port)
		}
	}
	if len(mapped) == 0 {
		return fmt.Errorf("upnp mapping failed for all ports on %d discovered gateways", len(devices))
	}

	m.mu.Lock()
	m.mappings = mapped
	m.mu.Unlock()
	return nil
}

func (m *upnpPortMapper) Close() error {
	m.mu.Lock()
	mappings := append([]mappedUPnPPort(nil), m.mappings...)
	m.mappings = nil
	m.mu.Unlock()

	var errs []error
	for _, mapping := range mappings {
		if err := mapping.device.DeletePortMapping(mapping.protocol, mapping.port); err != nil {
			logx.Warn("upnp port unmap failed", "gateway", mapping.device.ID(), "protocol", mapping.protocol, "port", mapping.port, "err", err)
			errs = append(errs, err)
			continue
		}
		logx.Info("upnp port unmapped", "gateway", mapping.device.ID(), "protocol", mapping.protocol, "port", mapping.port)
	}
	return errors.Join(errs...)
}

func desiredUPnPPortMappings(settings Settings) []upnpPortMapping {
	mappings := make([]upnpPortMapping, 0, 2)
	if settings.ListenPort > 0 {
		mappings = append(mappings, upnpPortMapping{
			protocol: internalupnp.TCP,
			port:     settings.ListenPort,
			name:     "goed2k TCP",
		})
	}
	if settings.EnableDHT && settings.UDPPort > 0 {
		mappings = append(mappings, upnpPortMapping{
			protocol: internalupnp.UDP,
			port:     settings.UDPPort,
			name:     "goed2k UDP",
		})
	}
	return mappings
}

func (s *Session) startUPnPMapping() {
	if !s.settings.EnableUPnP {
		return
	}

	s.mu.Lock()
	if s.upnp != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultUPnPDiscoverTimeout)
	manager := &upnpManager{
		mapper: newSessionPortMapper(),
		cancel: cancel,
		done:   make(chan struct{}),
	}
	settings := s.settings
	s.upnp = manager
	s.mu.Unlock()

	go func() {
		defer close(manager.done)
		if err := manager.mapper.MapContext(ctx, settings); err != nil && ctx.Err() == nil {
			logx.Warn("upnp unavailable", "err", err)
		}
	}()
}

func (s *Session) stopUPnPMapping() {
	s.mu.Lock()
	manager := s.upnp
	s.upnp = nil
	s.mu.Unlock()
	if manager != nil {
		manager.stop()
	}
}
