package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/theme"
	"github.com/urdadx/nukri/internal/ui/browser"
)

type Model struct {
	width, height  int
	styles         Styles
	data           browser.Data
	preview        *preview.Service
	previewPool    *PreviewPool
	visualState    *browser.VisualState
	backHistory    []historyEntry
	forwardHistory []historyEntry
	prefetchSeq    int
	lastClickPath  string
	lastClickAt    time.Time
}

const doubleClickWindow = 500 * time.Millisecond

type historyEntry struct {
	path         string
	selectedPath string
}

func New(t theme.Theme) Model {
	svc := preview.NewService()
	return Model{width: 120, height: 32, styles: NewStyles(t), preview: svc, previewPool: NewPreviewPool(svc, 2), visualState: &browser.VisualState{}, data: browser.Data{Selected: -1, PreviewSelected: -1}}
}

func (Model) Init() tea.Cmd { return loadFilesystem }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		m.clampPreviewOffset()
	case loadedMsg:
		m.data.CWD, m.data.Places, m.data.Entries = message.cwd, message.places, message.entries
		m.data.LoadError = ""
		m.data.Selected = -1
		m.data.Preview = preview.View{}
		m.data.PreviewEntries = nil
		m.data.PreviewIsDir = false
		m.data.PreviewSelected = -1
		m.data.PreviewOffset = 0
		m.data.PreviewError = ""
		m.data.PreviewLoading = false
		if message.err != nil {
			m.data.LoadError = message.err.Error()
		}
		if len(message.entries) != 0 {
			m.data.Selected = entryIndexByPath(message.entries, message.selectedPath)
			m.data.PreviewLoading = true
			layout := browser.ResolveLayout(m.width, max(1, m.height-1))
			width := layout.PreviewWidth - 4
			m, prefetch := schedulePrefetch(m, message.entries, m.data.Selected, width, m.styles.Theme.CodeSyntaxHighlight)
			return m, tea.Batch(loadPreview(m.previewPool, message.entries[m.data.Selected], width, m.initialCodeWindow(), m.styles.Theme.CodeSyntaxHighlight), prefetch)
		}
	case previewMsg:
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
		}
	case prefetchMsg:
		m.submitPrefetch(message)
	case tea.MouseMsg:
		if message.Button == tea.MouseButtonLeft && message.Action == tea.MouseActionPress {
			if path := m.sidebarPathAt(message.X, message.Y); path != "" {
				return m.navigateTo(path, "", true)
			}
			if index := m.entryIndexAt(message.X, message.Y); index >= 0 {
				return m.clickEntry(index, time.Now())
			}
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
	case tea.KeyMsg:
		switch message.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "down", "j":
			return m.moveSelection(1)
		case "up", "k":
			return m.moveSelection(-1)
		case "enter":
			return m.enterSelected()
		case "backspace", "left", "h":
			return m.goParent()
		case "alt+left":
			return m.goHistoryBack()
		case "alt+right":
			return m.goHistoryForward()
		case "tab":
			return m.cyclePlace(1)
		case "shift+tab":
			return m.cyclePlace(-1)
		case "pgdown", "ctrl+d":
			return m, m.scrollPreview(m.previewPageSize())
		case "pgup", "ctrl+u":
			return m, m.scrollPreview(-m.previewPageSize())
		}
	}
	return m, nil
}

