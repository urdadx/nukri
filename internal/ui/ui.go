package ui

import (
	"io"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	appfileops "github.com/urdadx/nukri/internal/app/fileops"
	"github.com/urdadx/nukri/internal/config/keys"
	fsfileops "github.com/urdadx/nukri/internal/fs/fileops"
	"github.com/urdadx/nukri/internal/kittydnd"
	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/theme"
	"github.com/urdadx/nukri/internal/ui/browser"
)

type Model struct {
	width, height  int
	styles         Styles
	data           browser.Data
	previewPool    *PreviewPool
	searchPool     *SearchPool
	visualState    *browser.VisualState
	backHistory    []historyEntry
	forwardHistory []historyEntry
	prefetchSeq    int
	lastClickPath  string
	lastClickAt    time.Time
	drag           dragState
	clipboard      appfileops.Clipboard
	status         string
	search         searchState
	placesStarted  bool
	archive        archiveState
}

const doubleClickWindow = 500 * time.Millisecond

type historyEntry struct {
	path         string
	selectedPath string
}

func New(t theme.Theme) Model {
	svc := preview.NewService()
	bar := progress.New(progress.WithDefaultGradient())
	bar.Width = 12
	bar.ShowPercentage = false
	return Model{width: 120, height: 32, styles: NewStyles(t), previewPool: NewPreviewPool(svc, 2), searchPool: NewSearchPool(), visualState: &browser.VisualState{}, data: browser.Data{Selected: -1, PreviewSelected: -1}, archive: archiveState{progress: bar}}
}

func (m Model) WithDragOutput(output io.Writer) Model {
	m.drag.output = output
	return m
}

func (Model) Init() tea.Cmd { return loadFilesystem }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		m.clampPreviewOffset()
	case loadedMsg:
		m.data.CWD, m.data.Entries = message.cwd, message.entries
		// The sidebar loads once, after the first listing, then keeps itself fresh.
		var places tea.Cmd
		if !m.placesStarted {
			m.placesStarted = true
			places = loadPlaces
		}
		m.data.LoadError = ""
		m.data.Selected = -1
		m.resetPreview(false)
		if message.err != nil {
			m.data.LoadError = message.err.Error()
		}
		if len(message.entries) != 0 {
			m.data.Selected = entryIndexByPath(message.entries, message.selectedPath)
			m.resetPreview(true)
			m, load := m.requestPreview()
			return m, tea.Batch(load, loadItemCounts(message.cwd, message.entries, false), places)
		}
		return m, places
	case placesMsg:
		m.data.Places = message.rows
		return m, refreshPlaces()
	case itemCountsMsg:
		switch {
		case message.preview:
			if m.data.PreviewIsDir && m.selectedPath() == message.cwd {
				m.data.PreviewEntries = applyItemCounts(m.data.PreviewEntries, message.counts)
			}
		case message.cwd == m.data.CWD:
			m.data.Entries = applyItemCounts(m.data.Entries, message.counts)
		}
	case previewMsg:
		if message.cancelled {
			return m, nil
		}
		if m.data.Selected >= 0 && m.data.Selected < len(m.data.Entries) && m.data.Entries[m.data.Selected].Entry.Path == message.path {
			m.data.PreviewLoading = false
			m.data.Preview = message.view
			m.data.PreviewEntries = message.entries
			m.data.PreviewIsDir = message.directory
			m.data.PreviewSelected = -1
			if message.directory && len(message.entries) > 0 {
				m.data.PreviewSelected = 0
			}
			m.data.PreviewError = ""
			if message.err != nil {
				m.data.PreviewError = message.err.Error()
			}
			if message.directory {
				return m, loadItemCounts(message.path, message.entries, true)
			}
		}
	case prefetchMsg:
		m.submitPrefetch(message)
	case searchMsg:
		return m.handleSearchEvent(message)
	case searchRefreshMsg:
		return m.handleSearchRefresh(message)
	case transferMsg:
		if message.origin == transferClipboard && message.err == nil {
			m.status = ""
		} else {
			m.status = message.statusText()
		}
		if message.origin == transferClipboard && message.completed > 0 {
			m.clipboard.ClearIf(message.source, message.operation)
		}
		if message.destination == m.data.CWD {
			return m, loadDirectory(message.destination, message.selectedPath)
		}
	case archiveMsg:
		m.archive.busy = false
		m.archive.operation = ""
		m.archive.percent = 0
		m.status = message.statusText()
		if message.err == nil && filepath.Dir(message.output) == m.data.CWD {
			return m, loadDirectory(m.data.CWD, message.output)
		}
	case archiveProgressMsg:
		m.archive.percent = message.percent
		return m, waitArchive(message.stream)
	case tea.MouseMsg:
		if m.archive.promptOpen {
			return m, nil
		}
		if m.search.open {
			return m.handleSearchMouse(message)
		}
		if message.Button == tea.MouseButtonLeft && message.Action == tea.MouseActionPress {
			if path := m.sidebarPathAt(message.X, message.Y); path != "" {
				return m.navigateTo(path, "", true)
			}
			if index := m.entryIndexAt(message.X, message.Y); index >= 0 {
				m.drag.candidate = m.data.Entries[index].Entry.Path
				return m.clickEntry(index, time.Now())
			}
		}
		if message.Button == tea.MouseButtonLeft && message.Action == tea.MouseActionRelease {
			m.drag.candidate = ""
		}
		if m.mouseOverEntries(message.X, message.Y) {
			switch message.Button {
			case tea.MouseButtonWheelUp:
				return m.moveSelection(-1)
			case tea.MouseButtonWheelDown:
				return m.moveSelection(1)
			}
		}
		if m.mouseOverPreview(message.X, message.Y) {
			switch message.Button {
			case tea.MouseButtonWheelUp:
				return m, m.scrollPreview(-m.previewWheelStep())
			case tea.MouseButtonWheelDown:
				return m, m.scrollPreview(m.previewWheelStep())
			default:
				return m, nil
			}
		}
	case kittydnd.Event:
		return m.handleDndEvent(message)
	case tea.KeyMsg:
		if m.archive.promptOpen {
			return m.handleArchivePromptKey(message)
		}
		if m.search.open {
			if message.Type == tea.KeyEsc {
				return m.closeSearch(), nil
			}
			return m.handleSearchKey(message)
		}
		if action, ok := keys.Resolve(message.String()); ok {
			return m.handleAction(action)
		}
	}
	return m, nil
}

