package test

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/file_info"
	"github.com/urdadx/nukri/internal/preview"
)

func TestRenderPDFFirstPage(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().PDF, "pdfinfo and pdftocairo")
	result := renderSample(t, service, "session_history.pdf")
	pdf, ok := result.(*preview.PDFPreview)
	if !ok {
		t.Fatalf("result = %T, want *preview.PDFPreview", result)
	}
	assertPNGImage(t, &pdf.Page)
	if len(pdf.Metadata) == 0 {
		t.Fatal("PDF preview returned no metadata")
	}
}

func TestRenderMarkdown(t *testing.T) {
	service := preview.NewService()
	path := filepath.Join("file_samples", "minimal.md")
	facts := fileinfo.InspectPath(path, core.File)
	result, err := service.Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: 60})
	if err != nil {
		t.Fatal(err)
	}
	markdown, ok := result.(*preview.MarkdownPreview)
	if !ok || markdown.Text == "" {
		t.Fatalf("result = %#v, want rendered text", result)
	}
	visibleText := ansi.Strip(markdown.Text)
	for _, value := range []string{"Nukri Markdown", "bold text", "First item", "fmt.Println"} {
		if !strings.Contains(visibleText, value) {
			t.Errorf("rendered Markdown does not contain %q: %q", value, visibleText)
		}
	}
}

func TestRenderSVG(t *testing.T) {
	service := preview.NewService()
	result := renderSample(t, service, "diamond.svg")
	svg, ok := result.(*preview.SVGPreview)
	if !ok {
		t.Fatalf("result = %T, want *preview.SVGPreview", result)
	}
	assertPNGImage(t, &svg.Image)
	if svg.Image.Width != 24 || svg.Image.Height != 30 {
		t.Fatalf("image dimensions = %dx%d, want 24x30", svg.Image.Width, svg.Image.Height)
	}
}

func TestRenderFont(t *testing.T) {
	service := preview.NewService()
	path := extractFontSample(t)
	facts := fileinfo.InspectPath(path, core.File)
	result, err := service.Render(context.Background(), preview.Request{Path: path, Facts: facts})
	if err != nil {
		t.Fatal(err)
	}
	font, ok := result.(*preview.FontPreview)
	if !ok {
		t.Fatalf("result = %#v, want font preview", result)
	}
	if !strings.Contains(font.Font.Family, "Roobert") || font.Font.Glyphs == 0 || font.Font.UnitsPerEm == 0 {
		t.Fatalf("font metadata = %#v", font.Font)
	}
	assertPNGImage(t, &font.Specimen)
}

func TestRenderDirectory(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	service := preview.NewService()
	facts := fileinfo.InspectPath(directory, core.Directory)
	result, err := service.Render(context.Background(), preview.Request{Path: directory, Facts: facts})
	if err != nil {
		t.Fatal(err)
	}
	directoryPreview, ok := result.(*preview.DirectoryPreview)
	if !ok {
		t.Fatalf("result = %#v, want two directory entries", result)
	}
	if directoryPreview.TotalItems != 2 || directoryPreview.FolderCount != 1 || directoryPreview.FileCount != 1 {
		t.Fatalf("directory preview = %#v", directoryPreview)
	}
}

func TestRenderAudio(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().Audio, "ffprobe and ffmpeg")
	path := writeWAVSample(t)
	facts := fileinfo.InspectPath(path, core.File)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := service.Render(ctx, preview.Request{Path: path, Facts: facts})
	if err != nil {
		t.Fatal(err)
	}
	audio, ok := result.(*preview.AudioPreview)
	if !ok {
		t.Fatalf("result = %#v, want audio preview", result)
	}
	if audio.Audio.Codec == "" || audio.Audio.SampleRate != 8000 || audio.Audio.Channels != 1 {
		t.Fatalf("audio metadata = %#v", audio.Audio)
	}
	if audio.Audio.Duration < 0.9 || audio.Audio.Duration > 1.1 || audio.Audio.Visual != preview.Waveform {
		t.Fatalf("audio duration/visual = %#v", audio.Audio)
	}
	assertPNGImage(t, &audio.Visual)
}

func TestRenderVideo(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().Video, "ffprobe and ffmpeg")
	path := writeVideoSample(t)
	facts := fileinfo.InspectPath(path, core.File)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := service.Render(ctx, preview.Request{Path: path, Facts: facts})
	if err != nil {
		t.Fatal(err)
	}
	video, ok := result.(*preview.VideoPreview)
	if !ok {
		t.Fatalf("result = %#v, want video preview", result)
	}
	if video.Video.Codec == "" || video.Video.Width != 320 || video.Video.Height != 180 {
		t.Fatalf("video metadata = %#v", video.Video)
	}
	if video.Video.Duration < 0.9 || video.Video.Duration > 1.1 {
		t.Fatalf("video duration = %v, want about one second", video.Video.Duration)
	}
	assertPNGImage(t, &video.Frame)
}

func TestListArchive(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().Archives, "7z or 7zz")
	result := renderSample(t, service, "roobert-font-family.zip")

	archive, ok := result.(*preview.ArchivePreview)
	if !ok {
		t.Fatalf("result = %#v, want archive listing", result)
	}
	if len(archive.Archive.Entries) == 0 {
		t.Fatal("archive preview returned no entries")
	}
	for _, entry := range archive.Archive.Entries {
		if entry.Path == "" {
			t.Fatalf("archive contains an entry without a path: %#v", entry)
		}
	}
}

