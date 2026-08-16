package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	ed2k "github.com/monkeyWie/goed2k"
)

type statusMsg ed2k.ClientStatusEvent

type managerAction string
type managerPage string

const (
	managerPageTransfers managerPage = "transfers"
	managerPageSearch    managerPage = "search"
)

const (
	managerActionNone     managerAction = ""
	managerActionQuit     managerAction = "quit"
	managerActionNew      managerAction = "new"
	managerActionSettings managerAction = "settings"
)

type tuiModel struct {
	app           *appContext
	client        *ed2k.Client
	cfg           runConfig
	targetPaths   []string
	includeDHT    bool
	deadline      time.Time
	width         int
	height        int
	status        ed2k.ClientStatus
	transfers     []ed2k.TransferSnapshot
	selectedHash  string
	table         table.Model
	statusMessage string
	quitMessage   string
	commandInput  textinput.Model
	commandMode   bool
	nextAction    managerAction
	statusEvents  <-chan ed2k.ClientStatusEvent
	search        ed2k.SearchSnapshot
	page          managerPage
	searchInput   textinput.Model
	searchTable   table.Model
	searchFocus   int
}

func runManagerTUI(app *appContext, cfg runConfig) (string, error) {
	cfg.links = nil
	for {
		result, err := runManagerScreen(app, cfg)
		if err != nil {
			return "", err
		}
		switch result.nextAction {
		case managerActionNew:
			task, err := runNewTaskTUI(cfg.outDir)
			if err != nil {
				continue
			}
			_, targetPath, err := app.client.AddLink(task.link, task.outDir)
			if err != nil {
				return "", err
			}
			cfg.outDir = task.outDir
			app.targetPaths = append(app.targetPaths, targetPath)
		case managerActionSettings:
			if len(app.client.Status().Transfers) > 0 {
				continue
			}
			nextCfg, err := runSettingsTUI(cfg)
			if err != nil {
				continue
			}
			app.client.Close()
			app, err = setupClient(nextCfg)
			if err != nil {
				return "", err
			}
			cfg = nextCfg
		default:
			return result.quitMessage, nil
		}
	}
}

func runManagerScreen(app *appContext, cfg runConfig) (tuiModel, error) {
	events, cancel := app.client.SubscribeStatus()
	defer cancel()

	model := newTUIModel(app, cfg, events)
	program := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return tuiModel{}, err
	}
	result, ok := finalModel.(tuiModel)
	if !ok {
		return tuiModel{}, fmt.Errorf("unexpected manager result")
	}
	return result, nil
}

func newTUIModel(app *appContext, cfg runConfig, events <-chan ed2k.ClientStatusEvent) tuiModel {
	commandInput := textinput.New()
	commandInput.Placeholder = "/new or /setting"
	commandInput.CharLimit = 256
	commandInput.Width = 48
	searchInput := textinput.New()
	searchInput.Placeholder = "search keywords and press Enter"
	searchInput.CharLimit = 256
	searchInput.Width = 64
	m := tuiModel{
		app:          app,
		client:       app.client,
		cfg:          cfg,
		targetPaths:  append([]string(nil), app.targetPaths...),
		includeDHT:   app.includeDHT,
		deadline:     app.deadline,
		status:       app.client.Status(),
		search:       app.client.SearchSnapshot(),
		commandInput: commandInput,
		searchInput:  searchInput,
		statusEvents: events,
		page:         managerPageTransfers,
	}
	m.table = newTransferTable()
	m.searchTable = newSearchTable()
	m.syncTransfers()
	m.syncSearchResults()
	return m
}

func newTransferTable() table.Model {
	columns := []table.Column{
		{Title: "Name", Width: 28},
		{Title: "State", Width: 12},
		{Title: "Done", Width: 8},
		{Title: "Recv", Width: 8},
		{Title: "Rate", Width: 10},
		{Title: "Peers", Width: 5},
		{Title: "Act", Width: 4},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(nil),
		table.WithFocused(true),
		table.WithHeight(12),
	)
	t.SetStyles(defaultTableStyles())
	return t
}

