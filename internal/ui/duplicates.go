package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urdadx/nukri/internal/fs/duplicates"
	"github.com/urdadx/nukri/internal/fs/trash"
	"github.com/urdadx/nukri/internal/ui/browser"
)

/*
duplicateState is the duplicate finder: a background scan of the current tree
for files with identical content, then a list of the groups it found. Rows
flatten the groups: a heading per group followed by its files.
*/
type duplicateState struct {
	open     bool
	root     string
	scanning bool
	cancel   context.CancelFunc
	token    int // identifies the current scan; stale events are dropped
	progress duplicates.Progress
	groups   []duplicates.Group
	rows     []duplicateRow
	selected int // index into rows, always a file row
	err      string
}

type duplicateRow struct {
	group int
	path  string // "" for the group heading
}

type duplicateEvent struct {
	token    int
	progress duplicates.Progress
	groups   []duplicates.Group
	done     bool
	err      error
}

type duplicateMsg struct {
	event  duplicateEvent
	stream <-chan duplicateEvent
}

func waitDuplicates(stream <-chan duplicateEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-stream
		if !ok {
			return nil
		}
		return duplicateMsg{event: event, stream: stream}
	}
}

func (m Model) openDuplicates() (tea.Model, tea.Cmd) {
	if m.data.CWD == "" {
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	token := m.duplicates.token + 1
	m.duplicates = duplicateState{open: true, root: m.data.CWD, scanning: true, cancel: cancel, token: token}
	stream := make(chan duplicateEvent, 1)
	root := m.data.CWD
	go func() {
		defer close(stream)
		groups, err := duplicates.Find(ctx, root, duplicates.Options{}, func(progress duplicates.Progress) {
			// Drop progress the UI hasn't caught up with; the next one replaces it.
			select {
			case stream <- duplicateEvent{token: token, progress: progress}:
			default:
			}
		})
		if ctx.Err() != nil {
			return
		}
		stream <- duplicateEvent{token: token, groups: groups, done: true, err: err}
	}()
	return m, waitDuplicates(stream)
}

func (m Model) closeDuplicates() Model {
	if m.duplicates.cancel != nil {
		m.duplicates.cancel()
	}
	m.duplicates = duplicateState{token: m.duplicates.token}
	return m
}

func (m Model) handleDuplicateEvent(message duplicateMsg) (tea.Model, tea.Cmd) {
	state := &m.duplicates
	if !state.open || message.event.token != state.token {
		return m, nil
	}
	if !message.event.done {
		state.progress = message.event.progress
		return m, waitDuplicates(message.stream)
	}
	state.scanning = false
	if message.event.err != nil {
		state.err = message.event.err.Error()
		return m, nil
	}
	state.setGroups(message.event.groups, "")
	return m, nil
}

// setGroups rebuilds the rows and selects keep (or the first file).
func (s *duplicateState) setGroups(groups []duplicates.Group, keep string) {
	s.groups = groups
	s.rows = s.rows[:0]
	s.selected = -1
	for index, group := range groups {
		s.rows = append(s.rows, duplicateRow{group: index})
		for _, path := range group.Paths {
			if path == keep || (s.selected < 0 && keep == "") {
				s.selected = len(s.rows)
			}
			s.rows = append(s.rows, duplicateRow{group: index, path: path})
		}
	}
	if s.selected < 0 && len(s.rows) > 1 {
		s.selected = 1
	}
}

// move steps the selection by delta file rows, skipping headings.
func (s *duplicateState) move(delta int) {
	for next := s.selected + delta; next >= 0 && next < len(s.rows); next += delta {
		if s.rows[next].path != "" {
			s.selected = next
			return
		}
	}
}

func (s *duplicateState) selectedPath() string {
	if s.selected < 0 || s.selected >= len(s.rows) {
		return ""
	}
	return s.rows[s.selected].path
}

func (m Model) handleDuplicateKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	state := &m.duplicates
	switch message.String() {
	case "esc", "q", "alt+d":
		return m.closeDuplicates(), nil
	case "up", "k":
		state.move(-1)
	case "down", "j":
		state.move(1)
	case "enter":
		path := state.selectedPath()
		if path == "" {
			return m, nil
		}
		m = m.closeDuplicates()
		return m.navigateTo(filepath.Dir(path), path, true)
	case "d", "delete":
		return m.trashDuplicate()
	}
	return m, nil
}

