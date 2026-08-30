package test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/core"
	fileinfo "github.com/urdadx/nukri/internal/file_info"
	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/ui"
	"github.com/urdadx/nukri/internal/ui/browser"
)

func poolTestEntry(t *testing.T, content string) browser.Entry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entry := core.Entry{
		Name:     "sample.txt",
		Path:     path,
		Kind:     core.File,
		Size:     info.Size(),
		Modified: info.ModTime(),
	}
	facts := fileinfo.InspectPath(path, core.File)
	return browser.Entry{Entry: entry, Facts: facts}
}

// expectMessage asserts that exactly one message is delivered and the channel
// is then closed.
func expectMessage(t *testing.T, ch <-chan tea.Msg) {
	t.Helper()
	select {
	case msg, ok := <-ch:
		if !ok {
			t.Fatal("expected one message before channel close")
		}
		if msg == nil {
			t.Fatal("expected a non-nil preview message")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for preview render")
	}
	if _, again := <-ch; again {
		t.Fatal("expected channel to be closed after single message")
	}
}

func TestPreviewPoolRendersSingleMessageAndCloses(t *testing.T) {
	pool := ui.NewPreviewPool(preview.NewService(), 2)
	defer pool.Close()

	entry := poolTestEntry(t, "hello world\nsecond line\n")
	expectMessage(t, pool.Submit(entry, 60, "", 0, true))
}

func TestPreviewPoolCacheHitReturnsImmediately(t *testing.T) {
	pool := ui.NewPreviewPool(preview.NewService(), 2)
	defer pool.Close()

	entry := poolTestEntry(t, "cached content\n")
	expectMessage(t, pool.Submit(entry, 60, "", 0, true))

	start := time.Now()
	expectMessage(t, pool.Submit(entry, 60, "", 0, true))
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("cache hit should be fast, took %v", elapsed)
	}
}

func TestPreviewPoolDifferentWidthIsDifferentCacheEntry(t *testing.T) {
	pool := ui.NewPreviewPool(preview.NewService(), 2)
	defer pool.Close()

	entry := poolTestEntry(t, "width sensitive\n")
	if ui.PreviewKey(entry, 60, "", 0) == ui.PreviewKey(entry, 80, "", 0) {
		t.Fatal("different widths must produce different cache keys")
	}
	expectMessage(t, pool.Submit(entry, 60, "", 0, true))
	expectMessage(t, pool.Submit(entry, 80, "", 0, true))
}

func TestPreviewPoolDedupesConcurrentIdenticalSubmits(t *testing.T) {
	pool := ui.NewPreviewPool(preview.NewService(), 2)
	defer pool.Close()

	entry := poolTestEntry(t, "dedup me\n")
	const n = 8
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := pool.Submit(entry, 60, "", 0, true)
			select {
			case msg, ok := <-ch:
				if !ok {
					t.Error("expected a message from a deduped submit")
					return
				}
				if msg == nil {
					t.Error("expected non-nil message")
				}
			case <-time.After(5 * time.Second):
				t.Error("timed out waiting for deduped submit result")
			}
		}()
	}
	wg.Wait()
}

func TestPreviewKeyDistinction(t *testing.T) {
	entry := func(name, path, mod string) browser.Entry {
		tm := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		if mod == "later" {
			tm = tm.Add(time.Hour)
		}
		return browser.Entry{Entry: core.Entry{Name: name, Path: path, Kind: core.File, Size: 10, Modified: tm}}
	}
	a := entry("a.txt", "/tmp/a.txt", "now")
	b := entry("b.txt", "/tmp/b.txt", "now")
	c := entry("a.txt", "/tmp/a.txt", "later")

	if ui.PreviewKey(a, 60, "", 0) == ui.PreviewKey(b, 60, "", 0) {
		t.Fatal("different paths should produce different keys")
	}
	if ui.PreviewKey(a, 60, "", 0) == ui.PreviewKey(c, 60, "", 0) {
		t.Fatal("different modification times should produce different keys")
	}
	if ui.PreviewKey(a, 60, "", 0) == ui.PreviewKey(a, 60, "monokai", 0) {
		t.Fatal("different syntax styles should produce different keys")
	}
	if ui.PreviewKey(a, 60, "", 0) != ui.PreviewKey(a, 60, "", 0) {
		t.Fatal("identical inputs must produce identical keys")
	}
	if ui.PreviewKey(a, 60, "", 0) == ui.PreviewKey(a, 60, "", 256) {
		t.Fatal("different code windows should produce different keys (incremental extension)")
	}
}
