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

func TestClipboardClearIfDoesNotClearNewerSelection(t *testing.T) {
	clipboard := appfileops.Clipboard{}
	clipboard.Cut([]string{"/tmp/old"})
	clipboard.Copy([]string{"/tmp/new", "/tmp/other"})
	clipboard.ClearIf([]string{"/tmp/old"}, fsfileops.Move)
	if clipboard.Empty() || !clipboard.Contains("/tmp/new") || clipboard.Operation != fsfileops.Copy {
		t.Fatalf("clipboard was unexpectedly cleared: %+v", clipboard)
	}

	clipboard.ClearIf([]string{"/tmp/new", "/tmp/other"}, fsfileops.Copy)
	if !clipboard.Empty() {
		t.Fatalf("clipboard was not cleared: %+v", clipboard)
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
