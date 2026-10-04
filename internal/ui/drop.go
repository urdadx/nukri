package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/fs/fileops"
	"github.com/urdadx/nukri/internal/kittydnd"
)

func applyDrop(destination string, paths []string, operation kittydnd.Operation) tea.Cmd {
	transferOperation := fileops.Copy
	if operation == kittydnd.Move {
		transferOperation = fileops.Move
	}
	return applyTransfer(destination, paths, transferOperation, transferDrop)
}