/*
trashDuplicate moves the selected copy to the trash. The last copy in a group
is kept, so the finder can never remove the only remaining one.
*/
func (m Model) trashDuplicate() (tea.Model, tea.Cmd) {
	state := &m.duplicates
	path := state.selectedPath()
	if path == "" {
		return m, nil
	}
	group := state.rows[state.selected].group
	if len(state.groups[group].Paths) < 2 {
		m.status = "This is the last copy; it stays"
		return m, nil
	}
	if _, err := trash.Trash(path); err != nil {
		m.status = "Trash failed: " + err.Error()
		return m, nil
	}
	m.status = "Moved " + filepath.Base(path) + " to trash"
	groups := make([]duplicates.Group, 0, len(state.groups))
	for index, current := range state.groups {
		if index == group {
			paths := make([]string, 0, len(current.Paths)-1)
			for _, other := range current.Paths {
				if other != path {
					paths = append(paths, other)
				}
			}
			if len(paths) < 2 {
				continue // no longer a duplicate
			}
			current.Paths = paths
		}
		groups = append(groups, current)
	}
	// Keep the selection near where it was.
	next := ""
	for _, row := range state.rows[state.selected+1:] {
		if row.path != "" {
			next = row.path
			break
		}
	}
	state.setGroups(groups, next)
	return m, nil
}

func (m Model) renderDuplicates(base string) string {
	state := m.duplicates
	width, height := m.searchDialogSize()
	innerWidth := max(1, width-4)
	t := m.styles.Theme
	modal := lipgloss.NewStyle().Foreground(lipgloss.Color(t.ModalFG)).Background(lipgloss.Color(t.ModalBG))
	muted := modal.Foreground(lipgloss.Color(t.SidebarDivider))
	heading := modal.Foreground(lipgloss.Color(t.ModalBorderActive)).Bold(true)
	selected := modal.Foreground(lipgloss.Color(t.FilePanelItemSelectedFG)).Background(lipgloss.Color(t.FilePanelItemSelectedBG)).Bold(true)

	status := ""
	switch {
	case state.scanning && state.progress.Phase == duplicates.Comparing:
		status = fmt.Sprintf("Comparing %d candidates…", state.progress.Candidates)
	case state.scanning:
		status = fmt.Sprintf("Scanning… %d files", state.progress.Scanned)
	case state.err == "":
		status = fmt.Sprintf("%d groups · %s reclaimable", len(state.groups), formatBytes(duplicates.Wasted(state.groups)))
	}
	title := heading.Render("Duplicate files")
	gap := max(1, innerWidth-lipgloss.Width(title)-lipgloss.Width(status))
	content := []string{modal.Width(innerWidth).Render(title + modal.Render(strings.Repeat(" ", gap)) + muted.Render(status))}
	content = append(content, muted.Width(innerWidth).Render(truncate(state.root, innerWidth)), modal.Width(innerWidth).Render(""))

	visible := max(1, height-6)
	switch {
	case state.err != "":
		content = append(content, muted.Width(innerWidth).Render(truncate(state.err, innerWidth)))
	case !state.scanning && len(state.rows) == 0:
		content = append(content, muted.Width(innerWidth).Render("No duplicate files found"))
	default:
		start := searchWindowStart(state.selected, len(state.rows), visible)
		for index := start; index < min(len(state.rows), start+visible); index++ {
			row := state.rows[index]
			group := state.groups[row.group]
			if row.path == "" {
				label := fmt.Sprintf("%d copies · %s each", len(group.Paths), formatBytes(group.Size))
				content = append(content, heading.Width(innerWidth).Render(truncate(label, innerWidth)))
				continue
			}
			relative, err := filepath.Rel(state.root, row.path)
			if err != nil {
				relative = row.path
			}
			line := truncate("  "+browser.SafeName(relative), innerWidth)
			if index == state.selected {
				content = append(content, selected.Width(innerWidth).Render(line))
			} else {
				content = append(content, modal.Width(innerWidth).Render(line))
			}
		}
	}
	for len(content) < height-3 {
		content = append(content, modal.Width(innerWidth).Render(""))
	}
	content = append(content, muted.Width(innerWidth).Render("enter go to file  d trash copy  esc close"))
	box := modal.Width(width-2).Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.ModalBorderActive)).BorderBackground(lipgloss.Color(t.ModalBG)).
		Padding(0, 1).Render(strings.Join(content, "\n"))
	left, top := m.searchDialogPosition(lipgloss.Width(box), lipgloss.Height(box))
	return overlaySearch(base, box, left, top)
}

// formatBytes is a short human-readable size, such as "4.2 MB".
func formatBytes(size int64) string {
	const unit = 1000
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value, exponent := float64(size)/unit, 0
	for value >= unit && exponent < 4 {
		value /= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %cB", value, "kMGTP"[exponent])
}