func (m Model) sidebarPathAt(x, y int) string {
	layout := browser.ResolveLayout(m.width, max(1, m.height-1))
	if layout.SidebarWidth == 0 || x < 0 || x >= layout.SidebarWidth || y < 3 || y >= m.height-1 {
		return ""
	}
	compact := layout.SidebarWidth <= 5
	rowIndex := y - 3
	renderedRow := 0
	for _, row := range m.data.Places {
		if row.Section != "" {
			if !compact {
				renderedRow++
			}
			continue
		}
		if renderedRow == rowIndex {
			if item := row.GetItem(); item != nil {
				return item.Path
			}
			return ""
		}
		renderedRow++
	}
	return ""
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
		if item := row.GetItem(); item != nil {
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
	m.data.Preview = preview.View{}
	m.data.PreviewEntries = nil
	m.data.PreviewIsDir = false
	m.data.PreviewSelected = -1
	m.data.PreviewOffset = 0
	m.data.PreviewError = ""
	m.data.PreviewLoading = false
	return m, loadDirectory(path, selectedPath)
}

func (m Model) mouseOverPreview(x, y int) bool {
	layout := browser.ResolveLayout(m.width, max(1, m.height-1))
	if layout.PreviewWidth == 0 || y < 0 || y >= m.height-1 {
		return false
	}
	if layout.Stacked {
		return x >= layout.SidebarWidth && x < m.width && y >= layout.FilesHeight
	}
	return x >= layout.SidebarWidth+layout.FilesWidth && x < m.width
}

func (m Model) mouseOverEntries(x, y int) bool {
	layout := browser.ResolveLayout(m.width, max(1, m.height-1))
	if x < layout.SidebarWidth || x >= layout.SidebarWidth+layout.FilesWidth || y < 0 || y >= m.height-1 {
		return false
	}
	return !layout.Stacked || y < layout.FilesHeight
}

func (m Model) entryIndexAt(x, y int) int {
	if !m.mouseOverEntries(x, y) || y < 3 {
		return -1
	}
	layout := browser.ResolveLayout(m.width, max(1, m.height-1))
	visibleRows := max(0, layout.FilesHeight-4)
	row := y - 3
	if row >= visibleRows {
		return -1
	}
	start := 0
	if m.data.Selected >= visibleRows {
		start = m.data.Selected - visibleRows + 1
	}
	index := start + row
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
	m.data.Preview = preview.View{}
	m.data.PreviewEntries = nil
	m.data.PreviewIsDir = false
	m.data.PreviewSelected = -1
	m.data.PreviewOffset = 0
	m.data.PreviewError = ""
	m.data.PreviewLoading = true
	layout := browser.ResolveLayout(m.width, max(1, m.height-1))
	width := layout.PreviewWidth - 4
	m, prefetch := schedulePrefetch(m, m.data.Entries, next, width, m.styles.Theme.CodeSyntaxHighlight)
	return m, tea.Batch(loadPreview(m.previewPool, m.data.Entries[next], width, m.initialCodeWindow(), m.styles.Theme.CodeSyntaxHighlight), prefetch)
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
	layout := browser.ResolveLayout(m.width, max(1, m.height-1))
	width := layout.PreviewWidth - 4
	return extendPreview(m.previewPool, m.data.Entries[m.data.Selected], width, window, m.styles.Theme.CodeSyntaxHighlight)
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
	layout := browser.ResolveLayout(m.width, max(1, m.height-1))
	height := m.height - 1
	if layout.Stacked {
		height = layout.PreviewHeight
	}
	if layout.PreviewWidth == 0 {
		return 0
	}
	return max(0, height-4)
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
	t := m.styles.Theme
	browserStyles := browser.Styles{
		RootFG: lipgloss.Color(t.FullScreenFG), RootBG: lipgloss.Color(t.FullScreenBG),
		PanelFG: lipgloss.Color(t.FilePanelFG), PanelBG: lipgloss.Color(t.FilePanelBG), PanelBorder: lipgloss.Color(t.FilePanelBorder), ActiveBorder: lipgloss.Color(t.FilePanelBorderActive),
		Path: lipgloss.Color(t.FilePanelTopPath), Directory: lipgloss.Color(t.FilePanelTopDirectoryIcon), SelectedFG: lipgloss.Color(t.FilePanelItemSelectedFG), SelectedBG: lipgloss.Color(t.FilePanelItemSelectedBG), Muted: lipgloss.Color(t.SidebarDivider),
		SidebarFG: lipgloss.Color(t.SidebarFG), SidebarBG: lipgloss.Color(t.SidebarBG), SidebarTitle: lipgloss.Color(t.FilePanelTopPath), SidebarBorder: lipgloss.Color(t.FilePanelBorder), SidebarIcon: lipgloss.Color(t.FilePanelTopDirectoryIcon),
		SidebarSelectedFG: lipgloss.Color(t.SidebarItemSelectedFG), SidebarSelectedBG: lipgloss.Color(t.SidebarItemSelectedBG), Cursor: lipgloss.Color(t.Cursor),
	}
	body := browser.Render(bodyWidth, bodyHeight, m.visualState, os.Stdout, browserStyles, m.data)
	view := lipgloss.JoinVertical(lipgloss.Left, body, renderFooter(m.width, m.styles, m.data))
	return m.styles.Root.Width(m.width).Height(m.height).MaxWidth(m.width).MaxHeight(m.height).Render(view)
}
