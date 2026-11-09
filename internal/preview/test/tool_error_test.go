package test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// fakePDFToPPM returns a pdftoppm stand-in that prints output to stdout.
func fakePDFToPPM(t *testing.T, output []byte) string {
	t.Helper()
	directory := t.TempDir()
	data := filepath.Join(directory, "page.ppm")
	if err := os.WriteFile(data, output, 0o600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(directory, "pdftoppm")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ncat '"+data+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return script
}

// PDF pages arrive from pdftoppm as raw PPM pixels; well-formed output becomes
// a PNG and malformed output is an error, never a crash.
func TestPDFPageFromPPMOutput(t *testing.T) {
	tools := preview.DiscoverTools()
	if tools.PDFInfo == "" {
		t.Skip("requires pdfinfo")
	}
	pixels := append([]byte("P6\n# comment\n2 1\n255\n"), 255, 0, 0, 0, 0, 255)
	for _, test := range []struct {
		name   string
		output []byte
		ok     bool
	}{
		{"valid", pixels, true},
		{"truncated pixels", pixels[:len(pixels)-2], false},
		{"not ppm", []byte("%PDF-1.7"), false},
		{"huge dimensions", []byte("P6 99999 99999 255\n"), false},
		{"16-bit", []byte("P6 1 1 65535\n\x00\x00\x00\x00\x00\x00"), false},
		{"empty", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			tools.PDFToPPM = fakePDFToPPM(t, test.output)
			path := filepath.Join("file_samples", "session_history.pdf")
			result, err := preview.NewServiceWithTools(tools).Render(context.Background(), preview.Request{
				Path: copyToTemp(t, path), Facts: file_infoForDocument(fileinfo.Pdf), Width: 40,
			})
			if !test.ok {
				if err == nil {
					t.Fatal("malformed pdftoppm output should fail")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			page := result.(*preview.PDFPreview).Page
			if page.Width != 2 || page.Height != 1 || page.MediaType != "image/png" {
				t.Fatalf("page = %dx%d %s", page.Width, page.Height, page.MediaType)
			}
		})
	}
}

// copyToTemp copies path to a fresh location so the preview cache can't answer.
func copyToTemp(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), filepath.Base(path))
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return target
}

// ZIP-based archives are read with the standard library even when 7-Zip is
// available, since starting 7-Zip costs far more than reading the directory.
func TestZipListingPrefersStandardLibrary(t *testing.T) {
	failing := filepath.Join(t.TempDir(), "7zz")
	if err := os.WriteFile(failing, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("file_samples", "roobert-font-family.zip")
	service := preview.NewServiceWithTools(preview.Tools{SevenZip: failing})
	result, err := service.Render(context.Background(), preview.Request{Path: path, Facts: fileinfo.InspectPath(path, core.File)})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	entries := result.(*preview.ArchivePreview).Archive.Entries
	if len(entries) != 38 {
		t.Fatalf("listed %d entries, want 38", len(entries))
	}
	for _, entry := range entries {
		if entry.Attributes == "" || entry.Modified == "" {
			t.Fatalf("entry lacks attributes or time: %#v", entry)
		}
	}
}

// 7-Zip's listing is parsed as it streams. A listing longer than the preview
// keeps is cut off at the entry limit and 7-Zip is stopped, so even endless
// output returns promptly; a failing 7-Zip still reports an error.
func TestSevenZipListingStopsAtEntryLimit(t *testing.T) {
	directory := t.TempDir()
	endless := filepath.Join(directory, "endless-7zz")
	script := "#!/bin/sh\ni=0\nwhile true; do i=$((i+1)); printf 'Path = dir/file%d.bin\\nSize = 10\\nPacked Size = 4\\nModified = 2026-01-02 03:04:05\\nAttributes = A\\n\\n' $i; done\n"
	if err := os.WriteFile(endless, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	failing := filepath.Join(directory, "failing-7zz")
	if err := os.WriteFile(failing, []byte("#!/bin/sh\necho 'Can not open the file as archive' >&2\nexit 2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "archive.7z")
	if err := os.WriteFile(path, []byte("7z\xbc\xaf\x27\x1c"), 0o600); err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectPath(path, core.File)

	start := time.Now()
	result, err := preview.NewServiceWithTools(preview.Tools{SevenZip: endless}).Render(context.Background(), preview.Request{Path: path, Facts: facts})
	if err != nil {
		t.Fatal(err)
	}
	archive := result.(*preview.ArchivePreview).Archive
	if len(archive.Entries) != preview.DefaultMaxArchiveEntries || !archive.Truncated {
		t.Fatalf("listed %d entries (truncated %v), want %d, truncated", len(archive.Entries), archive.Truncated, preview.DefaultMaxArchiveEntries)
	}
	if first := archive.Entries[0]; first.Path != "dir/file1.bin" || first.Size != 10 || first.PackedSize != 4 || first.Modified != "2026-01-02 03:04:05" {
		t.Fatalf("first entry = %#v", first)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("listing took %v; 7-Zip was not stopped", elapsed)
	}

	if _, err := preview.NewServiceWithTools(preview.Tools{SevenZip: failing}).Render(context.Background(), preview.Request{Path: path, Facts: facts}); err == nil {
		t.Fatal("a failing 7-Zip should produce an error")
	}
}

// When rsvg-convert is missing, SVGs render through ImageMagick. If ImageMagick
// fails on a file, the preview asks for rsvg-convert, which renders full SVG.
func TestSVGFallsBackToImageMagick(t *testing.T) {
	imageMagick := preview.DiscoverTools().ImageMagick
	if imageMagick == "" {
		t.Skip("requires ImageMagick")
	}
	service := preview.NewServiceWithTools(preview.Tools{ImageMagick: imageMagick})
	good := copyToTemp(t, filepath.Join("file_samples", "diamond.svg"))
	result, err := service.Render(context.Background(), preview.Request{Path: good, Facts: fileinfo.InspectPath(good, core.File), Width: 30})
	if err != nil {
		t.Fatalf("render through ImageMagick: %v", err)
	}
	if image := result.(*preview.SVGPreview).Image; image.Width != 24 || image.Height != 30 {
		t.Fatalf("image = %dx%d, want 24x30", image.Width, image.Height)
	}

	broken := filepath.Join(t.TempDir(), "broken.svg")
	if err := os.WriteFile(broken, []byte("<svg xmlns='http://www.w3.org/2000/svg'><unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = service.Render(context.Background(), preview.Request{Path: broken, Facts: fileinfo.InspectPath(broken, core.File), Width: 30})
	var toolErr *preview.ToolUnavailableError
	if !errors.As(err, &toolErr) || toolErr.Tool != "rsvg-convert" {
		t.Fatalf("error = %v, want a suggestion to install rsvg-convert", err)
	}
}