func newSearchTable() table.Model {
	columns := []table.Column{
		{Title: "#", Width: 4},
		{Title: "Name", Width: 34},
		{Title: "Size", Width: 10},
		{Title: "Src", Width: 10},
		{Title: "Sources", Width: 8},
		{Title: "Complete", Width: 8},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(nil),
		table.WithFocused(true),
		table.WithHeight(12),
	)
	t.SetStyles(defaultTableStyles())
	return t
}

func defaultTableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("62")).
		Bold(true)
	return styles
}

func (m tuiModel) Init() tea.Cmd {
	return waitStatusCmd(m.statusEvents)
}

func waitStatusCmd(events <-chan ed2k.ClientStatusEvent) tea.Cmd {
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return nil
		}
		return statusMsg(event)
	}
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.consumeNotices()
		m.status = msg.Status
		m.search = m.client.SearchSnapshot()
		m.syncTransfers()
		m.syncSearchResults()
		return m, waitStatusCmd(m.statusEvents)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil
	case tea.KeyMsg:
		if m.page == managerPageSearch {
			return m.updateSearchPage(msg)
		}
		if m.commandMode {
			switch msg.String() {
			case "esc":
				m.commandMode = false
				m.commandInput.Blur()
				m.commandInput.SetValue("")
				return m, nil
			case "enter":
				return m.executeCommand()
			}
			var cmd tea.Cmd
			m.commandInput, cmd = m.commandInput.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.commandMode = true
			m.commandInput.SetValue("/")
			m.commandInput.Focus()
			return m, nil
		case "p":
			m.toggleSelectedPause()
			m.status = m.client.Status()
			m.search = m.client.SearchSnapshot()
			m.syncTransfers()
			return m, nil
		case "x":
			m.removeSelectedTransfer()
			m.status = m.client.Status()
			m.search = m.client.SearchSnapshot()
			m.syncTransfers()
			if len(m.status.Transfers) == 0 {
				m.statusMessage = "all transfers removed"
			}
			return m, nil
		case "r":
			m.consumeNotices()
			m.status = m.client.Status()
			m.search = m.client.SearchSnapshot()
			m.syncTransfers()
			return m, nil
		}

		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		m.updateSelectionFromCursor()
		return m, cmd
	}
	return m, nil
}

func (m tuiModel) updateSearchPage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.page = managerPageTransfers
		m.searchFocus = 0
		m.searchInput.Blur()
		m.table = newTransferTable()
		m.resize()
		m.status = m.client.Status()
		m.syncTransfers()
		return m, nil
	case "tab", "shift+tab":
		if m.searchFocus == 0 {
			m.searchFocus = 1
			m.searchInput.Blur()
		} else {
			m.searchFocus = 0
			m.searchInput.Focus()
		}
		return m, nil
	case "enter":
		if m.searchFocus == 0 {
			query := strings.TrimSpace(m.searchInput.Value())
			if query == "" {
				m.statusMessage = "search query is empty"
				return m, nil
			}
			handle, err := m.client.StartSearch(ed2k.SearchParams{
				Query: query,
				Scope: ed2k.SearchScopeAll,
			})
			if err != nil {
				m.statusMessage = err.Error()
				return m, nil
			}
			m.search = handle.Snapshot()
			m.syncSearchResults()
			m.statusMessage = "search started: " + query
			return m, nil
		}
		if err := m.downloadSelectedSearchResult(); err != nil {
			m.statusMessage = err.Error()
			return m, nil
		}
		return m, nil
	}

	if m.searchFocus == 0 {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "d":
		if err := m.downloadSelectedSearchResult(); err != nil {
			m.statusMessage = err.Error()
			return m, nil
		}
		return m, nil
	case "r":
		m.search = m.client.SearchSnapshot()
		m.syncSearchResults()
		return m, nil
	case "s":
		if err := m.client.StopSearch(); err != nil {
			m.statusMessage = err.Error()
			return m, nil
		}
		m.search = m.client.SearchSnapshot()
		m.syncSearchResults()
		m.statusMessage = "search stopped"
		return m, nil
	}

	var cmd tea.Cmd
	m.searchTable, cmd = m.searchTable.Update(msg)
	return m, cmd
}

