package test

import (
	"testing"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/ui"
	"github.com/urdadx/nukri/internal/ui/browser"
)

func prefetchFixture(names []struct{ name, kind string }) []browser.Entry {
	entries := make([]browser.Entry, 0, len(names))
	for _, n := range names {
		kind := core.File
		if n.kind == "dir" {
			kind = core.Directory
		}
		entries = append(entries, browser.Entry{Entry: core.Entry{Name: n.name, Kind: kind, Path: "/" + n.name}})
	}
	return entries
}

// entryNames extracts just names/paths so assertions read cleanly.
func prefetchNames(entries []browser.Entry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Entry.Name
	}
	return names
}

func TestPrefetchNeighborsAroundSelection(t *testing.T) {
	entries := prefetchFixture([]struct{ name, kind string }{
		{"a", "file"}, {"b", "file"}, {"c", "file"}, {"d", "file"}, {"e", "file"}, {"f", "file"},
	})

	got := prefetchNames(ui.PrefetchNeighbors(entries, 2, 2))
	want := []string{"b", "d", "a", "e"}
	if len(got) != len(want) {
		t.Fatalf("PrefetchNeighbors = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("PrefetchNeighbors = %v, want %v", got, want)
		}
	}
}

func TestPrefetchNeighborsClipsAtEdges(t *testing.T) {
	entries := prefetchFixture([]struct{ name, kind string }{
		{"a", "file"}, {"b", "file"}, {"c", "file"},
	})

	// Selected index 0: only the neighbors to the right.
	head := ui.PrefetchNeighbors(entries, 0, 2)
	if got, want := prefetchNames(head), []string{"b", "c"}; !equalStrings(got, want) {
		t.Fatalf("head = %v, want %v", got, want)
	}

	// Selected index 2 (last): only the neighbors to the left.
	tail := ui.PrefetchNeighbors(entries, 2, 2)
	if got, want := prefetchNames(tail), []string{"b", "a"}; !equalStrings(got, want) {
		t.Fatalf("tail = %v, want %v", got, want)
	}
}

func TestPrefetchNeighborsExcludesDirectories(t *testing.T) {
	entries := prefetchFixture([]struct{ name, kind string }{
		{"prev", "file"}, {"prevdir", "dir"}, {"sel", "file"}, {"nextdir", "dir"}, {"next", "file"},
	})

	got := prefetchNames(ui.PrefetchNeighbors(entries, 2, 2))
	want := []string{"prev", "next"}
	if !equalStrings(got, want) {
		t.Fatalf("PrefetchNeighbors = %v, want %v", got, want)
	}
}

func TestPrefetchNeighborsEmptyResult(t *testing.T) {
	entries := prefetchFixture([]struct{ name, kind string }{
		{"only", "file"},
	})
	if got := ui.PrefetchNeighbors(entries, 0, 2); len(got) != 0 {
		t.Fatalf("expected no neighbors for a single file, got %v", prefetchNames(got))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
