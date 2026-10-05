package ui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/config/keys"
	"github.com/urdadx/nukri/internal/fs/fileops"
)

func (m Model) handleFileAction(action keys.Action) (tea.Model, tea.Cmd) {
	switch action {
	case keys.ActionCopy:
		return m.copySelected()
	case keys.ActionCut:
		return m.cutSelected()
	case keys.ActionPaste:
		return m.pasteClipboard()
	case keys.ActionCancelClipboard:
		return m.cancelClipboard()
	default:
		return m, nil
	}
}

func (m Model) cancelClipboard() (tea.Model, tea.Cmd) {
	if m.clipboard.Empty() {
		return m, nil
	}
	m.clipboard.Clear()
	m.status = "Clipboard cleared"
	return m, nil
}

func (m Model) clipboardStatus() string {
	if m.clipboard.Empty() {
		return ""
	}
	if m.clipboard.Operation == fileops.Move {
		return "1 file cut"
	}
	return "1 file copied"
}

func (m Model) copySelected() (tea.Model, tea.Cmd) {
	path := m.selectedPath()
	if path == "" {
		m.status = "Nothing selected"
		return m, nil
	}
	m.clipboard.Copy(path)
	m.status = ""
	return m, nil
}

func (m Model) cutSelected() (tea.Model, tea.Cmd) {
	path := m.selectedPath()
	if path == "" {
		m.status = "Nothing selected"
		return m, nil
	}
	m.clipboard.Cut(path)
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
	m.status = verb + " " + filepath.Base(m.clipboard.Source) + "..."
	return m, applyTransfer(m.data.CWD, []string{m.clipboard.Source}, m.clipboard.Operation, transferClipboard)
}

func (m Model) selectedPath() string {
	if m.data.Selected < 0 || m.data.Selected >= len(m.data.Entries) {
		return ""
	}
	return m.data.Entries[m.data.Selected].Entry.Path
}