func (m *tuiModel) consumeNotices() {
	if m.app == nil {
		return
	}
	notes := m.app.drainNotices()
	if len(notes) == 0 {
		return
	}
	m.statusMessage = notes[len(notes)-1]
}

func (m tuiModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}
	if m.page == managerPageSearch {
		return m.renderSearchPage()
	}
	header := m.renderHeader()
	leftWidth := maxInt(42, m.width/2-1)
	rightWidth := maxInt(40, m.width-leftWidth-3)

	leftPane := panelStyle(leftWidth).Render(m.table.View())
	rightPane := panelStyle(rightWidth).Render(m.renderDetails(rightWidth - 2))

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	baseFooter := "keys: ↑/↓ or j/k move • / command • p pause/resume • x remove • r refresh • q quit • /search resources"
	footer := footerStyle.Render(baseFooter)
	if strings.TrimSpace(m.statusMessage) != "" {
		footer = footerStyle.Render(m.statusMessage + "    " + baseFooter)
	}
	if m.commandMode {
		footer = footerStyle.Render("command: " + m.commandInput.View() + "    available: /new /search /setting /quit")
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m tuiModel) renderSearchPage() string {
	header := headerStyle.Render("goed2k search")
	inputLabel := "query"
	if m.searchFocus == 0 {
		inputLabel = "query *"
	}
	resultsLabel := "results"
	if m.searchFocus == 1 {
		resultsLabel = "results *"
	}
	info := []string{
		fmt.Sprintf("%s: %s", inputLabel, m.searchInput.View()),
		fmt.Sprintf("state=%s  scope=%s  total=%d", m.search.State, searchScopeLabel(m.search.Params.Scope), len(m.search.Results)),
	}
	if result := m.selectedSearchResult(); result != nil {
		info = append(info, fmt.Sprintf("selected=%s", emptyFallback(result.FileName, result.Hash.String())))
		if link := result.ED2KLink(); link != "" {
			info = append(info, "ed2k="+trimString(link, maxInt(24, m.width-16)))
		}
	}
	if m.search.KadKeyword != "" {
		info = append(info, fmt.Sprintf("kad_keyword=%q", m.search.KadKeyword))
	}
	if strings.TrimSpace(m.search.Error) != "" {
		info = append(info, "error="+m.search.Error)
	}
	searchPane := panelStyle(maxInt(80, m.width-2)).Render(strings.Join(info, "\n"))
	resultsPane := panelStyle(maxInt(80, m.width-2)).Render(titleStyle.Render(resultsLabel) + "\n" + m.searchTable.View())
	footer := footerStyle.Render("Enter search/download • Tab switch focus • d download selected • s stop search • Esc back • q quit")
	if strings.TrimSpace(m.statusMessage) != "" {
		footer = footerStyle.Render(m.statusMessage + "    Enter search/download • Tab switch focus • d download selected • s stop search • Esc back • q quit")
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, searchPane, resultsPane, footer)
}

func (m *tuiModel) syncTransfers() {
	transfers := append([]ed2k.TransferSnapshot(nil), m.status.Transfers...)
	sort.Slice(transfers, func(i, j int) bool {
		return transfers[i].CreateTime < transfers[j].CreateTime
	})
	m.transfers = transfers
	rows := make([]table.Row, 0, len(transfers))
	selectedCursor := 0
	for i, transfer := range transfers {
		if transfer.Hash.String() == m.selectedHash {
			selectedCursor = i
		}
		rows = append(rows, table.Row{
			trimString(transfer.FileName, 28),
			string(transfer.Status.State),
			fmt.Sprintf("%.1f%%", percent(transfer.Status.TotalDone, transfer.Status.TotalWanted)),
			fmt.Sprintf("%.1f%%", percent(transfer.Status.TotalReceived, transfer.Status.TotalWanted)),
			humanRate(transfer.Status.DownloadRate),
			fmt.Sprintf("%d", transfer.Status.NumPeers),
			fmt.Sprintf("%d", transfer.ActivePeers),
		})
	}
	m.table.SetRows(rows)
	if len(rows) == 0 {
		m.selectedHash = ""
		m.table.SetCursor(0)
		return
	}
	if m.selectedHash == "" {
		selectedCursor = minInt(len(rows)-1, m.table.Cursor())
	}
	m.table.SetCursor(selectedCursor)
	m.updateSelectionFromCursor()
}

