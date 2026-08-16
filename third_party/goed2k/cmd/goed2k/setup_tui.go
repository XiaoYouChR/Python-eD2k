package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	setupFocusOutDir = iota
	setupFocusServers
	setupFocusServerMet
	setupFocusKAD
	setupFocusUPnP
	setupFocusKADNodesDat
	setupFocusKADNodes
	setupFocusListenPort
	setupFocusUDPPort
	setupFocusPeerTimeout
	setupFocusTimeout
	setupFocusLinkInput
	setupFocusLinks
	setupFocusStart
	setupFocusCount
)

type setupFinishedMsg struct{}

type setupModel struct {
	outDirInput      textinput.Model
	serverInput      textinput.Model
	serverMetInput   textinput.Model
	kadNodesDatInput textinput.Model
	kadNodesInput    textinput.Model
	listenPortInput  textinput.Model
	udpPortInput     textinput.Model
	peerTimeoutInput textinput.Model
	timeoutInput     textinput.Model
	linkInput        textinput.Model

	cfg          runConfig
	focus        int
	selectedLink int
	status       string
	submitted    bool
}

func runSetupTUI(initial runConfig) (runConfig, error) {
	model := newSetupModel(initial)
	program := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return initial, err
	}
	result, ok := finalModel.(setupModel)
	if !ok {
		return initial, fmt.Errorf("unexpected setup result")
	}
	if !result.submitted {
		return initial, fmt.Errorf("setup aborted")
	}
	return result.cfg, nil
}

func newSetupModel(cfg runConfig) setupModel {
	m := setupModel{
		cfg: cfg,
	}
	m.outDirInput = newSetupInput("download directory", cfg.outDir)
	m.serverInput = newSetupInput("host:port,host:port", cfg.serverAddr)
	m.serverMetInput = newSetupInput("path/url/ed2k serverlist", cfg.serverMetPath)
	m.kadNodesDatInput = newSetupInput("path,url,path,url", cfg.kadNodesDat)
	m.kadNodesInput = newSetupInput("udp-host:port,udp-host:port", cfg.kadNodes)
	m.listenPortInput = newSetupInput("4661", strconv.Itoa(cfg.listenPort))
	m.udpPortInput = newSetupInput("4662", strconv.Itoa(cfg.udpPort))
	m.peerTimeoutInput = newSetupInput("30", strconv.Itoa(cfg.peerTimeout))
	timeoutValue := ""
	if cfg.timeout > 0 {
		timeoutValue = cfg.timeout.String()
	}
	m.timeoutInput = newSetupInput("0 or 30m", timeoutValue)
	m.linkInput = newSetupInput("paste ed2k link and press Enter", "")
	if len(m.cfg.links) == 0 {
		m.selectedLink = -1
	}
	m.syncFocus()
	return m
}

func newSetupInput(placeholder, value string) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.SetValue(value)
	input.CharLimit = 4096
	input.Width = 72
	return input
}

