package ui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/kittydnd"
)

// dragState tracks Kitty drag-and-drop: files dragged out of nukri and drops
// offered to it.
type dragState struct {
	output io.Writer // terminal the DnD protocol is written to; nil disables DnD

	candidate string // entry under the last left press, dragged if a drag starts
	payload   []byte
	active    bool // a drag out of nukri is in progress

	dropOperation kittydnd.Operation // negotiated for the drop being offered
	dropMIME      int
	dropTarget    string // directory under the pointer, where the drop lands
}

// end forgets the outgoing drag.
func (d *dragState) end() {
	d.candidate = ""
	d.payload, d.active = nil, false
}

// resetDrop forgets the incoming drop negotiation.
func (d *dragState) resetDrop() {
	d.dropOperation, d.dropMIME, d.dropTarget = 0, 0, ""
}

func (m Model) handleDndEvent(event kittydnd.Event) (tea.Model, tea.Cmd) {
	if m.drag.output == nil {
		return m, nil
	}
	switch event.Kind {
	case kittydnd.DropOffer:
		if m.drag.active {
			m.drag.resetDrop()
			if event.Final {
				_, _ = io.WriteString(m.drag.output, kittydnd.FinishDropSequence(0)+kittydnd.CancelSequence())
				m.drag.end()
			} else {
				_, _ = io.WriteString(m.drag.output, kittydnd.RejectDropSequence())
			}
			return m, nil
		}
		// When the source allows either, copy: a move would take the files out of
		// the app they were dragged from.
		operation := event.Operation
		if operation == kittydnd.Either {
			operation = kittydnd.Copy
		}
		target, ok := m.dropTarget(event.X, event.Y)
		if !ok {
			m.drag.resetDrop()
			sequence := kittydnd.RejectDropSequence()
			if event.Final {
				sequence = kittydnd.FinishDropSequence(0)
			}
			_, _ = io.WriteString(m.drag.output, sequence)
			return m, nil
		}
		m.drag.dropOperation, m.drag.dropMIME, m.drag.dropTarget = operation, event.MIME, target
		if event.Final {
			_, _ = io.WriteString(m.drag.output, kittydnd.RequestDropDataSequence(event.MIME))
		} else {
			_, _ = io.WriteString(m.drag.output, kittydnd.AcceptDropSequence(operation))
		}
	case kittydnd.DropLeave:
		m.drag.resetDrop()
	case kittydnd.DropUnsupported:
		sequence := kittydnd.RejectDropSequence()
		if event.Final {
			sequence = kittydnd.FinishDropSequence(0)
			if m.drag.active {
				sequence += kittydnd.CancelSequence()
				m.drag.end()
			}
		}
		_, _ = io.WriteString(m.drag.output, sequence)
	case kittydnd.DropData:
		if m.drag.active {
			_, _ = io.WriteString(m.drag.output, kittydnd.FinishDropSequence(0)+kittydnd.CancelSequence())
			m.drag.end()
			m.drag.resetDrop()
			return m, nil
		}
		if m.drag.dropOperation == 0 || event.MIME != m.drag.dropMIME {
			_, _ = io.WriteString(m.drag.output, kittydnd.FinishDropSequence(0))
			m.status = "Drop was not negotiated"
			return m, nil
		}
		if len(event.UnsupportedSchemes) > 0 || len(event.Paths) == 0 {
			_, _ = io.WriteString(m.drag.output, kittydnd.FinishDropSequence(0))
			if len(event.UnsupportedSchemes) > 0 {
				m.status = "Unsupported drop URI scheme: " + event.UnsupportedSchemes[0]
			} else {
				m.status = "Drop contains no local files"
			}
			m.drag.resetDrop()
			return m, nil
		}
		operation, target := m.drag.dropOperation, m.drag.dropTarget
		m.drag.resetDrop()
		_, _ = io.WriteString(m.drag.output, kittydnd.FinishDropSequence(operation))
		m.status = "Dropping files..."
		return m, applyDrop(target, event.Paths, operation)
	case kittydnd.DropDataError:
		_, _ = io.WriteString(m.drag.output, kittydnd.FinishDropSequence(0))
		m.drag.resetDrop()
		m.status = "Drop failed: " + event.Error
	case kittydnd.DragOffer:
		if m.drag.active {
			return m, nil
		}
		path := m.drag.candidate
		if path == "" {
			if index := m.entryIndexAt(event.X, event.Y); index >= 0 {
				path = m.data.Entries[index].Entry.Path
			}
		}
		paths, label := []string{path}, m.dragLabel(path)
		// Dragging a marked entry drags every marked entry.
		if m.data.Marked[path] {
			if paths = m.targets(); len(paths) > 1 {
				label = "" // StartSequence labels it "N selected files"
			}
		}
		payload, sequence := kittydnd.StartSequence(paths, label)
		_, _ = io.WriteString(m.drag.output, sequence)
		if len(payload) > 0 {
			m.drag.payload = payload
			m.drag.active = true
		}
	case kittydnd.DragDataRequested:
		_, _ = io.WriteString(m.drag.output, kittydnd.DataSequence(event.MIME, m.drag.payload))
	case kittydnd.DragEnded, kittydnd.DragError:
		m.drag.end()
	}
	return m, nil
}

/*
dropTarget returns the directory a drop over cell (x, y) lands in: the folder
under the pointer in the list or the sidebar, else the folder being viewed. ok
is false over the Trash place, which takes no drops.
*/
func (m Model) dropTarget(x, y int) (string, bool) {
	if item := m.sidebarItemAt(x, y); item != nil {
		return item.Path, item.Kind != core.Trash
	}
	if index := m.entryIndexAt(x, y); index >= 0 && m.data.Entries[index].Entry.IsDirectory() {
		return m.data.Entries[index].Entry.Path, true
	}
	return m.data.CWD, true
}

func (m Model) dragLabel(path string) string {
	for _, item := range m.data.Entries {
		if item.Entry.Path == path {
			return item.Icon + " " + item.Entry.Name
		}
	}
	return ""
}
