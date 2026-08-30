package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/urdadx/nukri/internal/theme"
)

type Styles struct {
	Root, Chrome, Muted, Accent, Button lipgloss.Style
	Theme                               theme.Theme
}

func NewStyles(t theme.Theme) Styles {
	return Styles{
		Root:   lipgloss.NewStyle().Foreground(lipgloss.Color(t.FullScreenFG)).Background(lipgloss.Color(t.FilePanelBG)),
		Chrome: lipgloss.NewStyle().Foreground(lipgloss.Color(t.FooterFG)).Background(lipgloss.Color(t.FooterBG)),
		Muted:  lipgloss.NewStyle().Foreground(lipgloss.Color(t.SidebarDivider)).Background(lipgloss.Color(t.FooterBG)),
		Accent: lipgloss.NewStyle().Foreground(lipgloss.Color(t.FilePanelTopPath)).Background(lipgloss.Color(t.FooterBG)).Bold(true),
		Button: lipgloss.NewStyle().Foreground(lipgloss.Color(t.FooterFG)).Background(lipgloss.Color(t.ModalBG)).Bold(true).Padding(0, 1),
		Theme:  t,
	}
}
