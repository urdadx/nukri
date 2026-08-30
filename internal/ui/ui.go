package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/theme"
	"github.com/urdadx/nukri/internal/ui/browser"
)

type Model struct {
	width, height int
	styles        Styles
	data          browser.Data
	preview       *preview.Service
}

func New(t theme.Theme) Model {
	return Model{width: 120, height: 32, styles: NewStyles(t), preview: preview.NewService(), data: browser.Data{Selected: -1}}
}

func (Model) Init() tea.Cmd { return loadFilesystem }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
	case loadedMsg:
		m.data.CWD, m.data.Places, m.data.Entries = message.cwd, message.places, message.entries
		if message.err != nil {
			m.data.LoadError = message.err.Error()
		}
		if len(message.entries) != 0 {
			m.data.Selected = 0
			m.data.PreviewLoading = true
			layout := browser.ResolveLayout(m.width, max(1, m.height-1))
			return m, loadPreview(m.preview, message.entries[0], layout.PreviewWidth-4)
		}
	case previewMsg:
		if m.data.Selected >= 0 && m.data.Selected < len(m.data.Entries) && m.data.Entries[m.data.Selected].Entry.Path == message.path {
			m.data.PreviewLoading = false
			m.data.Preview = message.view
			if message.err != nil {
				m.data.PreviewError = message.err.Error()
			}
		}
	case tea.KeyMsg:
		if message.String() == "q" || message.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
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
	body := browser.Render(bodyWidth, bodyHeight, browserStyles, m.data)
	view := lipgloss.JoinVertical(lipgloss.Left, body, renderFooter(m.width, m.styles, m.data))
	return m.styles.Root.Width(m.width).Height(m.height).MaxWidth(m.width).MaxHeight(m.height).Render(view)
}