func (m *tuiModel) syncSearchResults() {
	rows := make([]table.Row, 0, len(m.search.Results))
	for i, result := range m.search.Results {
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", i+1),
			trimString(emptyFallback(result.FileName, result.Hash.String()), 34),
			humanSize(result.FileSize),
			searchResultSourceLabel(result.Source),
			fmt.Sprintf("%d", result.Sources),
			fmt.Sprintf("%d", result.CompleteSources),
		})
	}
	m.searchTable.SetRows(rows)
	if len(rows) == 0 {
		m.searchTable.SetCursor(0)
		return
	}
	if m.searchTable.Cursor() >= len(rows) {
		m.searchTable.SetCursor(len(rows) - 1)
	}
}

func (m *tuiModel) updateSelectionFromCursor() {
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(m.transfers) {
		return
	}
	m.selectedHash = m.transfers[cursor].Hash.String()
}

func (m *tuiModel) resize() {
	if m.height <= 0 {
		return
	}
	m.table.SetHeight(maxInt(8, m.height-6))
	m.table.SetWidth(maxInt(42, m.width/2-2))
	m.searchTable.SetHeight(maxInt(8, m.height-10))
	m.searchTable.SetWidth(maxInt(80, m.width-6))
}

func (m tuiModel) executeCommand() (tea.Model, tea.Cmd) {
	command := strings.TrimSpace(m.commandInput.Value())
	m.commandMode = false
	m.commandInput.Blur()
	m.commandInput.SetValue("")

	switch command {
	case "/new":
		m.nextAction = managerActionNew
		return m, tea.Quit
	case "/search":
		m.page = managerPageSearch
		m.searchFocus = 0
		m.searchInput.Focus()
		m.searchTable = newSearchTable()
		m.resize()
		m.search = m.client.SearchSnapshot()
		m.syncSearchResults()
		m.statusMessage = ""
		return m, nil
	case "/setting", "/settings":
		if len(m.status.Transfers) > 0 {
			m.statusMessage = "settings are locked after transfers start"
			return m, nil
		}
		m.nextAction = managerActionSettings
		return m, tea.Quit
	case "/quit", "/exit":
		m.nextAction = managerActionQuit
		return m, tea.Quit
	case "/search-stop", "/searchstop", "/search stop":
		if err := m.client.StopSearch(); err != nil {
			m.statusMessage = err.Error()
			return m, nil
		}
		m.search = m.client.SearchSnapshot()
		m.syncSearchResults()
		m.statusMessage = "search stopped"
		return m, nil
	case "":
		return m, nil
	default:
		m.statusMessage = "unknown command: " + command
		return m, nil
	}
}

func (m *tuiModel) downloadSearchResult(index int) error {
	if index < 0 || index >= len(m.search.Results) {
		return fmt.Errorf("search result %d out of range", index+1)
	}
	link := m.search.Results[index].ED2KLink()
	if link == "" {
		return fmt.Errorf("search result %d has no valid ed2k link", index+1)
	}
	_, targetPath, err := m.client.AddLink(link, m.cfg.outDir)
	if err != nil {
		return err
	}
	m.targetPaths = append(m.targetPaths, targetPath)
	m.status = m.client.Status()
	m.search = m.client.SearchSnapshot()
	m.table = newTransferTable()
	m.resize()
	m.syncTransfers()
	m.syncSearchResults()
	m.statusMessage = "added " + emptyFallback(m.search.Results[index].FileName, m.search.Results[index].Hash.String())
	return nil
}

func (m *tuiModel) downloadSelectedSearchResult() error {
	cursor := m.searchTable.Cursor()
	return m.downloadSearchResult(cursor)
}

