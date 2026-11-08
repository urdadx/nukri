package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urdadx/nukri/internal/archive"
)

// archiveState is the create-archive prompt and the running archive operation.
type archiveState struct {
	promptOpen bool
	name       string // archive name being typed in the prompt
	selectAll  bool   // the name is selected, so typing replaces it

	busy      bool
	operation string // shown in the footer while busy
	percent   float64
	progress  progress.Model
}

type archiveMsg struct {
	operation string
	output    string
	err       error
}

type archiveProgressMsg struct {
	percent float64
	stream  <-chan tea.Msg
}

func waitArchive(stream <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-stream
	}
}

func (m Model) openArchivePrompt() (tea.Model, tea.Cmd) {
	if m.archive.busy {
		m.status = "Archive operation already in progress"
		return m, nil
	}
	if m.data.Selected < 0 || m.data.Selected >= len(m.data.Entries) {
		m.status = "Nothing selected"
		return m, nil
	}
	m.archive.promptOpen = true
	m.archive.name = m.data.Entries[m.data.Selected].Entry.Name + ".zip"
	m.archive.selectAll = true
	return m, nil
}

func (m Model) handleArchivePromptKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "esc", "ctrl+c":
		m.archive.promptOpen = false
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.archive.name)
		if name == "" || filepath.Base(name) != name {
			m.status = "Archive name must be a filename"
			return m, nil
		}
		if _, err := archive.FormatFromPath(name); err != nil {
			m.status = err.Error()
			return m, nil
		}
		source := m.data.Entries[m.data.Selected].Entry.Path
		destination := filepath.Join(m.data.CWD, name)
		m.archive.promptOpen = false
		m.archive.busy = true
		m.archive.operation = "create"
		m.archive.percent = 0
		m.status = ""
		return m, createArchive(source, destination)
	case "backspace":
		if m.archive.selectAll {
			m.archive.name = ""
			m.archive.selectAll = false
		} else {
			runes := []rune(m.archive.name)
			if len(runes) > 0 {
				m.archive.name = string(runes[:len(runes)-1])
			}
		}
		return m, nil
	}
	if message.Type == tea.KeyRunes {
		if m.archive.selectAll {
			m.archive.name = ""
			m.archive.selectAll = false
		}
		m.archive.name += string(message.Runes)
	}
	return m, nil
}

func (m Model) extractSelectedArchive() (tea.Model, tea.Cmd) {
	if m.archive.busy {
		m.status = "Archive operation already in progress"
		return m, nil
	}
	if m.data.Selected < 0 || m.data.Selected >= len(m.data.Entries) {
		m.status = "Nothing selected"
		return m, nil
	}
	path := m.data.Entries[m.data.Selected].Entry.Path
	if _, err := archive.FormatFromPath(path); err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.archive.busy = true
	m.archive.operation = "extract"
	m.archive.percent = 0
	m.status = ""
	return m, extractArchive(path)
}

func createArchive(source, destination string) tea.Cmd {
	stream := make(chan tea.Msg, 8)
	go func() {
		output, err := archive.CreateWithProgress(destination, []string{source}, func(value archive.Progress) {
			percent := 0.0
			if value.Total > 0 {
				percent = min(1, float64(value.Completed)/float64(value.Total))
			}
			message := archiveProgressMsg{percent: percent, stream: stream}
			select {
			case stream <- message:
			default:
			}
		})
		stream <- archiveMsg{operation: "create", output: output, err: err}
		close(stream)
	}()
	return waitArchive(stream)
}

func extractArchive(source string) tea.Cmd {
	stream := make(chan tea.Msg, 8)
	go func() {
		output, err := archive.ExtractWithProgress(source, func(value archive.Progress) {
			percent := 0.0
			if value.Total > 0 {
				percent = min(1, float64(value.Completed)/float64(value.Total))
			}
			message := archiveProgressMsg{percent: percent, stream: stream}
			select {
			case stream <- message:
			default:
			}
		})
		stream <- archiveMsg{operation: "extract", output: output, err: err}
		close(stream)
	}()
	return waitArchive(stream)
}

func (m Model) archiveStatus() string {
	if !m.archive.busy {
		return ""
	}
	verb := "Creating"
	if m.archive.operation == "extract" {
		verb = "Extracting"
	}
	return fmt.Sprintf("%s archive %s %3.0f%%", verb, m.archive.progress.ViewAs(m.archive.percent), m.archive.percent*100)
}

func (m Model) renderArchivePrompt(base string) string {
	width := min(max(40, m.width/2), max(1, m.width-4))
	innerWidth := max(1, width-4)
	t := m.styles.Theme
	modal := lipgloss.NewStyle().Foreground(lipgloss.Color(t.ModalFG)).Background(lipgloss.Color(t.ModalBG))
	muted := modal.Foreground(lipgloss.Color(t.SidebarDivider))
	name := truncate(m.archive.name, max(1, innerWidth-3))
	content := []string{
		modal.Bold(true).Foreground(lipgloss.Color(t.ModalBorderActive)).Width(innerWidth).Render("Create archive"),
		modal.Width(innerWidth).Render("❯ " + name + "█"),
		muted.Width(innerWidth).Render("zip, tar, tar.gz, or tgz"),
		muted.Width(innerWidth).Render("enter create  esc cancel"),
	}
	dialog := modal.Width(width-2).Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.ModalBorderActive)).BorderBackground(lipgloss.Color(t.ModalBG)).
		Padding(0, 1).Render(strings.Join(content, "\n"))
	left := max(0, (m.width-lipgloss.Width(dialog))/2)
	top := max(0, (m.height-lipgloss.Height(dialog))/2)
	return overlaySearch(base, dialog, left, top)
}

func (message archiveMsg) statusText() string {
	if message.err != nil {
		return fmt.Sprintf("Archive %s failed: %v", message.operation, message.err)
	}
	if message.operation == "create" {
		return "Created " + filepath.Base(message.output)
	}
	return "Extracted to " + filepath.Base(message.output)
}