func (m setupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.focus = (m.focus + 1) % setupFocusCount
			m.syncFocus()
			return m, nil
		case "shift+tab":
			m.focus = (m.focus - 1 + setupFocusCount) % setupFocusCount
			m.syncFocus()
			return m, nil
		case "up":
			if m.focus == setupFocusLinks {
				m.moveSelectedLink(-1)
				return m, nil
			}
			m.focus = (m.focus - 1 + setupFocusCount) % setupFocusCount
			m.syncFocus()
			return m, nil
		case "down":
			if m.focus == setupFocusLinks {
				m.moveSelectedLink(1)
				return m, nil
			}
			m.focus = (m.focus + 1) % setupFocusCount
			m.syncFocus()
			return m, nil
		case " ":
			if m.focus == setupFocusKAD {
				m.cfg.enableKAD = !m.cfg.enableKAD
				return m, nil
			}
			if m.focus == setupFocusUPnP {
				m.cfg.enableUPnP = !m.cfg.enableUPnP
				return m, nil
			}
		case "enter":
			switch m.focus {
			case setupFocusKAD:
				m.cfg.enableKAD = !m.cfg.enableKAD
				return m, nil
			case setupFocusUPnP:
				m.cfg.enableUPnP = !m.cfg.enableUPnP
				return m, nil
			case setupFocusLinkInput:
				m.addLink()
				return m, nil
			case setupFocusStart:
				next, cmd := m.submit()
				return next, cmd
			}
		case "d", "backspace":
			if m.focus == setupFocusLinks {
				m.removeSelectedLink()
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	switch m.focus {
	case setupFocusOutDir:
		m.outDirInput, cmd = m.outDirInput.Update(msg)
	case setupFocusServers:
		m.serverInput, cmd = m.serverInput.Update(msg)
	case setupFocusServerMet:
		m.serverMetInput, cmd = m.serverMetInput.Update(msg)
	case setupFocusKADNodesDat:
		m.kadNodesDatInput, cmd = m.kadNodesDatInput.Update(msg)
	case setupFocusKADNodes:
		m.kadNodesInput, cmd = m.kadNodesInput.Update(msg)
	case setupFocusListenPort:
		m.listenPortInput, cmd = m.listenPortInput.Update(msg)
	case setupFocusUDPPort:
		m.udpPortInput, cmd = m.udpPortInput.Update(msg)
	case setupFocusPeerTimeout:
		m.peerTimeoutInput, cmd = m.peerTimeoutInput.Update(msg)
	case setupFocusTimeout:
		m.timeoutInput, cmd = m.timeoutInput.Update(msg)
	case setupFocusLinkInput:
		m.linkInput, cmd = m.linkInput.Update(msg)
	}
	return m, cmd
}

func (m setupModel) View() string {
	lines := []string{
		headerStyle.Render("goed2k setup"),
		"",
		m.renderField(setupFocusOutDir, "Output", m.outDirInput.View()),
		m.renderField(setupFocusServers, "Servers", m.serverInput.View()),
		m.renderField(setupFocusServerMet, "Server.met", m.serverMetInput.View()),
		m.renderToggle(setupFocusKAD, "KAD", m.cfg.enableKAD),
		m.renderToggle(setupFocusUPnP, "UPNP", m.cfg.enableUPnP),
		m.renderField(setupFocusKADNodesDat, "KAD nodes.dat", m.kadNodesDatInput.View()),
		m.renderField(setupFocusKADNodes, "KAD bootstrap", m.kadNodesInput.View()),
		m.renderField(setupFocusListenPort, "Listen port", m.listenPortInput.View()),
		m.renderField(setupFocusUDPPort, "UDP port", m.udpPortInput.View()),
		m.renderField(setupFocusPeerTimeout, "Peer timeout", m.peerTimeoutInput.View()),
		m.renderField(setupFocusTimeout, "Timeout", m.timeoutInput.View()),
		m.renderField(setupFocusLinkInput, "Add link", m.linkInput.View()),
		m.renderLinks(),
		m.renderStart(),
		"",
		footerStyle.Render("Tab/Shift+Tab move • Enter add/start • Space toggle KAD/UPNP • d delete link • q quit"),
	}
	if strings.TrimSpace(m.status) != "" {
		lines = append(lines, footerStyle.Render(m.status))
	}
	return strings.Join(lines, "\n")
}

func (m setupModel) renderField(focus int, label, value string) string {
	prefix := "  "
	style := lipgloss.NewStyle()
	if m.focus == focus {
		prefix = "> "
		style = style.Bold(true).Foreground(lipgloss.Color("69"))
	}
	return style.Render(fmt.Sprintf("%s%-12s %s", prefix, label, value))
}

func (m setupModel) renderToggle(focus int, label string, enabled bool) string {
	value := "off"
	if enabled {
		value = "on"
	}
	prefix := "  "
	style := lipgloss.NewStyle()
	if m.focus == focus {
		prefix = "> "
		style = style.Bold(true).Foreground(lipgloss.Color("69"))
	}
	return style.Render(fmt.Sprintf("%s%-12s [%s]", prefix, label, value))
}

func (m setupModel) renderLinks() string {
	prefix := "  "
	style := lipgloss.NewStyle()
	if m.focus == setupFocusLinks {
		prefix = "> "
		style = style.Bold(true).Foreground(lipgloss.Color("69"))
	}
	lines := []string{style.Render(fmt.Sprintf("%s%-12s %d", prefix, "Links", len(m.cfg.links)))}
	if len(m.cfg.links) == 0 {
		lines = append(lines, "               no links added")
		return strings.Join(lines, "\n")
	}
	for i, link := range m.cfg.links {
		linePrefix := "               "
		if m.focus == setupFocusLinks && i == m.selectedLink {
			linePrefix = "             * "
		}
		lines = append(lines, linePrefix+trimString(link, 96))
	}
	return strings.Join(lines, "\n")
}

func (m setupModel) renderStart() string {
	label := "[ Start downloads ]"
	if m.focus == setupFocusStart {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Padding(0, 1).Render(label)
	}
	return "  " + label
}

func (m *setupModel) syncFocus() {
	inputs := []*textinput.Model{
		&m.outDirInput,
		&m.serverInput,
		&m.serverMetInput,
		&m.kadNodesDatInput,
		&m.kadNodesInput,
		&m.listenPortInput,
		&m.udpPortInput,
		&m.peerTimeoutInput,
		&m.timeoutInput,
		&m.linkInput,
	}
	for _, input := range inputs {
		input.Blur()
	}
	switch m.focus {
	case setupFocusOutDir:
		m.outDirInput.Focus()
	case setupFocusServers:
		m.serverInput.Focus()
	case setupFocusServerMet:
		m.serverMetInput.Focus()
	case setupFocusKADNodesDat:
		m.kadNodesDatInput.Focus()
	case setupFocusKADNodes:
		m.kadNodesInput.Focus()
	case setupFocusListenPort:
		m.listenPortInput.Focus()
	case setupFocusUDPPort:
		m.udpPortInput.Focus()
	case setupFocusPeerTimeout:
		m.peerTimeoutInput.Focus()
	case setupFocusTimeout:
		m.timeoutInput.Focus()
	case setupFocusLinkInput:
		m.linkInput.Focus()
	case setupFocusLinks:
		if len(m.cfg.links) == 0 {
			m.selectedLink = -1
		} else if m.selectedLink < 0 || m.selectedLink >= len(m.cfg.links) {
			m.selectedLink = len(m.cfg.links) - 1
		}
	}
}

func (m *setupModel) moveSelectedLink(delta int) {
	if len(m.cfg.links) == 0 {
		m.selectedLink = -1
		return
	}
	if m.selectedLink < 0 {
		m.selectedLink = 0
		return
	}
	m.selectedLink += delta
	if m.selectedLink < 0 {
		m.selectedLink = 0
	}
	if m.selectedLink >= len(m.cfg.links) {
		m.selectedLink = len(m.cfg.links) - 1
	}
}

func (m *setupModel) addLink() {
	link := strings.TrimSpace(m.linkInput.Value())
	if link == "" {
		m.status = "link is empty"
		return
	}
	m.cfg.links = append(m.cfg.links, link)
	m.selectedLink = len(m.cfg.links) - 1
	m.linkInput.SetValue("")
	m.status = fmt.Sprintf("added link #%d", len(m.cfg.links))
}

func (m *setupModel) removeSelectedLink() {
	if len(m.cfg.links) == 0 || m.selectedLink < 0 || m.selectedLink >= len(m.cfg.links) {
		m.status = "no link selected"
		return
	}
	removed := m.cfg.links[m.selectedLink]
	m.cfg.links = append(m.cfg.links[:m.selectedLink], m.cfg.links[m.selectedLink+1:]...)
	if len(m.cfg.links) == 0 {
		m.selectedLink = -1
	} else if m.selectedLink >= len(m.cfg.links) {
		m.selectedLink = len(m.cfg.links) - 1
	}
	m.status = "removed " + trimString(removed, 80)
}

func (m *setupModel) submit() (setupModel, tea.Cmd) {
	cfg, err := m.currentConfig()
	if err != nil {
		m.status = err.Error()
		return *m, nil
	}
	m.cfg = cfg
	m.submitted = true
	return *m, tea.Quit
}

func (m setupModel) currentConfig() (runConfig, error) {
	cfg := m.cfg
	cfg.outDir = strings.TrimSpace(m.outDirInput.Value())
	if cfg.outDir == "" {
		cfg.outDir = "."
	}
	cfg.serverAddr = strings.TrimSpace(m.serverInput.Value())
	cfg.serverMetPath = strings.TrimSpace(m.serverMetInput.Value())
	cfg.kadNodesDat = strings.TrimSpace(m.kadNodesDatInput.Value())
	cfg.kadNodes = strings.TrimSpace(m.kadNodesInput.Value())
	listenPort, err := parseIntField("listen port", m.listenPortInput.Value())
	if err != nil {
		return cfg, err
	}
	udpPort, err := parseIntField("udp port", m.udpPortInput.Value())
	if err != nil {
		return cfg, err
	}
	peerTimeout, err := parseIntField("peer timeout", m.peerTimeoutInput.Value())
	if err != nil {
		return cfg, err
	}
	timeout, err := parseDurationField(m.timeoutInput.Value())
	if err != nil {
		return cfg, err
	}
	cfg.listenPort = listenPort
	cfg.udpPort = udpPort
	cfg.peerTimeout = peerTimeout
	cfg.timeout = timeout
	if len(cfg.links) == 0 {
		return cfg, fmt.Errorf("add at least one ed2k link")
	}
	return cfg, nil
}

func parseIntField(name, value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("%s is empty", name)
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return parsed, nil
}

func parseDurationField(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "0" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout: %w", err)
	}
	return parsed, nil
}

