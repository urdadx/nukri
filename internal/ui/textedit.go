package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

/*
textEditor is a small multi-line editor for the create and rename dialogs. With
fixedLines the line count can't change, because each line belongs to one item.
*/
type textEditor struct {
	lines      [][]rune
	line, col  int
	fixedLines bool
}

func newTextEditor(value string, fixedLines bool) textEditor {
	editor := textEditor{fixedLines: fixedLines}
	for _, line := range strings.Split(value, "\n") {
		editor.lines = append(editor.lines, []rune(line))
	}
	editor.col = len(editor.lines[0])
	return editor
}

func (e textEditor) value() string {
	lines := make([]string, len(e.lines))
	for index, line := range e.lines {
		lines[index] = string(line)
	}
	return strings.Join(lines, "\n")
}

// update applies a key and reports whether it was handled.
func (e *textEditor) update(message tea.KeyMsg) bool {
	current := e.lines[e.line]
	switch message.String() {
	case "left", "ctrl+b":
		if e.col > 0 {
			e.col--
		} else if e.line > 0 {
			e.line--
			e.col = len(e.lines[e.line])
		}
	case "right", "ctrl+f":
		if e.col < len(current) {
			e.col++
		} else if e.line < len(e.lines)-1 {
			e.line++
			e.col = 0
		}
	case "up":
		if e.line > 0 {
			e.line--
			e.col = min(e.col, len(e.lines[e.line]))
		}
	case "down", "tab":
		if e.line < len(e.lines)-1 {
			e.line++
			e.col = min(e.col, len(e.lines[e.line]))
		}
	case "shift+tab":
		if e.line > 0 {
			e.line--
			e.col = min(e.col, len(e.lines[e.line]))
		}
	case "home", "ctrl+a":
		e.col = 0
	case "end", "ctrl+e":
		e.col = len(current)
	case "ctrl+u":
		e.lines[e.line] = current[e.col:]
		e.col = 0
	case "ctrl+k":
		e.lines[e.line] = current[:e.col]
	case "backspace", "ctrl+h":
		switch {
		case e.col > 0:
			e.lines[e.line] = append(current[:e.col-1:e.col-1], current[e.col:]...)
			e.col--
		case e.line > 0 && !e.fixedLines:
			previous := e.lines[e.line-1]
			e.col = len(previous)
			e.lines[e.line-1] = append(previous[:len(previous):len(previous)], current...)
			e.lines = append(e.lines[:e.line], e.lines[e.line+1:]...)
			e.line--
		}
	case "delete", "ctrl+d":
		switch {
		case e.col < len(current):
			e.lines[e.line] = append(current[:e.col:e.col], current[e.col+1:]...)
		case e.line < len(e.lines)-1 && !e.fixedLines:
			e.lines[e.line] = append(current[:len(current):len(current)], e.lines[e.line+1]...)
			e.lines = append(e.lines[:e.line+1], e.lines[e.line+2:]...)
		}
	case "alt+enter", "ctrl+j":
		if e.fixedLines {
			return false
		}
		rest := append([]rune(nil), current[e.col:]...)
		e.lines[e.line] = current[:e.col:e.col]
		e.lines = append(e.lines[:e.line+1], append([][]rune{rest}, e.lines[e.line+1:]...)...)
		e.line++
		e.col = 0
	default:
		if message.Type != tea.KeyRunes && message.Type != tea.KeySpace {
			return false
		}
		runes := message.Runes
		if message.Type == tea.KeySpace {
			runes = []rune{' '}
		}
		// Pasted text with newlines adds lines unless they're fixed.
		for _, r := range runes {
			if r == '\n' || r == '\r' {
				if !e.fixedLines {
					e.update(tea.KeyMsg{Type: tea.KeyCtrlJ})
				}
				continue
			}
			line := e.lines[e.line]
			e.lines[e.line] = append(line[:e.col:e.col], append([]rune{r}, line[e.col:]...)...)
			e.col++
		}
	}
	return true
}
