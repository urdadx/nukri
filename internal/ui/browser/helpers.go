package browser

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func panel(content, title string, width, height int, fg, bg, border, titleColor lipgloss.Color) string {
	return panelWithPadding(content, title, width, height, 1, 1, fg, bg, border, titleColor)
}

func panelWithPadding(content, title string, width, height, leftPadding, rightPadding int, fg, bg, border, titleColor lipgloss.Color) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	contentWidth := max(0, width-2-leftPadding-rightPadding)
	if title != "" {
		styled := lipgloss.NewStyle().Foreground(titleColor).Background(bg).Bold(true).Render(title)
		content = styled + "\n" + strings.Repeat(" ", contentWidth) + "\n" + content
	}
	contentLines := strings.Split(content, "\n")
	contentHeight := max(0, height-2)
	if len(contentLines) > contentHeight {
		contentLines = contentLines[:contentHeight]
	}
	for i := range contentLines {
		contentLines[i] = ansi.Truncate(contentLines[i], contentWidth, "")
	}
	return lipgloss.NewStyle().
		Foreground(fg).Background(bg).Border(lipgloss.RoundedBorder()).BorderForeground(border).BorderBackground(bg).
		PaddingLeft(leftPadding).PaddingRight(rightPadding).Width(max(0, width-2)).Height(max(0, height-2)).
		Render(strings.Join(contentLines, "\n"))
}

func truncate(text string, width int) string { return ansi.Truncate(text, width, "…") }