type newTaskConfig struct {
	link   string
	outDir string
}

type newTaskModel struct {
	linkInput   textinput.Model
	outDirInput textinput.Model
	focus       int
	status      string
	submitted   bool
	result      newTaskConfig
}

func runNewTaskTUI(defaultOutDir string) (newTaskConfig, error) {
	model := newNewTaskModel(defaultOutDir)
	program := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return newTaskConfig{}, err
	}
	result, ok := finalModel.(newTaskModel)
	if !ok {
		return newTaskConfig{}, fmt.Errorf("unexpected new task result")
	}
	if !result.submitted {
		return newTaskConfig{}, fmt.Errorf("new task aborted")
	}
	return result.result, nil
}

func newNewTaskModel(defaultOutDir string) newTaskModel {
	m := newTaskModel{
		linkInput:   newSetupInput("paste ed2k file link", ""),
		outDirInput: newSetupInput("download directory", defaultOutDir),
	}
	m.linkInput.Focus()
	return m
}

func (m newTaskModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m newTaskModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "tab", "down":
			m.focus = (m.focus + 1) % 3
			m.syncFocus()
			return m, nil
		case "shift+tab", "up":
			m.focus = (m.focus - 1 + 3) % 3
			m.syncFocus()
			return m, nil
		case "enter":
			if m.focus == 2 {
				next, cmd := m.submit()
				return next, cmd
			}
			m.focus = (m.focus + 1) % 3
			m.syncFocus()
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.focus {
	case 0:
		m.linkInput, cmd = m.linkInput.Update(msg)
	case 1:
		m.outDirInput, cmd = m.outDirInput.Update(msg)
	}
	return m, cmd
}

