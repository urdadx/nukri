package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/fs/fileops"
)

func (m Model) cancelClipboard() (tea.Model, tea.Cmd) {
	if m.clipboard.Empty() {
		return m, nil
	}
	m.clipboard.Clear()
	m.status = "Clipboard cleared"
	return m, nil
}

func (m Model) clipboardStatus() string {
	count := len(m.clipboard.Sources)
	if count == 0 {
		return ""
	}
	items := fmt.Sprintf("%d items", count)
	if count == 1 {
		items = "1 item"
	}
	if m.clipboard.Operation == fileops.Move {
		return items + " cut"
	}
	return items + " copied"
}

// copySelected puts the marked entries, or the focused one, on the clipboard.
func (m Model) copySelected() (tea.Model, tea.Cmd) {
	targets := m.targets()
	if len(targets) == 0 {
		m.status = "Nothing selected"
		return m, nil
	}
	m.clipboard.Copy(targets)
	m.clearMarks()
	m.status = ""
	return m, nil
}

// cutSelected marks the marked entries, or the focused one, to be moved.
func (m Model) cutSelected() (tea.Model, tea.Cmd) {
	targets := m.targets()
	if len(targets) == 0 {
		m.status = "Nothing selected"
		return m, nil
	}
	m.clipboard.Cut(targets)
	m.clearMarks()
	m.status = ""
	return m, nil
}

func (m Model) pasteClipboard() (tea.Model, tea.Cmd) {
	if m.clipboard.Empty() {
		m.status = "Clipboard is empty"
		return m, nil
	}
	if m.data.CWD == "" {
		m.status = "Paste destination is unavailable"
		return m, nil
	}
	verb := "Copying"
	if m.clipboard.Operation == fileops.Move {
		verb = "Moving"
	}
	m.status = verb + " " + countLabel(m.clipboard.Sources) + "..."
	return m, applyTransfer(m.data.CWD, m.clipboard.Sources, m.clipboard.Operation, transferClipboard)
}

func (m Model) selectedPath() string {
	if m.data.Selected < 0 || m.data.Selected >= len(m.data.Entries) {
		return ""
	}
	return m.data.Entries[m.data.Selected].Entry.Path
}
