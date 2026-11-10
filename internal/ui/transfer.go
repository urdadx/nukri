package ui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/fs/fileops"
)

type transferOrigin uint8

const (
	transferClipboard transferOrigin = iota + 1
	transferDrop
)

type transferMsg struct {
	destination  string
	selectedPath string
	sources      []string
	operation    fileops.Operation
	origin       transferOrigin
	completed    int
	failed       []string // sources that were not transferred
	err          error
}

func applyTransfer(destination string, sources []string, operation fileops.Operation, origin transferOrigin) tea.Cmd {
	return func() tea.Msg {
		result := fileops.ApplyTransfer(destination, sources, operation)
		selectedPath := ""
		if len(result.Destinations) > 0 {
			selectedPath = result.Destinations[0]
		}
		return transferMsg{
			destination: destination, selectedPath: selectedPath, sources: sources,
			operation: operation, origin: origin, completed: len(result.Destinations),
			failed: result.Failed, err: errors.Join(result.Errors...),
		}
	}
}

func (message transferMsg) statusText() string {
	verb := "Copied"
	if message.operation == fileops.Move {
		verb = "Moved"
	}
	if message.completed == 0 && message.err != nil {
		prefix := "Paste failed: "
		if message.origin == transferDrop {
			prefix = "Drop failed: "
		}
		return prefix + message.err.Error()
	}
	status := fmt.Sprintf("%s %d item", verb, message.completed)
	if message.completed != 1 {
		status += "s"
	}
	if message.err != nil {
		status += "; some items failed"
	}
	return status
}
