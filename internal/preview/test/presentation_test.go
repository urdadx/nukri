package test

import (
	"testing"

	"github.com/urdadx/nukri/internal/file_info"
	"github.com/urdadx/nukri/internal/preview"
)

func TestBuildViewForEveryPreviewType(t *testing.T) {
	image := preview.Image{MediaType: "image/png", Data: []byte("png"), Width: 100, Height: 50}
	metadata := []preview.Field{{Name: "Codec", Value: "example"}}
	tests := []struct {
		name       string
		value      preview.Preview
		wantTitle  string
		wantVisual bool
	}{
		{"PDF", &preview.PDFPreview{Page: image, Metadata: metadata}, "PDF", true},
		{"SVG", &preview.SVGPreview{Image: image}, "SVG", true},
		{"office", &preview.OfficePreview{Format: fileinfo.Docx, Page: image}, "Microsoft Word Open XML Document", true},
		{"ebook", &preview.EbookPreview{Format: fileinfo.Mobi, Page: image}, "Mobipocket eBook", true},
		{"Markdown", &preview.MarkdownPreview{Text: "heading\nbody"}, "Markdown", false},
		{"directory", &preview.DirectoryPreview{Entries: []preview.DirectoryEntry{{Name: "file.txt", Size: 4}}, TotalItems: 1, FileCount: 1}, "Directory", false},
		{"archive", &preview.ArchivePreview{Archive: preview.Archive{Entries: []preview.ArchiveEntry{{Path: "file.txt", Size: 4}}}}, "Archive", false},
		{"EPUB", &preview.EPUBPreview{Book: preview.EPUB{Titles: []string{"Book"}}, Metadata: metadata}, "Book", false},
		{"font", &preview.FontPreview{Font: preview.Font{Family: "Example", Subfamily: "Regular"}, Specimen: image}, "Example", true},
		{"audio", &preview.AudioPreview{Audio: preview.Audio{Title: "Song"}, Visual: image, Metadata: metadata}, "Song", true},
		{"video", &preview.VideoPreview{Video: preview.Video{Title: "Movie"}, Frame: image, Metadata: metadata}, "Movie", true},
		{"CSV", &preview.CSVPreview{Rows: [][]string{{"name"}, {"nukri"}}, Metadata: preview.CSVMetadata{RowCount: 2, ColumnCount: 1, Delimiter: ','}}, "CSV", false},
		{"torrent", &preview.TorrentPreview{Torrent: preview.Torrent{Name: "download"}, Files: []preview.TorrentFile{{Path: "file.bin", Size: 4}}}, "download", false},
		{"ISO", &preview.ISOPreview{ISO: preview.ISO{VolumeID: "INSTALL"}, Entries: []preview.ISOEntry{{Path: "/file.txt"}}}, "INSTALL", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view, err := preview.BuildView(test.value, preview.ViewOptions{Width: 60})
			if err != nil {
				t.Fatal(err)
			}
			if view.Title != test.wantTitle {
				t.Fatalf("title = %q, want %q", view.Title, test.wantTitle)
			}
			if (view.Visual != nil) != test.wantVisual {
				t.Fatalf("visual = %#v, want present %t", view.Visual, test.wantVisual)
			}
		})
	}
}

func TestBuildCSVViewSupportsHorizontalScrolling(t *testing.T) {
	value := &preview.CSVPreview{
		Rows:     [][]string{{"alpha", "beta", "gamma", "delta"}, {"1", "2", "3", "4"}},
		Metadata: preview.CSVMetadata{RowCount: 2, ColumnCount: 4, Delimiter: ','},
	}
	view, err := preview.BuildView(value, preview.ViewOptions{Width: 20, ColumnOffset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if view.Scroll != preview.BothScroll || view.Footer == "" {
		t.Fatalf("view = %#v", view)
	}
}

func TestBuildViewRejectsNil(t *testing.T) {
	if _, err := preview.BuildView(nil, preview.ViewOptions{}); err == nil {
		t.Fatal("nil preview should be rejected")
	}
}
