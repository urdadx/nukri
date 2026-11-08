package test

import (
	"context"
	"errors"
	"os"
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

// SVG and AVIF have no built-in decoder: without their tool the preview names
// the command to install instead of rendering.
func TestImageFormatsNameMissingTool(t *testing.T) {
	tests := []struct {
		sample string
		tool   string
	}{
		{"diamond.svg", "rsvg-convert"},
		{"gradient.avif", "imagemagick"},
	}
	for _, test := range tests {
		t.Run(test.sample, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("file_samples", test.sample))
			if err != nil {
				t.Fatal(err)
			}
			// A fresh path keeps the on-disk image cache from answering.
			path := filepath.Join(t.TempDir(), test.sample)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			facts := fileinfo.InspectPath(path, core.File)
			_, err = preview.NewServiceWithTools(preview.Tools{}).Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: 40})
			var toolErr *preview.ToolUnavailableError
			if !errors.As(err, &toolErr) || toolErr.Tool != test.tool {
				t.Fatalf("error = %v, want missing %s", err, test.tool)
			}
		})
	}
}