func (m tuiModel) selectedSearchResult() *ed2k.SearchResult {
	cursor := m.searchTable.Cursor()
	if cursor < 0 || cursor >= len(m.search.Results) {
		return nil
	}
	return &m.search.Results[cursor]
}

func (m *tuiModel) toggleSelectedPause() {
	transfer := m.selectedTransfer()
	if transfer == nil {
		return
	}
	if transfer.Status.Paused {
		if err := m.client.ResumeTransfer(transfer.Hash); err != nil {
			m.statusMessage = err.Error()
			return
		}
		m.statusMessage = "resumed " + transfer.FileName
		return
	}
	if err := m.client.PauseTransfer(transfer.Hash); err != nil {
		m.statusMessage = err.Error()
		return
	}
	m.statusMessage = "paused " + transfer.FileName
}

func (m *tuiModel) removeSelectedTransfer() {
	transfer := m.selectedTransfer()
	if transfer == nil {
		return
	}
	if err := m.client.RemoveTransfer(transfer.Hash, false); err != nil {
		m.statusMessage = err.Error()
		return
	}
	m.statusMessage = "removed " + transfer.FileName
}

func (m tuiModel) selectedTransfer() *ed2k.TransferSnapshot {
	if m.selectedHash == "" && len(m.transfers) > 0 {
		return &m.transfers[0]
	}
	for i := range m.transfers {
		if m.transfers[i].Hash.String() == m.selectedHash {
			return &m.transfers[i]
		}
	}
	return nil
}

func (m tuiModel) renderHeader() string {
	idClass := "UNKNOWN"
	for _, server := range m.status.Servers {
		if server.Primary {
			idClass = server.IDClass()
			break
		}
	}
	status := fmt.Sprintf(
		"goed2k  id=%s tcp=%d udp=%d transfers=%d peers=%d progress=%.1f%% recv=%.1f%% rate=%s up=%s",
		idClass,
		m.runtimeListenPort(),
		m.runtimeUDPPort(),
		len(m.status.Transfers),
		len(m.status.Peers),
		percent(m.status.TotalDone, m.status.TotalWanted),
		percent(m.status.TotalReceived, m.status.TotalWanted),
		humanRate(m.status.DownloadRate),
		humanRate(m.status.UploadRate),
	)
	if m.includeDHT {
		dht := m.client.DHTStatus()
		status += fmt.Sprintf("  kad live=%d known=%d run=%d", dht.LiveNodes, dht.KnownNodes, dht.RunningTraversals)
	}
	return headerStyle.Render(status)
}

func (m tuiModel) renderDetails(width int) string {
	transfer := m.selectedTransfer()
	if transfer == nil {
		return m.renderEmptyState(width)
	}

	sections := []string{
		m.renderTransferSection(*transfer, width),
		m.renderPieceSection(*transfer, width),
		m.renderPeerSection(*transfer, width),
		m.renderServerSection(width),
	}
	return strings.Join(sections, "\n\n")
}

func (m tuiModel) renderEmptyState(width int) string {
	lines := []string{
		titleStyle.Render("Download Manager"),
		"no transfers yet",
		"",
		fmt.Sprintf("output=%s", emptyFallback(m.cfg.outDir, ".")),
		fmt.Sprintf("servers=%s", emptyFallback(m.cfg.serverAddr, "-")),
		fmt.Sprintf("server.met=%s", emptyFallback(m.cfg.serverMetPath, "-")),
		fmt.Sprintf("kad=%t  upnp=%t  nodes.dat=%s", m.cfg.enableKAD, m.cfg.enableUPnP, emptyFallback(m.cfg.kadNodesDat, "-")),
		fmt.Sprintf("kad bootstrap=%s", emptyFallback(m.cfg.kadNodes, "-")),
		fmt.Sprintf("listen=%d  udp=%d  peer-timeout=%ds", m.runtimeListenPort(), m.runtimeUDPPort(), m.cfg.peerTimeout),
	}
	if !m.deadline.IsZero() {
		lines = append(lines, fmt.Sprintf("timeout=%s", time.Until(m.deadline).Round(time.Second)))
	} else {
		lines = append(lines, "timeout=0")
	}
	lines = append(lines,
		"",
		progressBarLine("done", m.status.TotalDone, m.status.TotalWanted, width-8),
		progressBarLine("recv", m.status.TotalReceived, m.status.TotalWanted, width-8),
		"",
		"type /new to add a transfer",
		"type /search to search resources",
		"type /setting to change startup config",
	)
	return strings.Join(lines, "\n")
}

