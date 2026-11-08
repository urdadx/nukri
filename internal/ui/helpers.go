package ui

import "github.com/charmbracelet/x/ansi"

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(text, width, "…")
}
