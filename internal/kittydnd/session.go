package kittydnd

import (
	"io"
	"strings"
	"sync"
)

/*
Session runs the drag-and-drop protocol over one terminal. It parses what the
terminal sends and, when nukri and the terminal are on different machines,
moves file contents through the terminal: fetching dropped files
(FetchRemoteDrop) and sending dragged ones (SendDragEntry).
*/
type Session struct {
	terminal io.Writer // requests and drag data are written here; may be nil
	events   chan<- Event
	parser   Parser // used only by the Filter goroutine

	mu    sync.Mutex
	fetch *fetch // remote drop being received, or nil

	sendOnce sync.Once
	sends    chan dragEntry
	handles  int // last directory handle given to the terminal
}

// NewSession returns a session writing protocol replies to terminal and
// sending what it decodes on events.
func NewSession(terminal io.Writer, events chan<- Event) *Session {
	return &Session{terminal: terminal, events: events}
}

// handle takes one complete OSC 72 sequence from the terminal.
func (s *Session) handle(sequence []byte) {
	if s.receive(sequence) {
		return
	}
	if event, ok := s.parser.Parse(sequence); ok {
		s.events <- event
	}
}

// splitSequence returns the metadata and payload of an OSC 72 sequence.
func splitSequence(sequence []byte) (metadata, payload string) {
	body := strings.TrimSuffix(strings.TrimSuffix(string(sequence), "\x1b\\"), "\a")
	body = strings.TrimPrefix(body, "\x1b]72;")
	metadata, payload, _ = strings.Cut(body, ";")
	return metadata, payload
}

// writeSequence writes one OSC 72 sequence to the terminal.
func (s *Session) writeSequence(metadata string) error {
	_, err := io.WriteString(s.terminal, "\x1b]72;"+metadata+"\x1b\\")
	return err
}
