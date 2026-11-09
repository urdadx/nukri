package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type dialogKind int

const (
	dialogText    dialogKind = iota // multi-line text input
	dialogList                      // pick one row
	dialogConfirm                   // yes or no
)

type dialogChoice struct {
	label, detail string
	key           string // single-key shortcut, optional
	value         any
}

/*
dialog is a modal over the browser used by the file actions: text input for
create and rename, a list for copy path and Open With, and confirmations.
submit runs on Enter (or a choice shortcut, or "y"); it returns the model to
continue with, and leaves the dialog open by setting m.dialog again, for example
to show validation errors.
*/
type dialog struct {
	kind   dialogKind
	title  string
	detail string // line under the title
	hint   string // key help at the bottom

	input      textEditor
	lineErrors map[int]string // validation errors by 0-based input line

	choices  []dialogChoice
	selected int

	submit func(m Model, d *dialog) (Model, tea.Cmd)
	// Optional: moved runs when a list's selection changes, cancel on Esc.
	moved  func(m Model, d *dialog) Model
	cancel func(m Model) Model
}

const dialogMaxInputRows = 12

// newTextDialog edits value. With multiline, Alt+Enter or Ctrl+J add lines;
// otherwise the line count is fixed, one per item being edited.
func newTextDialog(title, detail, value string, multiline bool, submit func(Model, *dialog) (Model, tea.Cmd)) *dialog {
	hint := "enter confirm  ↑↓ line  esc cancel"
	if multiline {
		hint = "enter confirm  alt+enter new line  esc cancel"
	}
	return &dialog{kind: dialogText, title: title, detail: detail, hint: hint,
		input: newTextEditor(value, !multiline), submit: submit}
}

func newChoiceDialog(title, detail string, choices []dialogChoice, submit func(Model, *dialog) (Model, tea.Cmd)) *dialog {
	return &dialog{kind: dialogList, title: title, detail: detail, choices: choices,
		hint: "enter select  esc cancel", submit: submit}
}

func newConfirmDialog(title, detail string, submit func(Model, *dialog) (Model, tea.Cmd)) *dialog {
	return &dialog{kind: dialogConfirm, title: title, detail: detail, hint: "y confirm  n cancel", submit: submit}
}

// chosen returns the selected choice's value.
func (d *dialog) chosen() any {
	if d.selected < 0 || d.selected >= len(d.choices) {
		return nil
	}
	return d.choices[d.selected].value
}

// lines returns the text input split into lines.
func (d *dialog) lines() []string {
	return strings.Split(d.input.value(), "\n")
}

func (m Model) handleDialogKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	d := m.dialog
	if message.Type == tea.KeyEsc {
		m.dialog = nil
		if d.cancel != nil {
			m = d.cancel(m)
		}
		return m, nil
	}
	switch d.kind {
	case dialogConfirm:
		switch message.String() {
		case "y", "Y", "enter":
			m.dialog = nil
			return d.submit(m, d)
		case "n", "N", "q":
			m.dialog = nil
		}
		return m, nil
	case dialogList:
		switch message.String() {
		case "up", "k", "ctrl+p":
			d.selected = max(0, d.selected-1)
			if d.moved != nil {
				m = d.moved(m, d)
			}
			return m, nil
		case "down", "j", "ctrl+n":
			d.selected = min(len(d.choices)-1, d.selected+1)
			if d.moved != nil {
				m = d.moved(m, d)
			}
			return m, nil
		case "enter":
			m.dialog = nil
			return d.submit(m, d)
		}
		for index, choice := range d.choices {
			if choice.key != "" && message.String() == choice.key {
				d.selected = index
				m.dialog = nil
				return d.submit(m, d)
			}
		}
		return m, nil
	default:
		// Plain Enter only: Alt+Enter is also KeyEnter, and adds a line.
		if message.String() == "enter" {
			m.dialog = nil
			return d.submit(m, d)
		}
		d.input.update(message)
		return m, nil
	}
}

