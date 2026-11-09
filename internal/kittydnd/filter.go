package kittydnd

import (
	"bytes"
	"io"
)

var oscIntroducer = []byte("\x1b]72;")

// maxPendingOSC bounds one buffered drag-and-drop sequence: a full base64 drop
// plus slack. Without it, an introducer that never terminates would hold back
// every later byte, keyboard input included.
const maxPendingOSC = maxEncodedDropBytes + 4096

// FilterInput copies terminal input from src to dst, removing Kitty
// drag-and-drop OSC 72 sequences and sending what they decode to on events. It
// returns when src fails or ends.
func FilterInput(src io.Reader, dst io.Writer, events chan<- Event) {
	var parser Parser
	scanned := 0
	buffer := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)
	for {
		n, err := src.Read(chunk)
		if n > 0 {
			buffer = flushInput(append(buffer, chunk[:n]...), dst, &parser, events, &scanned)
		}
		if err != nil {
			return
		}
	}
}

// flushInput writes everything in buffer that is not part of a DnD sequence and
// returns what must wait for more input. scanned records how far a pending
// sequence was already searched for its terminator, so a long one isn't
// rescanned from the start on every read.
func flushInput(buffer []byte, dst io.Writer, parser *Parser, events chan<- Event, scanned *int) []byte {
	for len(buffer) > 0 {
		start := bytes.Index(buffer, oscIntroducer)
		if start < 0 {
			keep := partialPrefixLength(buffer, oscIntroducer)
			// A lone escape is a complete keyboard input. Only retain partial
			// prefixes once the OSC introducer has also arrived.
			if keep == 1 {
				keep = 0
			}
			_, _ = dst.Write(buffer[:len(buffer)-keep])
			return buffer[len(buffer)-keep:]
		}
		if start > 0 {
			_, _ = dst.Write(buffer[:start])
			buffer = buffer[start:]
		}
		end := oscEnd(buffer, *scanned)
		if end < 0 {
			if len(buffer) > maxPendingOSC {
				// Never terminated: drop it so input flows again.
				*scanned = 0
				return buffer[:0]
			}
			*scanned = len(buffer)
			return buffer
		}
		*scanned = 0
		if event, ok := parser.Parse(buffer[:end]); ok {
			events <- event
		}
		buffer = buffer[end:]
	}
	return buffer
}

// oscEnd returns the index just past the sequence's terminator, searching from
// from, or -1 while it is incomplete.
func oscEnd(buffer []byte, from int) int {
	// Back up one byte: an ESC ending the last read may start the ST terminator.
	for index := max(len(oscIntroducer), from-1); index < len(buffer); index++ {
		if buffer[index] == '\a' {
			return index + 1
		}
		if buffer[index] == '\x1b' && index+1 < len(buffer) && buffer[index+1] == '\\' {
			return index + 2
		}
	}
	return -1
}

// partialPrefixLength returns the length of the longest proper prefix of prefix
// that data ends with.
func partialPrefixLength(data, prefix []byte) int {
	for length := min(len(data), len(prefix)-1); length > 0; length-- {
		if bytes.HasSuffix(data, prefix[:length]) {
			return length
		}
	}
	return 0
}
