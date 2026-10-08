package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urdadx/nukri/internal/archive"
)

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
	if m.archiveBusy {
		m.status = "Archive operation already in progress"
		return m, nil
	}
	if m.data.Selected < 0 || m.data.Selected >= len(m.data.Entries) {
		m.status = "Nothing selected"
		return m, nil
	}
	m.archivePromptOpen = true
	m.archiveName = m.data.Entries[m.data.Selected].Entry.Name + ".zip"
	m.archiveSelectAll = true
	return m, nil
}

func (m Model) handleArchivePromptKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "esc", "ctrl+c":
		m.archivePromptOpen = false
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.archiveName)
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
		m.archivePromptOpen = false
		m.archiveBusy = true
		m.archiveOperation = "create"
		m.archivePercent = 0
		m.status = ""
		return m, createArchive(source, destination)
	case "backspace":
		if m.archiveSelectAll {
			m.archiveName = ""
			m.archiveSelectAll = false
		} else {
			runes := []rune(m.archiveName)
			if len(runes) > 0 {
				m.archiveName = string(runes[:len(runes)-1])
			}
		}
		return m, nil
	}
	if message.Type == tea.KeyRunes {
		if m.archiveSelectAll {
			m.archiveName = ""
			m.archiveSelectAll = false
		}
		m.archiveName += string(message.Runes)
	}
	return m, nil
}

func (m Model) extractSelectedArchive() (tea.Model, tea.Cmd) {
	if m.archiveBusy {
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
	m.archiveBusy = true
	m.archiveOperation = "extract"
	m.status = "Extracting " + filepath.Base(path) + "..."
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
	return func() tea.Msg {
		output, err := archive.Extract(source)
		return archiveMsg{operation: "extract", output: output, err: err}
	}
}

func (m Model) archiveStatus() string {
	if !m.archiveBusy {
		return ""
	}
	if m.archiveOperation != "create" {
		return "Extracting archive..."
	}
	return fmt.Sprintf("Creating archive %s %3.0f%%", m.archiveProgress.ViewAs(m.archivePercent), m.archivePercent*100)
}

func (m Model) renderArchivePrompt(base string) string {
	width := min(max(40, m.width/2), max(1, m.width-4))
	innerWidth := max(1, width-4)
	t := m.styles.Theme
	modal := lipgloss.NewStyle().Foreground(lipgloss.Color(t.ModalFG)).Background(lipgloss.Color(t.ModalBG))
	muted := modal.Foreground(lipgloss.Color(t.SidebarDivider))
	name := truncate(m.archiveName, max(1, innerWidth-3))
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
