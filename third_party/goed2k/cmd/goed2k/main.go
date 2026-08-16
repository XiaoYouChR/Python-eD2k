package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	ed2k "github.com/monkeyWie/goed2k"
)

const (
	// from https://www.emule-security.org/serverlist/
	defaultServerList = "45.82.80.155:5687,176.123.5.89:4725,85.121.5.137:4232,176.123.2.239:4232,145.239.2.134:4661,91.208.162.87:4232,37.15.61.236:4232"
	defaultServerMet  = "ed2k://|serverlist|http://upd.emule-security.org/server.met|/"
	// from https://www.nodes-dat.com/
	defaultNodesDat = "http://www.alldivx.de/nodes/nodes.dat,https://upd.emule-security.org/nodes.dat"
)

type runConfig struct {
	links         []string
	outDir        string
	serverAddr    string
	serverMetPath string
	listenPort    int
	udpPort       int
	enableKAD     bool
	enableUPnP    bool
	kadNodesDat   string
	kadNodes      string
	peerTimeout   int
	timeout       time.Duration
}

type appContext struct {
	client      *ed2k.Client
	targetPaths []string
	includeDHT  bool
	deadline    time.Time
	noticeCh    chan string
}

func main() {
	cfg := defaultRunConfig()

	app, err := setupClient(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer app.client.Close()

	message, err := runManagerTUI(app, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if strings.TrimSpace(message) != "" {
		fmt.Println(message)
	}
}

func defaultRunConfig() runConfig {
	return runConfig{
		outDir:        ".",
		serverAddr:    defaultServerList,
		serverMetPath: defaultServerMet,
		listenPort:    4661,
		udpPort:       4662,
		enableKAD:     true,
		enableUPnP:    true,
		kadNodesDat:   defaultNodesDat,
		peerTimeout:   30,
	}
}

func configureFileLogger() (*slog.Logger, error) {
	file, err := os.OpenFile("goed2k.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})), nil
}

func setupClient(cfg runConfig) (*appContext, error) {
	settings := ed2k.NewSettings()
	logger, err := configureFileLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init failed: %v\n", err)
	} else {
		settings.Logger = logger
	}
	settings.ReconnectToServer = true
	settings.ListenPort = cfg.listenPort
	settings.UDPPort = cfg.udpPort
	settings.EnableDHT = cfg.enableKAD
	settings.EnableUPnP = cfg.enableUPnP
	settings.PeerConnectionTimeout = cfg.peerTimeout

	client := ed2k.NewClient(settings)
	if cfg.enableKAD {
		client.EnableDHT()
	}
	if err := client.Start(); err != nil {
		return nil, fmt.Errorf("listen failed on port %d: %w", settings.ListenPort, err)
	}

	app := &appContext{
		client:      client,
		targetPaths: nil,
		includeDHT:  cfg.enableKAD,
		noticeCh:    make(chan string, 32),
	}
	if cfg.timeout > 0 {
		app.deadline = time.Now().Add(cfg.timeout)
	}
	startBackgroundBootstrap(app, cfg)
	return app, nil
}

func startBackgroundBootstrap(app *appContext, cfg runConfig) {
	if app == nil || app.client == nil {
		return
	}
	if cfg.serverAddr != "" {
		go connectServersBestEffort(app, splitCommaList(cfg.serverAddr))
	}
	if cfg.serverMetPath != "" {
		go loadServerMetBestEffort(app, splitCommaList(cfg.serverMetPath))
	}
	if cfg.enableKAD && cfg.kadNodesDat != "" {
		go func() {
			if err := app.client.LoadDHTNodesDat(cfg.kadNodesDat); err != nil {
				app.notify("KAD nodes.dat unavailable: %v", err)
			}
		}()
	}
	if cfg.enableKAD && cfg.kadNodes != "" {
		go func() {
			if err := app.client.AddDHTBootstrapNodes(cfg.kadNodes); err != nil {
				app.notify("KAD bootstrap nodes ignored: %v", err)
			}
		}()
	}
}

func loadServerMetBestEffort(app *appContext, sources []string) {
	for _, item := range sources {
		entries, err := app.client.LoadServerMet(item)
		if err != nil {
			app.notify("server.met unavailable: %v", err)
			continue
		}
		addrs := make([]string, 0, len(entries))
		for _, entry := range entries {
			if addr := entry.Address(); addr != "" {
				addrs = append(addrs, addr)
			}
		}
		connectServersBestEffort(app, addrs)
	}
}

func connectServersBestEffort(app *appContext, servers []string) {
	seen := make(map[string]struct{}, len(servers))
	for _, serverAddr := range servers {
		serverAddr = strings.TrimSpace(serverAddr)
		if serverAddr == "" {
			continue
		}
		if _, ok := seen[serverAddr]; ok {
			continue
		}
		seen[serverAddr] = struct{}{}
		if err := app.client.Connect(serverAddr); err != nil {
			app.notify("server unavailable: %s (%v)", serverAddr, err)
		}
	}
}

func (a *appContext) notify(format string, args ...any) {
	if a == nil || a.noticeCh == nil {
		return
	}
	message := fmt.Sprintf(format, args...)
	select {
	case a.noticeCh <- message:
	default:
	}
}

func (a *appContext) drainNotices() []string {
	if a == nil || a.noticeCh == nil {
		return nil
	}
	messages := make([]string, 0, 4)
	for {
		select {
		case msg := <-a.noticeCh:
			if strings.TrimSpace(msg) != "" {
				messages = append(messages, msg)
			}
		default:
			return messages
		}
	}
}

func splitCommaList(value string) []string {
	parts := make([]string, 0, 4)
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			parts = append(parts, item)
		}
	}
	return parts
}

func completionMessage(paths []string) string {
	if len(paths) == 1 {
		return fmt.Sprintf("completed: %s", paths[0])
	}
	if len(paths) > 1 {
		return fmt.Sprintf("completed %d transfers", len(paths))
	}
	return "completed"
}

func timeoutMessage(paths []string) string {
	if len(paths) == 1 {
		return fmt.Sprintf("stopped before completion: %s", paths[0])
	}
	if len(paths) > 1 {
		return fmt.Sprintf("stopped before completion: %d transfers", len(paths))
	}
	return "stopped before completion"
}