func (m newTaskModel) View() string {
	lines := []string{
		headerStyle.Render("new transfer"),
		"",
		m.renderField(0, "Link", m.linkInput.View()),
		m.renderField(1, "Output", m.outDirInput.View()),
		m.renderStart(2, "[ Add transfer ]"),
		"",
		footerStyle.Render("Tab/Shift+Tab move • Enter confirm • q quit"),
	}
	if strings.TrimSpace(m.status) != "" {
		lines = append(lines, footerStyle.Render(m.status))
	}
	return strings.Join(lines, "\n")
}

func (m newTaskModel) renderField(focus int, label, value string) string {
	prefix := "  "
	style := lipgloss.NewStyle()
	if m.focus == focus {
		prefix = "> "
		style = style.Bold(true).Foreground(lipgloss.Color("69"))
	}
	return style.Render(fmt.Sprintf("%s%-8s %s", prefix, label, value))
}

func (m newTaskModel) renderStart(focus int, label string) string {
	if m.focus == focus {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Padding(0, 1).Render(label)
	}
	return "  " + label
}

func (m *newTaskModel) syncFocus() {
	m.linkInput.Blur()
	m.outDirInput.Blur()
	switch m.focus {
	case 0:
		m.linkInput.Focus()
	case 1:
		m.outDirInput.Focus()
	}
}

func (m *newTaskModel) submit() (newTaskModel, tea.Cmd) {
	link := strings.TrimSpace(m.linkInput.Value())
	outDir := strings.TrimSpace(m.outDirInput.Value())
	if link == "" {
		m.status = "link is empty"
		return *m, nil
	}
	if outDir == "" {
		outDir = "."
	}
	m.submitted = true
	m.result = newTaskConfig{
		link:   link,
		outDir: outDir,
	}
	return *m, tea.Quit
}

