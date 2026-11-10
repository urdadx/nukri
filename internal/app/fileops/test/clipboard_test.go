package test

import (
	"testing"

	appfileops "github.com/urdadx/nukri/internal/app/fileops"
	fsfileops "github.com/urdadx/nukri/internal/fs/fileops"
)

func TestClipboardCopyAndCut(t *testing.T) {
	var clipboard appfileops.Clipboard
	clipboard.Copy([]string{"/tmp/a", "/tmp/b"})
	if len(clipboard.Sources) != 2 || clipboard.Operation != fsfileops.Copy || !clipboard.Contains("/tmp/b") {
		t.Fatalf("copy state = %+v", clipboard)
	}

	clipboard.Cut([]string{"/tmp/cut"})
	if len(clipboard.Sources) != 1 || clipboard.Operation != fsfileops.Move || clipboard.Contains("/tmp/a") {
		t.Fatalf("cut state = %+v", clipboard)
	}
}

func TestClipboardKeepsItsOwnCopyOfSources(t *testing.T) {
	sources := []string{"/tmp/a"}
	var clipboard appfileops.Clipboard
	clipboard.Copy(sources)
	sources[0] = "/tmp/changed"
	if !clipboard.Contains("/tmp/a") {
		t.Fatal("clipboard shares its slice with the caller")
	}
}

// A paste that finishes after a newer copy or cut leaves the newer one alone.
func TestClipboardPastedDoesNotClearNewerSelection(t *testing.T) {
	clipboard := appfileops.Clipboard{}
	clipboard.Cut([]string{"/tmp/old"})
	clipboard.Cut([]string{"/tmp/new", "/tmp/other"})
	clipboard.Pasted([]string{"/tmp/old"}, fsfileops.Move, nil)
	if clipboard.Empty() || !clipboard.Contains("/tmp/new") || clipboard.Operation != fsfileops.Move {
		t.Fatalf("clipboard was unexpectedly cleared: %+v", clipboard)
	}

	clipboard.Pasted([]string{"/tmp/new", "/tmp/other"}, fsfileops.Move, nil)
	if !clipboard.Empty() {
		t.Fatalf("clipboard was not cleared after the move: %+v", clipboard)
	}
}

// Copied items stay on the clipboard after pasting, to paste them elsewhere too.
func TestClipboardKeepsCopiedItemsAfterPaste(t *testing.T) {
	clipboard := appfileops.Clipboard{}
	sources := []string{"/tmp/a", "/tmp/b"}
	clipboard.Copy(sources)
	clipboard.Pasted(sources, fsfileops.Copy, nil)
	if len(clipboard.Sources) != 2 || clipboard.Operation != fsfileops.Copy {
		t.Fatalf("clipboard = %+v, want both items still copied", clipboard)
	}
}

// Items that failed to paste stay on the clipboard so they can be retried.
func TestClipboardKeepsFailedItemsAfterPaste(t *testing.T) {
	clipboard := appfileops.Clipboard{}
	sources := []string{"/tmp/a", "/tmp/b", "/tmp/c"}
	clipboard.Cut(sources)
	clipboard.Pasted(sources, fsfileops.Move, []string{"/tmp/b"})
	if len(clipboard.Sources) != 1 || !clipboard.Contains("/tmp/b") || clipboard.Operation != fsfileops.Move {
		t.Fatalf("clipboard = %+v, want only the failed item, still cut", clipboard)
	}
}

// Only one paste runs at a time, even if the clipboard is cleared meanwhile.
func TestClipboardRunsOnePasteAtATime(t *testing.T) {
	clipboard := appfileops.Clipboard{}
	clipboard.Copy([]string{"/tmp/a"})
	if !clipboard.BeginPaste() {
		t.Fatal("the first paste was refused")
	}
	clipboard.Clear()
	clipboard.Copy([]string{"/tmp/b"})
	if clipboard.BeginPaste() {
		t.Fatal("a second paste started while the first was running")
	}
	clipboard.Pasted([]string{"/tmp/a"}, fsfileops.Copy, nil)
	if !clipboard.BeginPaste() {
		t.Fatal("a paste was refused after the first one finished")
	}
}

func TestClipboardClear(t *testing.T) {
	clipboard := appfileops.Clipboard{}
	clipboard.Copy([]string{"/tmp/file"})
	clipboard.Clear()
	if !clipboard.Empty() {
		t.Fatalf("clipboard was not cleared: %+v", clipboard)
	}
}
