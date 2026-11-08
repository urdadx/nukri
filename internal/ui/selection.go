package ui

import (
	"maps"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/ui/browser"
)

// toggleMark marks or unmarks the focused entry and moves to the next one, so
// holding Space marks a run of entries.
func (m Model) toggleMark() (tea.Model, tea.Cmd) {
	if m.data.Selected < 0 || m.data.Selected >= len(m.data.Entries) {
		return m, nil
	}
	path := m.data.Entries[m.data.Selected].Entry.Path
	marked := maps.Clone(m.data.Marked) // the map is shared with copies of the model
	if marked == nil {
		marked = map[string]bool{}
	}
	if marked[path] {
		delete(marked, path)
	} else {
		marked[path] = true
	}
	m.data.Marked = marked
	return m.moveSelection(1)
}

// clearMarks drops every mark and reports whether there were any.
func (m *Model) clearMarks() bool {
	had := len(m.data.Marked) > 0
	m.data.Marked = nil
	return had
}

// keepMarks drops marks for paths no longer listed, after a reload.
func (m *Model) keepMarks(entries []browser.Entry) {
	if len(m.data.Marked) == 0 {
		return
	}
	kept := map[string]bool{}
	for _, entry := range entries {
		if m.data.Marked[entry.Entry.Path] {
			kept[entry.Entry.Path] = true
		}
	}
	m.data.Marked = kept
}

// targets are the paths a file action applies to: the marked entries in listing
// order, or else the focused entry.
func (m Model) targets() []string {
	var paths []string
	if len(m.data.Marked) > 0 {
		for _, entry := range m.data.Entries {
			if m.data.Marked[entry.Entry.Path] {
				paths = append(paths, entry.Entry.Path)
			}
		}
		return paths
	}
	if m.data.Selected >= 0 && m.data.Selected < len(m.data.Entries) {
		paths = append(paths, m.data.Entries[m.data.Selected].Entry.Path)
	}
	return paths
}