// handleAction runs a key-bound action in the file browser.
func (m Model) handleAction(action keys.Action) (tea.Model, tea.Cmd) {
	switch action {
	case keys.ActionQuit:
		return m, tea.Quit
	case keys.ActionSearch:
		return m.openSearch()
	case keys.ActionDown:
		return m.moveSelection(1)
	case keys.ActionUp:
		return m.moveSelection(-1)
	case keys.ActionOpen:
		return m.enterSelected()
	case keys.ActionParent:
		return m.goParent()
	case keys.ActionHistoryBack:
		return m.goHistoryBack()
	case keys.ActionHistoryForward:
		return m.goHistoryForward()
	case keys.ActionNextPlace:
		return m.cyclePlace(1)
	case keys.ActionPreviousPlace:
		return m.cyclePlace(-1)
	case keys.ActionPreviewPageDown:
		return m, m.scrollPreview(m.previewPageSize())
	case keys.ActionPreviewPageUp:
		return m, m.scrollPreview(-m.previewPageSize())
	case keys.ActionCreateArchive:
		return m.openArchivePrompt()
	case keys.ActionExtractArchive:
		return m.extractSelectedArchive()
	case keys.ActionCopy:
		return m.copySelected()
	case keys.ActionCut:
		return m.cutSelected()
	case keys.ActionPaste:
		return m.pasteClipboard()
	case keys.ActionCancelClipboard:
		return m.cancelClipboard()
	}
	return m, nil
}

func (m Model) sidebarPathAt(x, y int) string {
	layout := m.layout()
	if layout.SidebarWidth == 0 || x < 0 || x >= layout.SidebarWidth || y < browser.PanelContentTop || y >= m.height-1 {
		return ""
	}
	return browser.SidebarPathAt(layout.SidebarWidth, m.data.Places, y-browser.PanelContentTop)
}

func entryIndexByPath(entries []browser.Entry, path string) int {
	if path != "" {
		for index := range entries {
			if entries[index].Entry.Path == path {
				return index
			}
		}
	}
	return 0
}

func (m Model) enterSelected() (tea.Model, tea.Cmd) {
	if m.data.Selected < 0 || m.data.Selected >= len(m.data.Entries) {
		return m, nil
	}
	entry := m.data.Entries[m.data.Selected].Entry
	if entry.IsDirectory() {
		return m.navigateTo(entry.Path, "", true)
	}
	return m, openFile(entry.Path)
}

func openFile(path string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("xdg-open", path).Run()
		return nil
	}
}

func (m Model) clickEntry(index int, clickedAt time.Time) (tea.Model, tea.Cmd) {
	entry := m.data.Entries[index].Entry
	doubleClick := index == m.data.Selected && entry.Path == m.lastClickPath && clickedAt.Sub(m.lastClickAt) <= doubleClickWindow
	m.lastClickPath, m.lastClickAt = entry.Path, clickedAt
	if !doubleClick {
		return m.moveSelection(index - m.data.Selected)
	}
	m.drag.candidate = ""
	m.lastClickPath, m.lastClickAt = "", time.Time{}
	return m.enterSelected()
}

