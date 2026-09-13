package ui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/fs/fileops"
	"github.com/urdadx/nukri/internal/kittydnd"
)

type dropMsg struct {
	destination  string
	selectedPath string
	operation    kittydnd.Operation
	completed    int
	err          error
}

func applyDrop(destination string, paths []string, operation kittydnd.Operation) tea.Cmd {
	return func() tea.Msg {
		fileOperation := fileops.Copy
		if operation == kittydnd.Move {
			fileOperation = fileops.Move
		}
		result := fileops.ApplyDrop(destination, paths, fileOperation)
		selectedPath := ""
		if len(result.Destinations) > 0 {
			selectedPath = result.Destinations[0]
		}
		return dropMsg{
			destination: destination, selectedPath: selectedPath, operation: operation,
			completed: len(result.Destinations), err: errors.Join(result.Errors...),
		}
	}
}

func (message dropMsg) statusText() string {
	verb := "Copied"
	if message.operation == kittydnd.Move {
		verb = "Moved"
	}
	if message.completed == 0 && message.err != nil {
		return "Drop failed: " + message.err.Error()
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
