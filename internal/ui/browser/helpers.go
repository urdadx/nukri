package browser

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func panel(content, title string, width, height int, fg, bg, border, titleColor lipgloss.Color) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if title != "" {
		styled := lipgloss.NewStyle().Foreground(titleColor).Background(bg).Bold(true).Render(title)
		content = styled + "\n" + strings.Repeat(" ", max(0, width-4)) + "\n" + content
	}
	contentLines := strings.Split(content, "\n")
	for i := range contentLines {
		contentLines[i] = ansi.Truncate(contentLines[i], max(0, width-4), "")
	}
	return lipgloss.NewStyle().
		Foreground(fg).Background(bg).Border(lipgloss.RoundedBorder()).BorderForeground(border).BorderBackground(bg).
		Padding(0, 1).Width(max(0, width-2)).Height(max(0, height-2)).
		Render(strings.Join(contentLines, "\n"))
}

func truncate(text string, width int) string { return ansi.Truncate(text, width, "…") }
