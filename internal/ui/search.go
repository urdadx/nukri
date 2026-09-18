package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urdadx/nukri/internal/core"
)

const searchMatchLimit = 100

type searchMsg struct {
	event  searchEvent
	stream <-chan searchEvent
}

func waitSearch(stream <-chan searchEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-stream
		if !ok {
			return nil
		}
		return searchMsg{event: event, stream: stream}
	}
}

func (m Model) openSearch() (tea.Model, tea.Cmd) {
	if m.data.CWD == "" {
		return m, nil
	}
	m.searchToken++
	m.searchOpen = true
	m.searchQuery = ""
	m.searchSelected = 0
	m.searchCandidates = nil
	m.searchMatches = nil
	m.searchLoading = true
	m.searchScanned = 0
	m.searchError = ""
	stream := m.searchPool.Submit(searchRequest{token: m.searchToken, root: m.data.CWD})
	return m, waitSearch(stream)
}

func (m Model) closeSearch() Model {
	m.searchOpen = false
	m.searchToken++
	m.searchLoading = false
	m.searchPool.Cancel()
	return m
}

func (m Model) handleSearchEvent(message searchMsg) (tea.Model, tea.Cmd) {
	if !m.searchOpen || message.event.token != m.searchToken {
		return m, nil
	}
	firstNew := len(m.searchCandidates)
	m.searchCandidates = append(m.searchCandidates, message.event.candidates...)
	m.searchScanned = message.event.scanned
	m.searchLoading = !message.event.done
	if message.event.err != nil {
		m.searchError = message.event.err.Error()
	}
	m.extendSearchMatches(firstNew)
	if message.event.done {
		return m, nil
	}
	return m, waitSearch(message.stream)
}

func (m *Model) refreshSearchMatches() {
	m.searchMatches = nil
	m.extendSearchMatches(0)
}

func (m *Model) extendSearchMatches(firstNew int) {
	query := strings.ToLower(strings.TrimSpace(m.searchQuery))
	type ranked struct {
		index int
		score int
	}
	values := make([]ranked, 0, len(m.searchMatches)+len(m.searchCandidates)-firstNew)
	for _, index := range m.searchMatches {
		score, ok := fuzzySearchScore(query, m.searchCandidates[index])
		if ok {
			values = append(values, ranked{index: index, score: score})
		}
	}
	for index := firstNew; index < len(m.searchCandidates); index++ {
		candidate := m.searchCandidates[index]
		score, ok := fuzzySearchScore(query, candidate)
		if !ok {
			continue
		}
		values = append(values, ranked{index: index, score: score})
	}
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].score != values[j].score {
			return values[i].score > values[j].score
		}
		return len(m.searchCandidates[values[i].index].relative) < len(m.searchCandidates[values[j].index].relative)
	})
	if len(values) > searchMatchLimit {
		values = values[:searchMatchLimit]
	}
	m.searchMatches = make([]int, len(values))
	for index, value := range values {
		m.searchMatches[index] = value.index
	}
	if len(m.searchMatches) == 0 {
		m.searchSelected = 0
	} else {
		m.searchSelected = min(m.searchSelected, len(m.searchMatches)-1)
	}
}

func fuzzySearchScore(query string, candidate searchCandidate) (int, bool) {
	if query == "" {
		return 0, true
	}
	queryRunes := []rune(query)
	text := []rune(candidate.relativeKey)
	score := 0
	position := 0
	streak := 0
	for _, needle := range queryRunes {
		found := -1
		for index, value := range text[position:] {
			if value == needle {
				found = position + index
				break
			}
		}
		if found < 0 {
			return 0, false
		}
		if found == position {
			streak++
			score += 10 + streak*4
		} else {
			streak = 0
			score -= found - position
		}
		if found == 0 || strings.ContainsRune("/-_ .", text[found-1]) {
			score += 12
		}
		position = found + 1
	}
	if candidate.nameKey == query {
		score += 100
	} else if strings.Contains(candidate.nameKey, query) {
		score += 40
	}
	return score, true
}

func (m Model) handleSearchKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "esc", "ctrl+c":
		return m.closeSearch(), nil
	case "enter":
		return m.activateSearchResult()
	case "down", "ctrl+n":
		if m.searchSelected < len(m.searchMatches)-1 {
			m.searchSelected++
		}
		return m, nil
	case "up", "ctrl+p":
		if m.searchSelected > 0 {
			m.searchSelected--
		}
		return m, nil
	case "backspace":
		runes := []rune(m.searchQuery)
		if len(runes) > 0 {
			m.searchQuery = string(runes[:len(runes)-1])
			m.searchSelected = 0
			m.refreshSearchMatches()
		}
		return m, nil
	}
	if message.Type == tea.KeyRunes {
		m.searchQuery += string(message.Runes)
		m.searchSelected = 0
		m.refreshSearchMatches()
	}
	return m, nil
}

