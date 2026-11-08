package ui

import (
	"io"
	"os"
	"sync"

	"github.com/urdadx/nukri/internal/ui/browser"
)

/*
TerminalOutput is the program's output. Every write to the terminal goes
through it: bubbletea's frames, drag-and-drop replies, and preview images. A
lock keeps those from interleaving, and after each write the preview image is
brought up to date, so images are drawn after the text they sit on.

It implements term.File, so bubbletea still detects the terminal and its size.
*/
type TerminalOutput struct {
	mu      sync.Mutex
	file    *os.File
	visuals *browser.VisualState
}

// TerminalOutput returns the output for running m on file (normally stdout).
func (m Model) TerminalOutput(file *os.File) *TerminalOutput {
	return &TerminalOutput{file: file, visuals: m.visualState}
}

func (t *TerminalOutput) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, err := t.file.Write(p)
	if err == nil {
		t.visuals.Flush(t.file)
	}
	return n, err
}

func (t *TerminalOutput) Read(p []byte) (int, error) { return t.file.Read(p) }

func (t *TerminalOutput) Fd() uintptr { return t.file.Fd() }

// Close does nothing: the program doesn't own stdout.
func (t *TerminalOutput) Close() error { return nil }

var _ io.ReadWriteCloser = (*TerminalOutput)(nil)
