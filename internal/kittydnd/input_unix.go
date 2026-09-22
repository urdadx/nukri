//go:build unix

package kittydnd

import (
	"io"
	"os"

	"github.com/charmbracelet/x/term"
)

var _ term.File = (*Input)(nil)

type Input struct {
	tty    *os.File
	reader *io.PipeReader
	events chan Event
	parser Parser
}

func OpenInput() (*Input, error) {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return nil, err
	}
	reader, writer := io.Pipe()
	input := &Input{tty: tty, reader: reader, events: make(chan Event, 8)}
	go input.filter(writer)
	return input, nil
}

func (i *Input) Read(p []byte) (int, error) { return i.reader.Read(p) }

func (i *Input) Write(p []byte) (int, error) { return i.tty.Write(p) }

func (i *Input) Fd() uintptr { return i.tty.Fd() }

func (i *Input) Events() <-chan Event { return i.events }

func (i *Input) Close() error {
	_ = i.reader.Close()
	return i.tty.Close()
}

func (i *Input) filter(writer *io.PipeWriter) {
	defer writer.Close()
	defer close(i.events)
	buffer := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)
	for {
		n, err := i.tty.Read(chunk)
		if n > 0 {
			buffer = append(buffer, chunk[:n]...)
			buffer = i.flush(buffer, writer)
		}
		if err != nil {
			return
		}
	}
}

func (i *Input) flush(buffer []byte, writer io.Writer) []byte {
	for len(buffer) > 0 {
		start := indexBytes(buffer, []byte("\x1b]72;"))
		if start < 0 {
			keep := partialPrefixLength(buffer, []byte("\x1b]72;"))
			// A lone escape is a complete keyboard input. Only retain partial
			// prefixes once the OSC introducer has also arrived.
			if keep == 1 {
				keep = 0
			}
			_, _ = writer.Write(buffer[:len(buffer)-keep])
			return buffer[len(buffer)-keep:]
		}
		if start > 0 {
			_, _ = writer.Write(buffer[:start])
			buffer = buffer[start:]
		}
		end := oscEnd(buffer)
		if end < 0 {
			return buffer
		}
		if event, ok := i.parser.Parse(buffer[:end]); ok {
			i.events <- event
		}
		buffer = buffer[end:]
	}
	return buffer
}

func oscEnd(buffer []byte) int {
	for index := len("\x1b]72;"); index < len(buffer); index++ {
		if buffer[index] == '\a' {
			return index + 1
		}
		if buffer[index] == '\x1b' && index+1 < len(buffer) && buffer[index+1] == '\\' {
			return index + 2
		}
	}
	return -1
}

func indexBytes(data, separator []byte) int {
	for index := 0; index+len(separator) <= len(data); index++ {
		match := true
		for offset := range separator {
			if data[index+offset] != separator[offset] {
				match = false
				break
			}
		}
		if match {
			return index
		}
	}
	return -1
}

func partialPrefixLength(data, prefix []byte) int {
	for length := min(len(data), len(prefix)-1); length > 0; length-- {
		match := true
		for index := 0; index < length; index++ {
			if data[len(data)-length+index] != prefix[index] {
				match = false
				break
			}
		}
		if match {
			return length
		}
	}
	return 0
}