/*
renderHint draws a key hint such as "y confirm  n cancel" across width: each
key in key's style, its description in description's. Parts are separated by
two spaces, and a part's key is its first word.
*/
func renderHint(hint string, key, description lipgloss.Style, width int) string {
	var parts []string
	for _, part := range strings.Split(hint, "  ") {
		name, text, _ := strings.Cut(part, " ")
		rendered := key.Render(name)
		if text != "" {
			rendered += description.Render(" " + text)
		}
		parts = append(parts, rendered)
	}
	line := ansi.Truncate(strings.Join(parts, description.Render("  ")), width, "…")
	return line + description.Render(strings.Repeat(" ", max(0, width-lipgloss.Width(line))))
}

func (m Model) renderDialog(base string) string {
	d := m.dialog
	width := min(max(48, m.width*2/3), max(1, m.width-4))
	innerWidth := max(1, width-4)
	t := m.styles.Theme
	modal := lipgloss.NewStyle().Foreground(lipgloss.Color(t.ModalFG)).Background(lipgloss.Color(t.ModalBG))
	muted := modal.Foreground(lipgloss.Color(t.ModalMuted()))
	accent := modal.Foreground(lipgloss.Color(t.ModalTitle())).Bold(true)
	errorStyle := modal.Foreground(lipgloss.Color(t.ModalError()))
	selected := modal.Foreground(lipgloss.Color(t.FilePanelItemSelectedFG)).Background(lipgloss.Color(t.FilePanelItemSelectedBG)).Bold(true)
	content := []string{accent.Width(innerWidth).Render(truncate(d.title, innerWidth))}
	if d.detail != "" {
		// A confirmation's detail names what it acts on, so it reads as text;
		// elsewhere the detail is help, so it stays muted.
		detail := muted
		if d.kind == dialogConfirm {
			detail = modal
		}
		content = append(content, detail.Width(innerWidth).Render(truncate(d.detail, innerWidth)))
	}
	content = append(content, modal.Width(innerWidth).Render(""))
	switch d.kind {
	case dialogText:
		lines := d.input.lines
		start := max(0, min(d.input.line-dialogMaxInputRows/2, len(lines)-dialogMaxInputRows))
		for index := start; index < min(len(lines), start+dialogMaxInputRows); index++ {
			marker := modal.Render("  ")
			if _, ok := d.lineErrors[index]; ok {
				marker = errorStyle.Render("! ")
			}
			text := string(lines[index])
			if index == d.input.line {
				col := d.input.col
				cursor := " "
				if col < len(lines[index]) {
					cursor = string(lines[index][col])
				}
				text = string(lines[index][:col]) + selected.Render(cursor) + modal.Render(string(lines[index][min(col+1, len(lines[index])):]))
			}
			content = append(content, modal.Width(innerWidth).Render(marker+ansi.Truncate(text, innerWidth-2, "…")))
		}
		shown := 0
		for line := 0; line < len(lines) && shown < 3; line++ {
			if message, ok := d.lineErrors[line]; ok {
				content = append(content, errorStyle.Width(innerWidth).Render(truncate(fmt.Sprintf("line %d: %s", line+1, message), innerWidth)))
				shown++
			}
		}
	case dialogList:
		visible := min(len(d.choices), max(3, m.height-12))
		start := searchWindowStart(d.selected, len(d.choices), visible)
		for index := start; index < min(len(d.choices), start+visible); index++ {
			choice := d.choices[index]
			prefix := "   "
			if choice.key != "" {
				prefix = " " + choice.key + " "
			}
			label := choice.label
			if choice.detail != "" {
				label += "  " + choice.detail
			}
			row := truncate(prefix+label, innerWidth)
			if index == d.selected {
				content = append(content, selected.Width(innerWidth).Render(row))
			} else {
				content = append(content, modal.Width(innerWidth).Render(row))
			}
		}
	}
	content = append(content, modal.Width(innerWidth).Render(""), renderHint(d.hint, accent, muted, innerWidth))
	box := modal.Width(width-2).Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.ModalBorderActive)).BorderBackground(lipgloss.Color(t.ModalBG)).
		Padding(0, 1).Render(strings.Join(content, "\n"))
	left := max(0, (m.width-lipgloss.Width(box))/2)
	top := max(0, (m.height-lipgloss.Height(box))/2)
	return overlaySearch(base, box, left, top)
}
