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

// Office previews are cached by the source document, so revisiting one skips
// the LibreOffice conversion entirely.
func TestOfficePreviewServesFromDiskCache(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().Documents, "libreoffice, pdfinfo, and pdftocairo")
	data, err := os.ReadFile(filepath.Join("file_samples", "Third Party.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sheet.xlsx")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectPath(path, core.File)
	render := func() *preview.OfficePreview {
		t.Helper()
		result, err := service.Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: 60})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return result.(*preview.OfficePreview)
	}
	first := render()

	// Same size and modification time but undecodable content: only a cache hit
	// can still produce the page.
	if err := os.WriteFile(path, make([]byte, src.Size()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, src.ModTime(), src.ModTime()); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	second := render()
	if string(second.Page.Data) != string(first.Page.Data) {
		t.Fatal("cached office page differs from the first render")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("cached office preview took %v", elapsed)
	}
}

// A revisited video comes from the disk cache: no ffprobe or ffmpeg run.
func TestVideoPreviewServesFromDiskCache(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().Video, "ffprobe and ffmpeg")
	path := writeVideoSample(t)
	src, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectPath(path, core.File)
	render := func() *preview.VideoPreview {
		t.Helper()
		result, err := service.Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: 20})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return result.(*preview.VideoPreview)
	}
	first := render()

	// Same size and modification time but undecodable content: only a cache hit
	// can still produce the preview.
	if err := os.WriteFile(path, make([]byte, src.Size()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, src.ModTime(), src.ModTime()); err != nil {
		t.Fatal(err)
	}
	second := render()
	if string(second.Frame.Data) != string(first.Frame.Data) || second.Video != first.Video {
		t.Fatal("cached video preview differs from the first render")
	}
	if len(second.Metadata) == 0 {
		t.Fatal("cached video preview lost its metadata fields")
	}
}

// A revisited audio file comes from the disk cache: no ffprobe or ffmpeg run.
func TestAudioPreviewServesFromDiskCache(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().Audio, "ffprobe and ffmpeg")
	path := writeWAVSample(t)
	src, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectPath(path, core.File)
	render := func() *preview.AudioPreview {
		t.Helper()
		result, err := service.Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: 20})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return result.(*preview.AudioPreview)
	}
	first := render()

	// Same size and modification time but undecodable content: only a cache hit
	// can still produce the preview.
	if err := os.WriteFile(path, make([]byte, src.Size()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, src.ModTime(), src.ModTime()); err != nil {
		t.Fatal(err)
	}
	second := render()
	if string(second.Visual.Data) != string(first.Visual.Data) || second.Audio != first.Audio {
		t.Fatal("cached audio preview differs from the first render")
	}
	if second.Audio.Visual != preview.Waveform || len(second.Metadata) == 0 {
		t.Fatalf("cached audio preview lost its details: %#v", second.Audio)
	}
}

// A revisited SVG comes from the disk cache: the renderer doesn't run again.
func TestSVGPreviewServesFromDiskCache(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().SVG, "rsvg-convert or imagemagick")
	data, err := os.ReadFile(filepath.Join("file_samples", "diamond.svg"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "diamond.svg")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectPath(path, core.File)
	render := func() *preview.SVGPreview {
		t.Helper()
		result, err := service.Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: 20})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return result.(*preview.SVGPreview)
	}
	first := render()

	// Same size and modification time but no longer an SVG: only a cache hit
	// can still produce the image.
	if err := os.WriteFile(path, make([]byte, src.Size()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, src.ModTime(), src.ModTime()); err != nil {
		t.Fatal(err)
	}
	second := render()
	if string(second.Image.Data) != string(first.Image.Data) || second.Image.Width != first.Image.Width {
		t.Fatal("cached SVG preview differs from the first render")
	}
}