type settingsModel struct {
	outDirInput      textinput.Model
	serverInput      textinput.Model
	serverMetInput   textinput.Model
	kadNodesDatInput textinput.Model
	kadNodesInput    textinput.Model
	listenPortInput  textinput.Model
	udpPortInput     textinput.Model
	peerTimeoutInput textinput.Model
	timeoutInput     textinput.Model
	cfg              runConfig
	focus            int
	status           string
	submitted        bool
}

func runSettingsTUI(initial runConfig) (runConfig, error) {
	model := newSettingsModel(initial)
	program := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return initial, err
	}
	result, ok := finalModel.(settingsModel)
	if !ok {
		return initial, fmt.Errorf("unexpected settings result")
	}
	if !result.submitted {
		return initial, fmt.Errorf("settings aborted")
	}
	return result.cfg, nil
}

func newSettingsModel(cfg runConfig) settingsModel {
	m := settingsModel{
		cfg: cfg,
	}
	m.outDirInput = newSetupInput("download directory", cfg.outDir)
	m.serverInput = newSetupInput("host:port,host:port", cfg.serverAddr)
	m.serverMetInput = newSetupInput("path/url/ed2k serverlist", cfg.serverMetPath)
	m.kadNodesDatInput = newSetupInput("path,url,path,url", cfg.kadNodesDat)
	m.kadNodesInput = newSetupInput("udp-host:port,udp-host:port", cfg.kadNodes)
	m.listenPortInput = newSetupInput("4661", strconv.Itoa(cfg.listenPort))
	m.udpPortInput = newSetupInput("4662", strconv.Itoa(cfg.udpPort))
	m.peerTimeoutInput = newSetupInput("30", strconv.Itoa(cfg.peerTimeout))
	timeoutValue := ""
	if cfg.timeout > 0 {
		timeoutValue = cfg.timeout.String()
	}
	m.timeoutInput = newSetupInput("0 or 30m", timeoutValue)
	m.syncFocus()
	return m
}

func (m settingsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m settingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "tab", "down":
			m.focus = (m.focus + 1) % 12
			m.syncFocus()
			return m, nil
		case "shift+tab", "up":
			m.focus = (m.focus - 1 + 12) % 12
			m.syncFocus()
			return m, nil
		case " ":
			if m.focus == 3 {
				m.cfg.enableKAD = !m.cfg.enableKAD
				return m, nil
			}
			if m.focus == 4 {
				m.cfg.enableUPnP = !m.cfg.enableUPnP
				return m, nil
			}
		case "enter":
			if m.focus == 11 {
				next, cmd := m.submit()
				return next, cmd
			}
			if m.focus == 3 {
				m.cfg.enableKAD = !m.cfg.enableKAD
				return m, nil
			}
			if m.focus == 4 {
				m.cfg.enableUPnP = !m.cfg.enableUPnP
				return m, nil
			}
			m.focus = (m.focus + 1) % 12
			m.syncFocus()
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.focus {
	case 0:
		m.outDirInput, cmd = m.outDirInput.Update(msg)
	case 1:
		m.serverInput, cmd = m.serverInput.Update(msg)
	case 2:
		m.serverMetInput, cmd = m.serverMetInput.Update(msg)
	case 5:
		m.kadNodesDatInput, cmd = m.kadNodesDatInput.Update(msg)
	case 6:
		m.kadNodesInput, cmd = m.kadNodesInput.Update(msg)
	case 7:
		m.listenPortInput, cmd = m.listenPortInput.Update(msg)
	case 8:
		m.udpPortInput, cmd = m.udpPortInput.Update(msg)
	case 9:
		m.peerTimeoutInput, cmd = m.peerTimeoutInput.Update(msg)
	case 10:
		m.timeoutInput, cmd = m.timeoutInput.Update(msg)
	}
	return m, cmd
}