func (m Model) goParent() (tea.Model, tea.Cmd) {
	if m.data.CWD == "" {
		return m, nil
	}
	parent := filepath.Dir(m.data.CWD)
	if parent == m.data.CWD {
		return m, nil
	}
	return m.navigateTo(parent, m.data.CWD, true)
}

func (m Model) cyclePlace(delta int) (tea.Model, tea.Cmd) {
	paths := make([]string, 0, len(m.data.Places))
	for _, row := range m.data.Places {
		if item := row.Item; item != nil {
			paths = append(paths, item.Path)
		}
	}
	if len(paths) == 0 {
		return m, nil
	}
	current := -1
	for index, path := range paths {
		if path == m.data.CWD {
			current = index
			break
		}
	}
	next := 0
	if current >= 0 {
		next = (current + delta + len(paths)) % len(paths)
	} else if delta < 0 {
		next = len(paths) - 1
	}
	return m.navigateTo(paths[next], "", true)
}

func (m Model) goHistoryBack() (tea.Model, tea.Cmd) {
	if len(m.backHistory) == 0 {
		return m, nil
	}
	target := m.backHistory[len(m.backHistory)-1]
	m.backHistory = m.backHistory[:len(m.backHistory)-1]
	m.forwardHistory = append(m.forwardHistory, m.currentHistoryEntry())
	return m.navigateTo(target.path, target.selectedPath, false)
}

func (m Model) goHistoryForward() (tea.Model, tea.Cmd) {
	if len(m.forwardHistory) == 0 {
		return m, nil
	}
	target := m.forwardHistory[len(m.forwardHistory)-1]
	m.forwardHistory = m.forwardHistory[:len(m.forwardHistory)-1]
	m.backHistory = append(m.backHistory, m.currentHistoryEntry())
	return m.navigateTo(target.path, target.selectedPath, false)
}

func (m Model) currentHistoryEntry() historyEntry {
	selectedPath := ""
	if m.data.Selected >= 0 && m.data.Selected < len(m.data.Entries) {
		selectedPath = m.data.Entries[m.data.Selected].Entry.Path
	}
	return historyEntry{path: m.data.CWD, selectedPath: selectedPath}
}

func (m Model) navigateTo(path, selectedPath string, recordHistory bool) (tea.Model, tea.Cmd) {
	if path == "" || path == m.data.CWD {
		return m, nil
	}
	if recordHistory && m.data.CWD != "" {
		m.backHistory = append(m.backHistory, m.currentHistoryEntry())
		m.forwardHistory = nil
	}
	m.data.CWD = path
	m.data.Entries = nil
	m.data.Selected = -1
	m.data.LoadError = ""
	m.resetPreview(false)
	return m, loadDirectory(path, selectedPath)
}

func (m Model) mouseOverPreview(x, y int) bool {
	layout := m.layout()
	if layout.PreviewWidth == 0 || y < 0 || y >= m.height-1 {
		return false
	}
	if layout.Stacked {
		return x >= layout.SidebarWidth && x < m.width && y >= layout.FilesHeight
	}
	return x >= layout.SidebarWidth+layout.FilesWidth && x < m.width
}

func (m Model) mouseOverEntries(x, y int) bool {
	layout := m.layout()
	if x < layout.SidebarWidth || x >= layout.SidebarWidth+layout.FilesWidth || y < 0 || y >= m.height-1 {
		return false
	}
	return !layout.Stacked || y < layout.FilesHeight
}

func (m Model) entryIndexAt(x, y int) int {
	if !m.mouseOverEntries(x, y) || y < browser.PanelContentTop {
		return -1
	}
	visibleRows := max(0, m.layout().FilesHeight-browser.PanelChrome)
	row := y - browser.PanelContentTop
	if row >= visibleRows {
		return -1
	}
	index := browser.ListWindowStart(m.data.Selected, visibleRows) + row
	if index >= len(m.data.Entries) {
		return -1
	}
	return index
}

func (m Model) moveSelection(delta int) (tea.Model, tea.Cmd) {
	if len(m.data.Entries) == 0 {
		return m, nil
	}
	next := min(max(m.data.Selected+delta, 0), len(m.data.Entries)-1)
	if next == m.data.Selected {
		return m, nil
	}
	m.data.Selected = next
	m.resetPreview(true)
	return m.requestPreview()
}

// resetPreview clears the preview pane, marking it loading when a new preview
// has been requested.
func (m *Model) resetPreview(loading bool) {
	m.data.Preview = preview.View{}
	m.data.PreviewEntries = nil
	m.data.PreviewIsDir = false
	m.data.PreviewSelected = -1
	m.data.PreviewOffset = 0
	m.data.PreviewError = ""
	m.data.PreviewLoading = loading
}

