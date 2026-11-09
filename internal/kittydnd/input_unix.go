//go:build unix

package kittydnd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/muesli/cancelreader"
)

var _ term.File = (*Input)(nil)

// ErrUnsupported is returned by OpenInput when the terminal doesn't support
// the drag-and-drop protocol.
var ErrUnsupported = errors.New("terminal does not support drag and drop")

type Input struct {
	*Session
	tty    *os.File
	reader *io.PipeReader
	events chan Event
}

/*
OpenInput opens the terminal for input with drag-and-drop sequences filtered
out, after asking the terminal whether it supports them. Replies and file
contents for transfers with another machine are written to terminal, which
must be safe to share with the program's own output.
*/
func OpenInput(terminal io.Writer) (*Input, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	supported, rest, err := probeTerminal(tty)
	if err != nil || !supported {
		_ = tty.Close()
		if err == nil {
			err = ErrUnsupported
		}
		return nil, err
	}
	reader, writer := io.Pipe()
	events := make(chan Event, 8)
	input := &Input{Session: NewSession(terminal, events), tty: tty, reader: reader, events: events}
	go input.filter(io.MultiReader(bytes.NewReader(rest), tty), writer)
	return input, nil
}

// probeTerminal sends ProbeSequence to tty and reads the answers in raw mode,
// giving up after probeTimeout.
func probeTerminal(tty *os.File) (bool, []byte, error) {
	state, err := term.MakeRaw(tty.Fd())
	if err != nil {
		return false, nil, err
	}
	defer func() { _ = term.Restore(tty.Fd(), state) }()
	reader, err := cancelreader.NewReader(tty)
	if err != nil {
		return false, nil, err
	}
	defer reader.Close()
	if _, err := tty.WriteString(ProbeSequence()); err != nil {
		return false, nil, err
	}
	timer := time.AfterFunc(probeTimeout, func() { reader.Cancel() })
	defer timer.Stop()
	supported, rest := Probe(reader)
	return supported, rest, nil
}

func (i *Input) Read(p []byte) (int, error) { return i.reader.Read(p) }

func (i *Input) Write(p []byte) (int, error) { return i.tty.Write(p) }

func (i *Input) Fd() uintptr { return i.tty.Fd() }

func (i *Input) Events() <-chan Event { return i.events }

func (i *Input) Close() error {
	_ = i.reader.Close()
	return i.tty.Close()
}

func (i *Input) filter(source io.Reader, writer *io.PipeWriter) {
	defer writer.Close()
	defer close(i.events)
	i.Session.Filter(source, writer)
}