func (m Model) activateSearchResult() (tea.Model, tea.Cmd) {
	if m.searchSelected < 0 || m.searchSelected >= len(m.searchMatches) {
		return m, nil
	}
	candidate := m.searchCandidates[m.searchMatches[m.searchSelected]]
	m = m.closeSearch()
	if candidate.directory {
		return m.navigateTo(candidate.path, "", true)
	}
	return m, openFile(candidate.path)
}

func (m Model) handleSearchMouse(message tea.MouseMsg) (tea.Model, tea.Cmd) {
	if message.Button != tea.MouseButtonLeft || message.Action != tea.MouseActionPress {
		return m, nil
	}
	width, height := m.searchDialogSize()
	left, top := (m.width-width)/2, (m.height-height)/2
	row := message.Y - top - 3
	if message.X <= left || message.X >= left+width-1 || row < 0 {
		return m, nil
	}
	start := searchWindowStart(m.searchSelected, len(m.searchMatches), height-5)
	index := start + row
	if index < 0 || index >= len(m.searchMatches) || row >= height-5 {
		return m, nil
	}
	m.searchSelected = index
	return m.activateSearchResult()
}

func (m Model) searchDialogSize() (int, int) {
	width := min(max(36, m.width*2/3), max(1, m.width-4))
	height := min(14, max(6, m.height-4))
	return width, height
}

func searchWindowStart(selected, count, visible int) int {
	if visible <= 0 || count <= visible {
		return 0
	}
	return min(max(0, selected-visible+1), count-visible)
}

func (m Model) renderSearch() string {
	width, height := m.searchDialogSize()
	innerWidth := max(1, width-4)
	t := m.styles.Theme
	background := lipgloss.Color(t.FullScreenBG)
	modal := lipgloss.NewStyle().Foreground(lipgloss.Color(t.ModalFG)).Background(lipgloss.Color(t.ModalBG))
	muted := modal.Foreground(lipgloss.Color(t.SidebarDivider))
	selected := modal.Foreground(lipgloss.Color(t.FilePanelItemSelectedFG)).Background(lipgloss.Color(t.FilePanelItemSelectedBG)).Bold(true)
	status := fmt.Sprintf("%d indexed", len(m.searchCandidates))
	if m.searchLoading {
		status = fmt.Sprintf("Scanning… %d", m.searchScanned)
	}
	titleText := truncate("Find in current tree", innerWidth)
	statusWidth := min(lipgloss.Width(status), max(0, innerWidth-lipgloss.Width(titleText)-2))
	status = truncate(status, statusWidth)
	headerGap := max(0, innerWidth-lipgloss.Width(titleText)-lipgloss.Width(status))
	header := modal.Bold(true).Foreground(lipgloss.Color(t.ModalBorderActive)).Render(titleText) +
		modal.Render(strings.Repeat(" ", headerGap)) + muted.Render(status)
	content := []string{
		modal.Width(innerWidth).Render(header),
	}
	if m.searchQuery == "" {
		content = append(content, modal.Width(innerWidth).Render("❯ "+muted.Render("type to fuzzy filter")+"█"))
	} else {
		content = append(content, modal.Width(innerWidth).Render("❯ "+truncate(m.searchQuery, max(1, innerWidth-3))+"█"))
	}
	visible := height - 5
	start := searchWindowStart(m.searchSelected, len(m.searchMatches), visible)
	if m.searchError != "" {
		content = append(content, muted.Width(innerWidth).Render("  "+truncate(m.searchError, innerWidth-2)))
	} else if len(m.searchMatches) == 0 {
		label := "  No matches"
		if m.searchLoading {
			label = "  Scanning…"
		}
		content = append(content, muted.Width(innerWidth).Render(label))
	} else {
		end := min(len(m.searchMatches), start+visible)
		for resultIndex := start; resultIndex < end; resultIndex++ {
			candidate := m.searchCandidates[m.searchMatches[resultIndex]]
			kind := core.File
			if candidate.directory {
				kind = core.Directory
			}
			icon := iconFor(core.Entry{Name: candidate.name, Path: candidate.path, Kind: kind})
			prefix := "  "
			style := modal
			if resultIndex == m.searchSelected {
				prefix = "› "
				style = selected
			}
			content = append(content, style.Width(innerWidth).Render(prefix+icon.Icon+" "+truncate(candidate.relative, max(1, innerWidth-5))))
		}
	}
	for len(content) < height-3 {
		content = append(content, modal.Width(innerWidth).Render(""))
	}
	content = append(content, muted.Width(innerWidth).Render("↑↓ select  enter open  esc close"))
	dialog := modal.Width(width-2).Height(height-2).Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.ModalBorderActive)).BorderBackground(lipgloss.Color(t.ModalBG)).
		Padding(0, 1).Render(strings.Join(content, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog, lipgloss.WithWhitespaceBackground(background))
}