// requestPreview loads the selected entry's preview and prefetches its
// neighbours.
func (m Model) requestPreview() (Model, tea.Cmd) {
	width, window, syntax := m.previewWidth(), m.initialCodeWindow(), m.styles.Theme.CodeSyntaxHighlight
	m, prefetch := schedulePrefetch(m, m.data.Entries, m.data.Selected, width, window, syntax)
	return m, tea.Batch(loadPreview(m.previewPool, m.data.Entries[m.data.Selected], width, window, syntax), prefetch)
}

func (m Model) layout() browser.Layout {
	return browser.ResolveLayout(m.width, max(1, m.height-1))
}

// previewWidth is the preview pane's content width in cells.
func (m Model) previewWidth() int {
	return m.layout().PreviewWidth - browser.PanelChrome
}

// initialCodeWindow is the leading-line budget for the first render of a code
// preview. It covers the visible pane with margin so the first paint stays cheap
// for very large sources; scrolling past it triggers an incremental extend.
func (m Model) initialCodeWindow() int {
	return max(m.previewVisibleLines()*2, 40)
}

// moreCodeLines reports whether the current code preview is truncated — more
// lines exist beyond the window already rendered into View.Lines.
func (m Model) moreCodeLines() bool {
	return m.data.Preview.TotalLines > len(m.data.Preview.Lines)
}

// previewAtBottom reports whether the preview pane is scrolled to the end of the
// currently-loaded lines.
func (m Model) previewAtBottom() bool {
	return m.data.PreviewOffset >= max(0, m.previewLineCount()-m.previewVisibleLines())
}

// extendCodePreview re-renders the selected code file with a larger leading-line
// window, returning a command that updates the pane with the added lines once
// rendered. It is only scheduled when more lines actually remain.
func (m Model) extendCodePreview() tea.Cmd {
	if m.data.Selected < 0 || m.data.Selected >= len(m.data.Entries) || !m.moreCodeLines() {
		return nil
	}
	window := max(len(m.data.Preview.Lines)*2, m.initialCodeWindow())
	return loadPreview(m.previewPool, m.data.Entries[m.data.Selected], m.previewWidth(), window, m.styles.Theme.CodeSyntaxHighlight)
}

func (m *Model) scrollPreview(delta int) tea.Cmd {
	m.data.PreviewOffset += delta
	m.clampPreviewOffset()
	if delta > 0 && m.previewAtBottom() && m.moreCodeLines() {
		return m.extendCodePreview()
	}
	return nil
}

func (m Model) previewPageSize() int {
	return max(1, m.previewVisibleLines()/2)
}

func (m Model) previewWheelStep() int {
	return min(max(m.previewVisibleLines()/6, 2), 4)
}

func (m Model) previewVisibleLines() int {
	layout := m.layout()
	height := m.height - 1
	if layout.Stacked {
		height = layout.PreviewHeight
	}
	if layout.PreviewWidth == 0 {
		return 0
	}
	return max(0, height-browser.PanelChrome)
}

func (m Model) previewLineCount() int {
	switch {
	case m.data.PreviewLoading, m.data.PreviewError != "":
		return 1
	case m.data.PreviewIsDir:
		return max(1, len(m.data.PreviewEntries))
	default:
		count := 2 + len(m.data.Preview.Lines)
		if m.data.Preview.Visual != nil {
			count += 2
		}
		if m.data.Preview.Footer != "" {
			count += 2
		}
		return count
	}
}

func (m *Model) clampPreviewOffset() {
	m.data.PreviewOffset = min(max(m.data.PreviewOffset, 0), max(0, m.previewLineCount()-m.previewVisibleLines()))
}

func (m Model) View() string {
	if m.width < 24 || m.height < 10 {
		return m.styles.Root.Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center).Render("Terminal too small")
	}
	bodyHeight := m.height - 1
	bodyWidth := m.width
	browserData := m.data
	browserData.OperationPath = m.clipboard.Source
	browserData.OperationCut = m.clipboard.Operation == fsfileops.Move
	body := browser.Render(bodyWidth, bodyHeight, m.visualState, m.styles.Browser, browserData)
	operation := m.clipboardStatus()
	if archiveStatus := m.archiveStatus(); archiveStatus != "" {
		operation = archiveStatus
	}
	view := lipgloss.JoinVertical(lipgloss.Left, body, renderFooter(m.width, m.styles, m.data, operation, m.status))
	view = m.styles.Root.Width(m.width).Height(m.height).MaxWidth(m.width).MaxHeight(m.height).Render(view)
	if m.search.open {
		m.visualState.Clear()
		return m.renderSearch(view)
	}
	if m.archive.promptOpen {
		m.visualState.Clear()
		return m.renderArchivePrompt(view)
	}
	return view
}
