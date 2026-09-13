package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/urdadx/nukri/internal/ui/browser"
)

func renderFooter(width int, s Styles, data browser.Data, status string) string {
	position, name := "0/0", "Loading..."
	if data.Selected >= 0 && data.Selected < len(data.Entries) {
		position = fmt.Sprintf("%d/%d", data.Selected+1, len(data.Entries))
		name = data.Entries[data.Selected].Entry.Name
	} else if data.LoadError != "" {
		name = "Load failed"
	}
	left := s.Accent.Render(" "+position+"  "+name) + s.Muted.Render(" │  main")
	if status == "" {
		status = "Ready"
	}
	right := s.Muted.Render(status + "  q quit ")
	if lipgloss.Width(left)+lipgloss.Width(right) > width {
		right = ansi.Truncate(right, max(0, width-lipgloss.Width(left)), "")
	}
	gap := max(0, width-lipgloss.Width(left)-lipgloss.Width(right))
	filler := s.Chrome.Render(strings.Repeat(" ", gap))
	return ansi.Truncate(left+filler+right, width, "")
}
