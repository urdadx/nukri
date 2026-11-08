package ui

import (
	"container/heap"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

// searchState is the fuzzy-finder overlay.
type searchState struct {
	open       bool
	query      string
	selected   int
	token      uint64 // identifies the current scan; stale results are dropped
	candidates []searchCandidate
	matches    []int // indexes into candidates, best first
	filterPool []int // candidates matching filterKey, reused while the query grows
	filterKey  string
	loading    bool
	scanned    int
	errMessage string
}

func (m Model) openSearch() (tea.Model, tea.Cmd) {
	if m.data.CWD == "" {
		return m, nil
	}
	// The token keeps counting so results from an earlier search are ignored.
	m.search = searchState{open: true, loading: true, token: m.search.token + 1}
	stream := m.searchPool.Submit(searchRequest{token: m.search.token, root: m.data.CWD})
	return m, waitSearch(stream)
}

func (m Model) closeSearch() Model {
	m.search.open = false
	m.search.token++
	m.search.loading = false
	m.searchPool.Cancel()
	return m
}

func (m Model) handleSearchEvent(message searchMsg) (tea.Model, tea.Cmd) {
	if !m.search.open || message.event.token != m.search.token {
		return m, nil
	}
	firstNew := len(m.search.candidates)
	m.search.candidates = append(m.search.candidates, message.event.candidates...)
	m.search.scanned = message.event.scanned
	m.search.loading = !message.event.done
	if message.event.err != nil {
		m.search.errMessage = message.event.err.Error()
	}
	m.extendSearchMatches(firstNew)
	if message.event.done {
		return m, nil
	}
	return m, waitSearch(message.stream)
}

func (m *Model) refreshSearchMatches() {
	query := strings.ToLower(strings.TrimSpace(m.search.query))
	pool := m.search.filterPool
	if m.search.filterKey == "" || !strings.HasPrefix(query, m.search.filterKey) {
		pool = nil
	}
	m.search.matches, m.search.filterPool = rankSearchCandidates(m.search.candidates, pool, query, query != "")
	m.search.filterKey = query
	m.clampSearchSelection()
}

func (m *Model) extendSearchMatches(firstNew int) {
	query := strings.ToLower(strings.TrimSpace(m.search.query))
	newPool := make([]int, 0, len(m.search.candidates)-firstNew)
	for index := firstNew; index < len(m.search.candidates); index++ {
		newPool = append(newPool, index)
	}
	newMatches, filtered := rankSearchCandidates(m.search.candidates, newPool, query, query != "")
	if query != "" {
		m.search.filterPool = append(m.search.filterPool, filtered...)
	}
	candidates := append(append([]int(nil), m.search.matches...), newMatches...)
	m.search.matches, _ = rankSearchCandidates(m.search.candidates, candidates, query, false)
	m.search.filterKey = query
	m.clampSearchSelection()
}

func (m *Model) clampSearchSelection() {
	if len(m.search.matches) == 0 {
		m.search.selected = 0
	} else {
		m.search.selected = min(m.search.selected, len(m.search.matches)-1)
	}
}

type rankedSearchCandidate struct {
	index, score int
}

type searchCandidateHeap struct {
	candidates []searchCandidate
	values     []rankedSearchCandidate
}

func (h searchCandidateHeap) Len() int { return len(h.values) }
func (h searchCandidateHeap) Less(i, j int) bool {
	return betterSearchCandidate(h.candidates, h.values[j], h.values[i])
}
func (h searchCandidateHeap) Swap(i, j int) { h.values[i], h.values[j] = h.values[j], h.values[i] }
func (h *searchCandidateHeap) Push(value any) {
	h.values = append(h.values, value.(rankedSearchCandidate))
}
func (h *searchCandidateHeap) Pop() any {
	last := len(h.values) - 1
	value := h.values[last]
	h.values = h.values[:last]
	return value
}

func rankSearchCandidates(candidates []searchCandidate, pool []int, query string, collectFiltered bool) ([]int, []int) {
	var filtered []int
	if collectFiltered {
		filtered = make([]int, 0, len(pool))
	}
	top := &searchCandidateHeap{candidates: candidates, values: make([]rankedSearchCandidate, 0, searchMatchLimit)}
	visit := func(index int) {
		candidate := candidates[index]
		score, ok := fuzzySearchScore(query, candidate)
		if !ok {
			return
		}
		if collectFiltered {
			filtered = append(filtered, index)
		}
		value := rankedSearchCandidate{index: index, score: score}
		if top.Len() < searchMatchLimit {
			heap.Push(top, value)
		} else if betterSearchCandidate(candidates, value, top.values[0]) {
			heap.Pop(top)
			heap.Push(top, value)
		}
	}
	if pool == nil {
		if collectFiltered {
			filtered = make([]int, 0, len(candidates))
		}
		for index := range candidates {
			visit(index)
		}
	} else {
		for _, index := range pool {
			visit(index)
		}
	}
	sort.Slice(top.values, func(i, j int) bool {
		return betterSearchCandidate(candidates, top.values[i], top.values[j])
	})
	matches := make([]int, len(top.values))
	for index, value := range top.values {
		matches[index] = value.index
	}
	return matches, filtered
}

func betterSearchCandidate(candidates []searchCandidate, left, right rankedSearchCandidate) bool {
	if left.score != right.score {
		return left.score > right.score
	}
	leftPath := candidates[left.index].relative
	rightPath := candidates[right.index].relative
	if len(leftPath) != len(rightPath) {
		return len(leftPath) < len(rightPath)
	}
	return leftPath < rightPath
}

func fuzzySearchScore(query string, candidate searchCandidate) (int, bool) {
	if query == "" {
		return 0, true
	}
	score := 0
	position := 0
	streak := 0
	for queryIndex := 0; queryIndex < len(query); queryIndex++ {
		needle := query[queryIndex]
		found := -1
		for index := position; index < len(candidate.relativeKey); index++ {
			if candidate.relativeKey[index] == needle {
				found = index
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
		if found == 0 || strings.ContainsRune("/-_ .", rune(candidate.relativeKey[found-1])) {
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
	case "ctrl+c":
		return m.closeSearch(), nil
	case "enter":
		return m.activateSearchResult()
	case "down", "ctrl+n":
		if m.search.selected < len(m.search.matches)-1 {
			m.search.selected++
		}
		return m, nil
	case "up", "ctrl+p":
		if m.search.selected > 0 {
			m.search.selected--
		}
		return m, nil
	case "backspace":
		runes := []rune(m.search.query)
		if len(runes) > 0 {
			m.search.query = string(runes[:len(runes)-1])
			m.search.selected = 0
			m.refreshSearchMatches()
		}
		return m, nil
	}
	if message.Type == tea.KeyRunes {
		m.search.query += string(message.Runes)
		m.search.selected = 0
		m.refreshSearchMatches()
	}
	return m, nil
}

func (m Model) activateSearchResult() (tea.Model, tea.Cmd) {
	if m.search.selected < 0 || m.search.selected >= len(m.search.matches) {
		return m, nil
	}
	candidate := m.search.candidates[m.search.matches[m.search.selected]]
	m = m.closeSearch()
	if candidate.directory {
		return m.navigateTo(candidate.path, "", true)
	}
	parent := filepath.Dir(candidate.path)
	if parent != m.data.CWD {
		return m.navigateTo(parent, candidate.path, true)
	}
	index := entryIndexByPath(m.data.Entries, candidate.path)
	return m.moveSelection(index - m.data.Selected)
}

func (m Model) handleSearchMouse(message tea.MouseMsg) (tea.Model, tea.Cmd) {
	if message.Button != tea.MouseButtonLeft || message.Action != tea.MouseActionPress {
		return m, nil
	}
	width, height := m.searchDialogSize()
	left, top := m.searchDialogPosition(width, height)
	row := message.Y - top - 3
	if message.X <= left || message.X >= left+width-1 || row < 0 {
		return m, nil
	}
	start := searchWindowStart(m.search.selected, len(m.search.matches), height-5)
	index := start + row
	if index < 0 || index >= len(m.search.matches) || row >= height-5 {
		return m, nil
	}
	m.search.selected = index
	return m.activateSearchResult()
}

func (m Model) searchDialogSize() (int, int) {
	width := min(max(40, m.width*3/4), max(1, m.width-4))
	height := min(max(10, m.height*2/3), max(6, m.height-5))
	return width, height
}

func (m Model) searchDialogPosition(width, height int) (int, int) {
	return max(0, (m.width-width)/2), max(0, (m.height-height)/2)
}

func searchWindowStart(selected, count, visible int) int {
	if visible <= 0 || count <= visible {
		return 0
	}
	return min(max(0, selected-visible+1), count-visible)
}

func (m Model) renderSearch(base string) string {
	width, height := m.searchDialogSize()
	innerWidth := max(1, width-4)
	t := m.styles.Theme
	modal := lipgloss.NewStyle().Foreground(lipgloss.Color(t.ModalFG)).Background(lipgloss.Color(t.ModalBG))
	muted := modal.Foreground(lipgloss.Color(t.SidebarDivider))
	selected := modal.Foreground(lipgloss.Color(t.FilePanelItemSelectedFG)).Background(lipgloss.Color(t.FilePanelItemSelectedBG)).Bold(true)
	status := fmt.Sprintf("%d indexed", len(m.search.candidates))
	if m.search.loading {
		status = fmt.Sprintf("Scanning… %d", m.search.scanned)
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
	if m.search.query == "" {
		content = append(content, modal.Width(innerWidth).Render("❯ "+muted.Render("type to fuzzy filter")+"█"))
	} else {
		content = append(content, modal.Width(innerWidth).Render("❯ "+truncate(m.search.query, max(1, innerWidth-3))+"█"))
	}
	visible := height - 5
	start := searchWindowStart(m.search.selected, len(m.search.matches), visible)
	if m.search.errMessage != "" {
		content = append(content, muted.Width(innerWidth).Render("  "+truncate(m.search.errMessage, innerWidth-2)))
	} else if len(m.search.matches) == 0 {
		label := "  No matches"
		if m.search.loading {
			label = "  Scanning…"
		}
		content = append(content, muted.Width(innerWidth).Render(label))
	} else {
		end := min(len(m.search.matches), start+visible)
		for resultIndex := start; resultIndex < end; resultIndex++ {
			candidate := m.search.candidates[m.search.matches[resultIndex]]
			kind := core.File
			if candidate.directory {
				kind = core.Directory
			}
			icon := iconFor(core.Entry{Name: candidate.name, Path: candidate.path, Kind: kind})
			prefix := "  "
			style := modal
			if resultIndex == m.search.selected {
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
	left, top := m.searchDialogPosition(lipgloss.Width(dialog), lipgloss.Height(dialog))
	return overlaySearch(base, dialog, left, top)
}

func overlaySearch(base, overlay string, left, top int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")
	for row, overlayLine := range overlayLines {
		index := top + row
		if index < 0 || index >= len(baseLines) {
			continue
		}
		right := left + lipgloss.Width(overlayLine)
		baseLines[index] = ansi.Cut(baseLines[index], 0, left) + overlayLine + ansi.Cut(baseLines[index], right, lipgloss.Width(baseLines[index]))
	}
	return strings.Join(baseLines, "\n")
}
