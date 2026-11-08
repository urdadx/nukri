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
	FilterInput(i.tty, writer, i.events)
}