func (m settingsModel) View() string {
	lines := []string{
		headerStyle.Render("settings"),
		"",
		m.renderField(0, "Output", m.outDirInput.View()),
		m.renderField(1, "Servers", m.serverInput.View()),
		m.renderField(2, "Server.met", m.serverMetInput.View()),
		m.renderToggle(3, "KAD", m.cfg.enableKAD),
		m.renderToggle(4, "UPNP", m.cfg.enableUPnP),
		m.renderField(5, "KAD nodes.dat", m.kadNodesDatInput.View()),
		m.renderField(6, "KAD bootstrap", m.kadNodesInput.View()),
		m.renderField(7, "Listen port", m.listenPortInput.View()),
		m.renderField(8, "UDP port", m.udpPortInput.View()),
		m.renderField(9, "Peer timeout", m.peerTimeoutInput.View()),
		m.renderField(10, "Timeout", m.timeoutInput.View()),
		m.renderStart(11, "[ Save settings ]"),
		"",
		footerStyle.Render("Tab/Shift+Tab move • Space toggle KAD/UPNP • Enter save • q quit"),
	}
	if strings.TrimSpace(m.status) != "" {
		lines = append(lines, footerStyle.Render(m.status))
	}
	return strings.Join(lines, "\n")
}

func (m settingsModel) renderField(focus int, label, value string) string {
	prefix := "  "
	style := lipgloss.NewStyle()
	if m.focus == focus {
		prefix = "> "
		style = style.Bold(true).Foreground(lipgloss.Color("69"))
	}
	return style.Render(fmt.Sprintf("%s%-12s %s", prefix, label, value))
}

func (m settingsModel) renderToggle(focus int, label string, enabled bool) string {
	value := "off"
	if enabled {
		value = "on"
	}
	prefix := "  "
	style := lipgloss.NewStyle()
	if m.focus == focus {
		prefix = "> "
		style = style.Bold(true).Foreground(lipgloss.Color("69"))
	}
	return style.Render(fmt.Sprintf("%s%-12s [%s]", prefix, label, value))
}

func (m settingsModel) renderStart(focus int, label string) string {
	if m.focus == focus {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Padding(0, 1).Render(label)
	}
	return "  " + label
}

func (m *settingsModel) syncFocus() {
	inputs := []*textinput.Model{
		&m.outDirInput,
		&m.serverInput,
		&m.serverMetInput,
		&m.kadNodesDatInput,
		&m.kadNodesInput,
		&m.listenPortInput,
		&m.udpPortInput,
		&m.peerTimeoutInput,
		&m.timeoutInput,
	}
	for _, input := range inputs {
		input.Blur()
	}
	switch m.focus {
	case 0:
		m.outDirInput.Focus()
	case 1:
		m.serverInput.Focus()
	case 2:
		m.serverMetInput.Focus()
	case 5:
		m.kadNodesDatInput.Focus()
	case 6:
		m.kadNodesInput.Focus()
	case 7:
		m.listenPortInput.Focus()
	case 8:
		m.udpPortInput.Focus()
	case 9:
		m.peerTimeoutInput.Focus()
	case 10:
		m.timeoutInput.Focus()
	}
}

func (m *settingsModel) submit() (settingsModel, tea.Cmd) {
	cfg := m.cfg
	cfg.outDir = strings.TrimSpace(m.outDirInput.Value())
	if cfg.outDir == "" {
		cfg.outDir = "."
	}
	cfg.serverAddr = strings.TrimSpace(m.serverInput.Value())
	cfg.serverMetPath = strings.TrimSpace(m.serverMetInput.Value())
	cfg.kadNodesDat = strings.TrimSpace(m.kadNodesDatInput.Value())
	cfg.kadNodes = strings.TrimSpace(m.kadNodesInput.Value())
	listenPort, err := parseIntField("listen port", m.listenPortInput.Value())
	if err != nil {
		m.status = err.Error()
		return *m, nil
	}
	udpPort, err := parseIntField("udp port", m.udpPortInput.Value())
	if err != nil {
		m.status = err.Error()
		return *m, nil
	}
	peerTimeout, err := parseIntField("peer timeout", m.peerTimeoutInput.Value())
	if err != nil {
		m.status = err.Error()
		return *m, nil
	}
	timeout, err := parseDurationField(m.timeoutInput.Value())
	if err != nil {
		m.status = err.Error()
		return *m, nil
	}
	cfg.listenPort = listenPort
	cfg.udpPort = udpPort
	cfg.peerTimeout = peerTimeout
	cfg.timeout = timeout
	m.cfg = cfg
	m.submitted = true
	return *m, tea.Quit
}