func (m tuiModel) renderTransferSection(transfer ed2k.TransferSnapshot, width int) string {
	kadPeers := countPeersWithSource(transfer.Peers, ed2k.PeerDHT)
	serverPeers := countPeersWithSource(transfer.Peers, ed2k.PeerServer)
	lines := []string{
		titleStyle.Render(transfer.FileName),
		fmt.Sprintf("state=%s  done=%.2f%%  recv=%.2f%%", transfer.Status.State, percent(transfer.Status.TotalDone, transfer.Status.TotalWanted), percent(transfer.Status.TotalReceived, transfer.Status.TotalWanted)),
		fmt.Sprintf("rate=%s  up=%s  peers=%d  active=%d  kad_peers=%d  server_peers=%d", humanRate(transfer.Status.DownloadRate), humanRate(transfer.Status.UploadRate), transfer.Status.NumPeers, transfer.ActivePeers, kadPeers, serverPeers),
		fmt.Sprintf("done=%d  recv=%d  total=%d", transfer.Status.TotalDone, transfer.Status.TotalReceived, transfer.Status.TotalWanted),
		progressBarLine("done", transfer.Status.TotalDone, transfer.Status.TotalWanted, width-8),
		progressBarLine("recv", transfer.Status.TotalReceived, transfer.Status.TotalWanted, width-8),
	}
	if link := transfer.ED2KLink(); link != "" {
		lines = append(lines, "ed2k="+trimString(link, maxInt(24, width-8)))
	}
	return strings.Join(lines, "\n")
}

func (m tuiModel) renderPieceSection(transfer ed2k.TransferSnapshot, width int) string {
	finished := 0
	downloading := 0
	missing := 0
	total := len(transfer.Pieces)
	downloadingPieces := make([]ed2k.PieceSnapshot, 0)
	for _, piece := range transfer.Pieces {
		switch piece.State {
		case ed2k.PieceSnapshotFinished:
			finished++
		case ed2k.PieceSnapshotDownloading:
			downloading++
			downloadingPieces = append(downloadingPieces, piece)
		default:
			missing++
		}
	}
	sort.Slice(downloadingPieces, func(i, j int) bool {
		if downloadingPieces[i].ReceivedBytes != downloadingPieces[j].ReceivedBytes {
			return downloadingPieces[i].ReceivedBytes > downloadingPieces[j].ReceivedBytes
		}
		return downloadingPieces[i].Index < downloadingPieces[j].Index
	})
	lines := []string{
		titleStyle.Render("Pieces"),
		fmt.Sprintf("total=%d  finished=%d  downloading=%d  missing=%d", total, finished, downloading, missing),
	}
	limit := minInt(len(downloadingPieces), 8)
	for i := 0; i < limit; i++ {
		piece := downloadingPieces[i]
		lines = append(lines,
			fmt.Sprintf("#%d recv=%.1f%% done=%.1f%% blocks=%d/%d pending=%d",
				piece.Index,
				percent(piece.ReceivedBytes, piece.TotalBytes),
				percent(piece.DoneBytes, piece.TotalBytes),
				piece.BlocksDone+piece.BlocksWriting,
				piece.BlocksTotal,
				piece.BlocksPending,
			))
	}
	if limit == 0 {
		lines = append(lines, "no downloading pieces")
	}
	return strings.Join(lines, "\n")
}

