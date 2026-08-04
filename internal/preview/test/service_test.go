package test

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
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

func TestRenderCSV(t *testing.T) {
	service := preview.NewService()
	for _, test := range []struct {
		name      string
		delimiter rune
		rows      int
		columns   int
	}{
		{name: "minimal.csv", delimiter: ',', rows: 3, columns: 3},
		{name: "minimal.tsv", delimiter: '\t', rows: 2, columns: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := renderSample(t, service, test.name)
			value, ok := result.(*preview.CSVPreview)
			if !ok {
				t.Fatalf("result = %T, want *preview.CSVPreview", result)
			}
			if value.Metadata.Delimiter != test.delimiter || value.Metadata.RowCount != test.rows || value.Metadata.ColumnCount != test.columns {
				t.Fatalf("metadata = %#v", value.Metadata)
			}
			if len(value.Rows) != test.rows || value.RowsTruncated || value.ColumnsTruncated || value.CellsTruncated {
				t.Fatalf("preview = %#v", value)
			}
			table := preview.RenderCSVTable(value, preview.CSVTableOptions{Width: 40})
			if !strings.Contains(table, value.Rows[0][0]) || !strings.Contains(table, fmt.Sprintf("Rows 1-%d of %d", test.rows, test.rows)) {
				t.Fatalf("table = %q", table)
			}
		})
	}
}

func TestRenderCSVTableHorizontalOffset(t *testing.T) {
	value := &preview.CSVPreview{
		Rows:     [][]string{{"alpha", "beta", "gamma", "delta"}, {"1", "2", "3", "4"}},
		Metadata: preview.CSVMetadata{RowCount: 2, ColumnCount: 4, Delimiter: ','},
	}
	table := preview.RenderCSVTable(value, preview.CSVTableOptions{Width: 20, ColumnOffset: 2})
	if strings.Contains(table, "alpha") || !strings.Contains(table, "gamma") || !strings.Contains(table, "more left") {
		t.Fatalf("table = %q", table)
	}
}

func TestRenderCSVTruncatesRaggedData(t *testing.T) {
	var source strings.Builder
	for row := 0; row < 101; row++ {
		columns := 2
		if row == 0 {
			columns = 51
		}
		for column := 0; column < columns; column++ {
			if column != 0 {
				source.WriteByte(',')
			}
			if row == 1 && column == 0 {
				source.WriteString(strings.Repeat("x", 1_001))
			} else {
				source.WriteString("value")
			}
		}
		source.WriteByte('\n')
	}
	path := filepath.Join(t.TempDir(), "large.csv")
	if err := os.WriteFile(path, []byte(source.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectPath(path, core.File)
	result, err := preview.NewService().Render(context.Background(), preview.Request{Path: path, Facts: facts})
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.(*preview.CSVPreview)
	if !ok {
		t.Fatalf("result = %T, want *preview.CSVPreview", result)
	}
	if value.Metadata.RowCount != 101 || value.Metadata.ColumnCount != 51 || len(value.Rows) != 100 {
		t.Fatalf("preview dimensions = %#v", value)
	}
	if !value.RowsTruncated || !value.ColumnsTruncated || !value.CellsTruncated {
		t.Fatalf("truncation flags = %#v", value)
	}
}

func TestRenderTorrent(t *testing.T) {
	service := preview.NewService()
	result := renderSample(t, service, "minimal.torrent")
	value, ok := result.(*preview.TorrentPreview)
	if !ok {
		t.Fatalf("result = %T, want *preview.TorrentPreview", result)
	}
	if value.Torrent.Name != "sample.txt" || value.Torrent.TotalSize != 12_345 || !value.Torrent.Private {
		t.Fatalf("torrent = %#v", value.Torrent)
	}
	if value.Torrent.InfoHashV1 == "" || len(value.Torrent.Trackers) != 1 {
		t.Fatalf("torrent hashes/trackers = %#v", value.Torrent)
	}
	if len(value.Files) != 1 || value.Files[0].Path != "sample.txt" || value.Files[0].Size != 12_345 {
		t.Fatalf("files = %#v", value.Files)
	}
	view, err := preview.BuildView(value, preview.ViewOptions{Width: 60})
	if err != nil {
		t.Fatal(err)
	}
	if view.Title != "sample.txt" || !strings.Contains(strings.Join(view.Lines, "\n"), "Info hash v1") {
		t.Fatalf("view = %#v", view)
	}
}

func TestRenderTorrentRejectsMalformedMetainfo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.torrent")
	if err := os.WriteFile(path, []byte("not bencode"), 0o600); err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectPath(path, core.File)
	if _, err := preview.NewService().Render(context.Background(), preview.Request{Path: path, Facts: facts}); err == nil {
		t.Fatal("malformed torrent should fail")
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

func TestRenderISO(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().ISO, "isoinfo")
	path := writeISOSample(t)
	facts := fileinfo.InspectPath(path, core.File)
	result, err := service.Render(context.Background(), preview.Request{Path: path, Facts: facts})
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.(*preview.ISOPreview)
	if !ok {
		t.Fatalf("result = %T, want *preview.ISOPreview", result)
	}
	if value.ISO.VolumeID != "NUKRI_TEST" || value.ISO.BlockSize == 0 || value.ISO.FileSize == 0 {
		t.Fatalf("ISO metadata = %#v", value.ISO)
	}
	found := false
	for _, entry := range value.Entries {
		if strings.HasSuffix(entry.Path, "/hello.txt") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ISO entries = %#v, want hello.txt", value.Entries)
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

func writeISOSample(t *testing.T) string {
	t.Helper()
	genisoimage, err := exec.LookPath("genisoimage")
	if err != nil {
		t.Skip("ISO fixture creation requires genisoimage")
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "hello.txt"), []byte("hello ISO"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(directory, "sample.iso")
	command := exec.Command(genisoimage, "-quiet", "-R", "-V", "NUKRI_TEST", "-o", output, source)
	if commandOutput, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create ISO sample: %v: %s", err, commandOutput)
	}
	return output
}
