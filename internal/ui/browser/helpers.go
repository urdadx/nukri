package browser

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Panel geometry shared by rendering and mouse hit-testing.
const (
	// PanelContentTop is the number of rows above a titled panel's content: the
	// top border, the title and a blank line.
	PanelContentTop = 3
	// PanelChrome is the width and height a panel's border and padding take up.
	PanelChrome = 4
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

/*
SafeName makes a file name safe to draw. Names may legally hold control
characters: a carriage return would move the cursor back over the panels to
its left, and an escape could send the terminal commands. Each one is shown as
"?".
*/
func SafeName(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '?'
		}
		return r
	}, name)
}

/*
safeLine prepares a preview line for the pane. Escape sequences are kept
(highlighted code is styled with them), but carriage returns and other control
characters are dropped and tabs become spaces, so the line is exactly as wide
as layout measures it.
*/
func safeLine(line string) string {
	if !strings.ContainsFunc(line, func(r rune) bool { return r != '\x1b' && unicode.IsControl(r) }) {
		return line
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case r != '\x1b' && unicode.IsControl(r):
			return -1
		}
		return r
	}, line)
}