func TestRenderOfficeDocumentThroughPDF(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().Documents, "libreoffice, pdfinfo, and pdftocairo")
	result := renderSample(t, service, "Third Party.xlsx")
	office, ok := result.(*preview.OfficePreview)
	if !ok || office.Format != fileinfo.Xlsx {
		t.Fatalf("result = %#v, want XLSX office preview", result)
	}
	assertPNGImage(t, &office.Page)
	if len(office.Metadata) == 0 {
		t.Fatal("converted spreadsheet returned no PDF metadata")
	}
}

func TestParseEPUB(t *testing.T) {
	service := preview.NewService()
	result := renderSample(t, service, "minimal.epub")

	epub, ok := result.(*preview.EPUBPreview)
	if !ok {
		t.Fatalf("result = %#v, want EPUB metadata", result)
	}
	if epub.Book.Identifier != "isbn" {
		t.Fatalf("identifier = %q, want isbn", epub.Book.Identifier)
	}
	if len(epub.Book.Titles) != 1 || epub.Book.Titles[0] != "Sample .epub Book" {
		t.Fatalf("titles = %#v", epub.Book.Titles)
	}
	if len(epub.Book.Creators) != 1 || epub.Book.Creators[0] != "Thomas Hansen" {
		t.Fatalf("creators = %#v", epub.Book.Creators)
	}
	if len(epub.Book.Languages) != 1 || epub.Book.Languages[0] != "en" {
		t.Fatalf("languages = %#v", epub.Book.Languages)
	}
}

func TestUnavailableToolAndCancellation(t *testing.T) {
	service := preview.NewServiceWithTools(preview.Tools{})
	facts := file_infoForDocument(fileinfo.Pdf)
	_, err := service.Render(context.Background(), preview.Request{Path: "book.pdf", Facts: facts})
	if !errors.Is(err, preview.ErrToolUnavailable) {
		t.Fatalf("error = %v, want ErrToolUnavailable", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = service.Render(ctx, preview.Request{Path: "book.pdf", Facts: facts})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func renderSample(t *testing.T, service *preview.Service, name string) preview.Preview {
	t.Helper()
	path := filepath.Join("file_samples", name)
	facts := fileinfo.InspectPath(path, core.File)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := service.Render(ctx, preview.Request{Path: path, Facts: facts})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertPNGImage(t *testing.T, image *preview.Image) {
	t.Helper()
	if image == nil || image.MediaType != "image/png" || len(image.Data) == 0 {
		t.Fatalf("image = %#v, want non-empty PNG", image)
	}
	if image.Width <= 0 || image.Height <= 0 {
		t.Fatalf("image dimensions = %dx%d", image.Width, image.Height)
	}
}

func requireCapability(t *testing.T, available bool, tools string) {
	t.Helper()
	if !available {
		t.Skip("preview requires " + tools + " in PATH")
	}
}

func file_infoForDocument(format fileinfo.DocumentFormat) fileinfo.FileFacts {
	return fileinfo.FileFacts{BuiltinClass: core.FileClassDocument, Preview: fileinfo.DocumentPreview(format)}
}

func extractFontSample(t *testing.T) string {
	t.Helper()
	archive, err := zip.OpenReader(filepath.Join("file_samples", "roobert-font-family.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	const sample = "RoobertMonoTRIAL-Regular-BF67243fd29a433.otf"
	for _, entry := range archive.File {
		if entry.Name != sample {
			continue
		}
		input, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer input.Close()
		path := filepath.Join(t.TempDir(), sample)
		output, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(output, input); err != nil {
			output.Close()
			t.Fatal(err)
		}
		if err := output.Close(); err != nil {
			t.Fatal(err)
		}
		return path
	}
	t.Fatalf("font sample %q was not found", sample)
	return ""
}

func writeWAVSample(t *testing.T) string {
	t.Helper()
	const sampleRate = 8000
	const sampleCount = sampleRate
	const dataSize = sampleCount * 2
	path := filepath.Join(t.TempDir(), "tone.wav")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	write := func(value any) {
		if err := binary.Write(file, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := file.WriteString("RIFF"); err != nil {
		t.Fatal(err)
	}
	write(uint32(36 + dataSize))
	if _, err := file.WriteString("WAVEfmt "); err != nil {
		t.Fatal(err)
	}
	write(uint32(16))
	write(uint16(1))
	write(uint16(1))
	write(uint32(sampleRate))
	write(uint32(sampleRate * 2))
	write(uint16(2))
	write(uint16(16))
	if _, err := file.WriteString("data"); err != nil {
		t.Fatal(err)
	}
	write(uint32(dataSize))
	for index := 0; index < sampleCount; index++ {
		sample := int16(math.Sin(2*math.Pi*440*float64(index)/sampleRate) * 12_000)
		write(sample)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeVideoSample(t *testing.T) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sample.mp4")
	command := exec.Command(ffmpeg,
		"-v", "error", "-nostdin", "-y",
		"-f", "lavfi", "-i", "color=c=0x1CB0F6:s=320x180:d=1",
		"-c:v", "mpeg4", "-pix_fmt", "yuv420p", path,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create video sample: %v: %s", err, output)
	}
	return path
}
