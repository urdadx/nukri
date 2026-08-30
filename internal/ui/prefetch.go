package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/ui/browser"
)

const (
	// prefetchDebounce is how long the selection must sit still before the
	// neighbors are prefetched. Fast scrolling reschedules the timer, so a burst
	// of selections only ever prefetches around the final selection.
	prefetchDebounce = 300 * time.Millisecond
	// prefetchRadius is how many files on each side of the selection to prefetch.
	prefetchRadius = 2
)

// prefetchMsg delivers the neighbor files to prefetch once the selection has
// gone idle. Its sequence number lets the model drop requests that were
// superseded by a newer selection.
type prefetchMsg struct {
	seq     int
	width   int
	syntax  string
	entries []browser.Entry
}

// PrefetchNeighbors returns the neighboring *files* within radius of the
// selected index, in the order they appear nearest-first. Only files are
// prefetched — directory previews are cheap to render on demand. It is exported
// so logic tests can exercise the neighbor-selection rule.
func PrefetchNeighbors(entries []browser.Entry, selected, radius int) []browser.Entry {
	neighbors := make([]browser.Entry, 0, min(radius*2, len(entries)))
	for i := 1; i <= radius; i++ {
		if idx := selected - i; idx >= 0 && entries[idx].Entry.Kind == core.File {
			neighbors = append(neighbors, entries[idx])
		}
		if idx := selected + i; idx < len(entries) && entries[idx].Entry.Kind == core.File {
			neighbors = append(neighbors, entries[idx])
		}
	}
	return neighbors
}

// schedulePrefetch bumps the model's prefetch sequence and returns a debounced
// command that, once the selection has been idle for prefetchDebounce, submits
// the neighboring *files* to the pool at low priority. The neighbors must not
// already be cached (Submit short-circuits those). The returned model carries
// the bumped sequence so the Update handler can discard stale ticks.
func schedulePrefetch(m Model, entries []browser.Entry, selected, width int, syntax string) (Model, tea.Cmd) {
	m.prefetchSeq++
	seq := m.prefetchSeq

	neighbors := PrefetchNeighbors(entries, selected, prefetchRadius)
	if len(neighbors) == 0 {
		return m, nil
	}

	cmd := tea.Tick(prefetchDebounce, func(time.Time) tea.Msg {
		return prefetchMsg{seq: seq, width: width, syntax: syntax, entries: neighbors}
	})
	return m, cmd
}

// submitPrefetch enqueues the prefetched neighbors at low priority. It never
// cancels the high-priority selection render. Work that is already cached or
// already queued is a no-op inside Submit. Prefetch warms the full highlight
// (codeWindow 0) so that a later selection renders from the disk cache instantly.
func (m *Model) submitPrefetch(msg prefetchMsg) {
	if msg.seq != m.prefetchSeq {
		return
	}
	for _, entry := range msg.entries {
		m.previewPool.Submit(entry, msg.width, msg.syntax, 0, false)
	}
}
