package test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/file_info"
	"github.com/urdadx/nukri/internal/preview"
)

// TestToolUnavailableNamesTool verifies that a missing external tool surfaces
// which binary needs to be installed, while still matching ErrToolUnavailable.
func TestToolUnavailableNamesTool(t *testing.T) {
	service := preview.NewServiceWithTools(preview.Tools{})
	facts := file_infoForDocument(fileinfo.Pdf)

	_, err := service.Render(context.Background(), preview.Request{Path: "book.pdf", Facts: facts})
	if !errors.Is(err, preview.ErrToolUnavailable) {
		t.Fatalf("error = %v, want ErrToolUnavailable", err)
	}

	var toolErr *preview.ToolUnavailableError
	if !errors.As(err, &toolErr) {
		t.Fatalf("error = %v, want *ToolUnavailableError", err)
	}
	if toolErr.Tool == "" {
		t.Fatalf("tool name is empty in %q", err)
	}
}

// TestNativeZipListingWithoutTool verifies ZIP-based archives (e.g. .jar)
// preview without any external archive tool.
func TestNativeZipListingWithoutTool(t *testing.T) {
	service := preview.NewServiceWithTools(preview.Tools{})
	path := filepath.Join("file_samples", "roobert-font-family.zip")
	facts := fileinfo.InspectPath(path, core.File)

	result, err := service.Render(context.Background(), preview.Request{Path: path, Facts: facts})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	archive, ok := result.(*preview.ArchivePreview)
	if !ok {
		t.Fatalf("result = %#v, want archive listing", result)
	}
	if len(archive.Archive.Entries) == 0 {
		t.Fatal("native zip listing returned no entries")
	}
	for _, entry := range archive.Archive.Entries {
		if entry.Path == "" {
			t.Fatalf("archive contains an entry without a path: %#v", entry)
		}
	}
}
