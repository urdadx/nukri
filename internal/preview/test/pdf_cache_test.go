package test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/file_info"
	"github.com/urdadx/nukri/internal/preview"
)

// TestPDFPreviewIgnoresSourceByteSize guards against the (incorrect) rejection
// of PDFs whose source file exceeds the image output byte limit. A PDF's on-disk
// size has no bearing on the size of its downscaled first-page preview, so a
// large document must still render. Padding a valid PDF with trailing bytes
// keeps it parsable by poppler while pushing it well past the limit.
func TestPDFPreviewIgnoresSourceByteSize(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().PDF, "pdfinfo and pdftocairo")

	path := copyPDFSample(t)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(make([]byte, 17<<20)); err != nil { // 17 MiB > DefaultMaxImageBytes
		file.Close()
		t.Fatal(err)
	}
	file.Close()

	pdf := renderPDFPreview(t, service, path, 60)
	if len(pdf.Page.Data) == 0 {
		t.Fatal("large PDF rendered an empty preview page")
	}
}


// copyPDFSample copies the bundled session_history.pdf fixture into a fresh
// temp path so tests can mutate it without touching the repo fixture.
func copyPDFSample(t *testing.T) string {
	t.Helper()
	src := filepath.Join("file_samples", "session_history.pdf")
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	dst := filepath.Join(t.TempDir(), "sample.pdf")
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return dst
}

func renderPDFPreview(t *testing.T, service *preview.Service, path string, width int) *preview.PDFPreview {
	t.Helper()
	facts := fileinfo.InspectPath(path, core.File)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := service.Render(ctx, preview.Request{Path: path, Facts: facts, Width: width})
	if err != nil {
		t.Fatalf("render PDF at width %d: %v", width, err)
	}
	pdf, ok := result.(*preview.PDFPreview)
	if !ok {
		t.Fatalf("expected *preview.PDFPreview, got %T", result)
	}
	return pdf
}

func TestPDFPreviewServesFromDiskCache(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().PDF, "pdfinfo and pdftocairo")
	path := copyPDFSample(t)
	src, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	size, mod := src.Size(), src.ModTime()

	first := renderPDFPreview(t, service, path, 60)
	if len(first.Page.Data) == 0 {
		t.Fatal("first render produced an empty page")
	}

	// Overwrite the source with garbage while preserving its size and
	// modification time, so the cache key is unchanged. If the second render
	// succeeds with identical page bytes and metadata, it must have been served
	// entirely from the on-disk cache — no tool was invoked to re-rasterize or
	// re-probe the (now undecodable) source.
	garbage := make([]byte, size)
	for i := range garbage {
		garbage[i] = 0xCD
	}
	if err := os.WriteFile(path, garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}

	second := renderPDFPreview(t, service, path, 60)
	if string(second.Page.Data) != string(first.Page.Data) {
		t.Fatal("cached PDF page bytes differ from the first render")
	}
	if second.Page.Width != first.Page.Width || second.Page.Height != first.Page.Height {
		t.Fatalf("cached page dimensions differ: first=%dx%d second=%dx%d",
			first.Page.Width, first.Page.Height, second.Page.Width, second.Page.Height)
	}
	if len(second.Metadata) != len(first.Metadata) {
		t.Fatalf("cached metadata length differs: first=%d second=%d", len(first.Metadata), len(second.Metadata))
	}
}

// TestPDFPreviewRendersToDisplaySize verifies the page is rasterized at a
// supersampled multiple of the display target size rather than always at the
// maximum, and that a narrower pane yields a smaller page.
func TestPDFPreviewRendersToDisplaySize(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().PDF, "pdfinfo and pdftocairo")
	path := copyPDFSample(t)

	small := renderPDFPreview(t, service, path, 60)
	longestSmall, _ := orderedDims(small.Page.Width, small.Page.Height)
	if longestSmall <= 480 {
		t.Fatalf("expected page supersampled above display target 480, got %d", longestSmall)
	}
	if longestSmall > 960 {
		t.Fatalf("expected page longest side at supersampled target 960, got %d", longestSmall)
	}

	wide := renderPDFPreview(t, service, path, 0)
	longestWide, _ := orderedDims(wide.Page.Width, wide.Page.Height)
	if longestWide > 1600 {
		t.Fatalf("expected page longest side capped at maxImageDimension 1600, got %d", longestWide)
	}
	if longestWide <= longestSmall {
		t.Fatalf("expected wider target to yield a larger page; small=%d wide=%d", longestSmall, longestWide)
	}
}

// orderedDims returns the two dimensions with the larger first.
func orderedDims(a, b int) (int, int) {
	if a >= b {
		return a, b
	}
	return b, a
}
