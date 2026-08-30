package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func fill(style lipgloss.Style, text string, width int) string {
	if width <= 0 {
		return ""
	}
	text = truncate(text, width)
	return style.Width(width).Render(text)
}

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(text)
	for len(runes) > 0 && lipgloss.Width(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func middleTruncate(text string, width int) string {
	if lipgloss.Width(text) <= width {
		return text
	}
	if width < 4 {
		return truncate(text, width)
	}
	runes := []rune(text)
	left, right := (width-1)/2, width-1-(width-1)/2
	return truncate(string(runes), left)[:left] + "…" + string(runes[len(runes)-right:])
}

func lines(text string) []string { return strings.Split(text, "\n") }