func (m tuiModel) renderPeerSection(transfer ed2k.TransferSnapshot, width int) string {
	lines := []string{titleStyle.Render("Peers")}
	peers := make([]ed2k.ClientPeerSnapshot, 0)
	for _, peer := range m.status.Peers {
		if peer.TransferHash.Compare(transfer.Hash) == 0 {
			peers = append(peers, peer)
		}
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Peer.DownloadSpeed != peers[j].Peer.DownloadSpeed {
			return peers[i].Peer.DownloadSpeed > peers[j].Peer.DownloadSpeed
		}
		return peers[i].Peer.Endpoint.String() < peers[j].Peer.Endpoint.String()
	})
	limit := minInt(len(peers), 8)
	for i := 0; i < limit; i++ {
		peer := peers[i]
		lines = append(lines, fmt.Sprintf("%s  down=%s up=%s src=%s fail=%d",
			peer.Peer.Endpoint.String(),
			humanRate(peer.Peer.DownloadSpeed),
			humanRate(peer.Peer.UploadSpeed),
			peer.Peer.SourceString(),
			peer.Peer.FailCount))
	}
	if limit == 0 {
		lines = append(lines, "no peers")
	}
	return strings.Join(lines, "\n")
}

func (m tuiModel) renderServerSection(width int) string {
	lines := []string{
		titleStyle.Render("Servers"),
		fmt.Sprintf("listen tcp=%d udp=%d", m.runtimeListenPort(), m.runtimeUDPPort()),
	}
	for _, server := range m.status.Servers {
		prefix := " "
		if server.Primary {
			prefix = "*"
		}
		lines = append(lines, fmt.Sprintf("%s %s %s hs=%t rate=%s up=%s last=%dms",
			prefix,
			trimString(emptyFallback(server.Address, server.Identifier), maxInt(16, width-28)),
			server.IDClass(),
			server.HandshakeCompleted,
			humanRate(server.DownloadRate),
			humanRate(server.UploadRate),
			server.MillisecondsSinceLastReceive))
	}
	if len(m.status.Servers) == 0 {
		lines = append(lines, "no servers")
	}
	return strings.Join(lines, "\n")
}

func (m tuiModel) runtimeListenPort() int {
	if m.client == nil || m.client.Session() == nil {
		return m.cfg.listenPort
	}
	if port := m.client.Session().GetListenPort(); port >= 0 {
		return port
	}
	return m.cfg.listenPort
}

func (m tuiModel) runtimeUDPPort() int {
	if m.client == nil || m.client.Session() == nil {
		return m.cfg.udpPort
	}
	if port := m.client.Session().GetUDPPort(); port >= 0 {
		return port
	}
	return m.cfg.udpPort
}

func progressBarLine(label string, value, total int64, width int) string {
	if width < 10 {
		width = 10
	}
	return fmt.Sprintf("%s %s %.1f%%", label, progressBar(value, total, width), percent(value, total))
}

func progressBar(value, total int64, width int) string {
	if width < 4 {
		width = 4
	}
	filled := 0
	if total > 0 {
		filled = int((value * int64(width)) / total)
	}
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("=", filled) + strings.Repeat(" ", width-filled) + "]"
}

func trimString(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 3 {
		return value[:width]
	}
	return value[:width-3] + "..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func countPeersWithSource(peers []ed2k.PeerInfo, source byte) int {
	count := 0
	for _, peer := range peers {
		if peer.HasSource(source) {
			count++
		}
	}
	return count
}

func searchScopeLabel(scope ed2k.SearchScope) string {
	switch scope {
	case ed2k.SearchScopeAll:
		return "server|kad"
	case ed2k.SearchScopeServer:
		return "server"
	case ed2k.SearchScopeDHT:
		return "kad"
	default:
		return "unknown"
	}
}

func searchResultSourceLabel(source ed2k.SearchResultSource) string {
	parts := make([]string, 0, 2)
	if source&ed2k.SearchResultServer != 0 {
		parts = append(parts, "server")
	}
	if source&ed2k.SearchResultKAD != 0 {
		parts = append(parts, "kad")
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, "|")
}

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("69"))
	panelBorder = lipgloss.RoundedBorder()
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(0, 1)
)

func panelStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Border(panelBorder).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)
}
