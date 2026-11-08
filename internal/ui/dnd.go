package ui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
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
}

// end forgets the outgoing drag.
func (d *dragState) end() {
	d.candidate = ""
	d.payload, d.active = nil, false
}

// resetDrop forgets the incoming drop negotiation.
func (d *dragState) resetDrop() {
	d.dropOperation, d.dropMIME = 0, 0
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
		operation := event.Operation
		if operation == kittydnd.Either {
			operation = kittydnd.Move
		}
		m.drag.dropOperation, m.drag.dropMIME = operation, event.MIME
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
		operation := m.drag.dropOperation
		m.drag.resetDrop()
		_, _ = io.WriteString(m.drag.output, kittydnd.FinishDropSequence(operation))
		m.status = "Dropping files..."
		return m, applyDrop(m.data.CWD, event.Paths, operation)
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
		payload, sequence := kittydnd.StartSequence([]string{path}, m.dragLabel(path))
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

func (m Model) dragLabel(path string) string {
	for _, item := range m.data.Entries {
		if item.Entry.Path == path {
			return item.Icon + " " + item.Entry.Name
		}
	}
	return ""
}
