package test

import (
	"testing"

	appfileops "github.com/urdadx/nukri/internal/app/fileops"
	fsfileops "github.com/urdadx/nukri/internal/fs/fileops"
)

func TestClipboardCopyAndCut(t *testing.T) {
	var clipboard appfileops.Clipboard
	clipboard.Copy("/tmp/copied")
	if clipboard.Source != "/tmp/copied" || clipboard.Operation != fsfileops.Copy {
		t.Fatalf("copy state = %+v", clipboard)
	}

	clipboard.Cut("/tmp/cut")
	if clipboard.Source != "/tmp/cut" || clipboard.Operation != fsfileops.Move {
		t.Fatalf("cut state = %+v", clipboard)
	}
}

func TestClipboardClearIfDoesNotClearNewerSelection(t *testing.T) {
	clipboard := appfileops.Clipboard{}
	clipboard.Cut("/tmp/old")
	clipboard.Copy("/tmp/new")
	clipboard.ClearIf("/tmp/old", fsfileops.Move)
	if clipboard.Empty() || clipboard.Source != "/tmp/new" || clipboard.Operation != fsfileops.Copy {
		t.Fatalf("clipboard was unexpectedly cleared: %+v", clipboard)
	}

	clipboard.ClearIf("/tmp/new", fsfileops.Copy)
	if !clipboard.Empty() {
		t.Fatalf("clipboard was not cleared: %+v", clipboard)
	}
}
