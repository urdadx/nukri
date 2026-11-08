package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/urdadx/nukri/internal/theme"
	"github.com/urdadx/nukri/internal/ui/browser"
)

type Styles struct {
	Root, Chrome, Muted, Accent, Button lipgloss.Style
	Browser                             browser.Styles
	Theme                               theme.Theme
}

func NewStyles(t theme.Theme) Styles {
	return Styles{
		Root:   lipgloss.NewStyle().Foreground(lipgloss.Color(t.FullScreenFG)).Background(lipgloss.Color(t.FilePanelBG)),
		Chrome: lipgloss.NewStyle().Foreground(lipgloss.Color(t.FooterFG)).Background(lipgloss.Color(t.FooterBG)),
		Muted:  lipgloss.NewStyle().Foreground(lipgloss.Color(t.SidebarDivider)).Background(lipgloss.Color(t.FooterBG)),
		Accent: lipgloss.NewStyle().Foreground(lipgloss.Color(t.FilePanelTopPath)).Background(lipgloss.Color(t.FooterBG)).Bold(true),
		Button: lipgloss.NewStyle().Foreground(lipgloss.Color(t.FooterFG)).Background(lipgloss.Color(t.ModalBG)).Bold(true).Padding(0, 1),
		Browser: browser.Styles{
			RootFG: lipgloss.Color(t.FullScreenFG), RootBG: lipgloss.Color(t.FullScreenBG),
			PanelFG: lipgloss.Color(t.FilePanelFG), PanelBG: lipgloss.Color(t.FilePanelBG), PanelBorder: lipgloss.Color(t.FilePanelBorder), ActiveBorder: lipgloss.Color(t.FilePanelBorderActive),
			Path: lipgloss.Color(t.FilePanelTopPath), Directory: lipgloss.Color(t.FilePanelTopDirectoryIcon), SelectedFG: lipgloss.Color(t.FilePanelItemSelectedFG), SelectedBG: lipgloss.Color(t.FilePanelItemSelectedBG), Muted: lipgloss.Color(t.SidebarDivider),
			SidebarFG: lipgloss.Color(t.SidebarFG), SidebarBG: lipgloss.Color(t.SidebarBG), SidebarTitle: lipgloss.Color(t.FilePanelTopPath), SidebarBorder: lipgloss.Color(t.FilePanelBorder), SidebarIcon: lipgloss.Color(t.FilePanelTopDirectoryIcon),
			SidebarSelectedFG: lipgloss.Color(t.SidebarItemSelectedFG), SidebarSelectedBG: lipgloss.Color(t.SidebarItemSelectedBG), Cursor: lipgloss.Color(t.Cursor),
			OperationCopy: lipgloss.Color(t.Correct), OperationCut: lipgloss.Color(t.Cancel),
		},
		Theme: t,
	}
}
