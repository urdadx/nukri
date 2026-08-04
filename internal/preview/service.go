package preview

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/file_info"
)

type Service struct {
	tools               Tools
	maxImageDimension   int
	maxImageBytes       int64
	maxArchiveEntries   int
	maxToolOutput       int64
	maxEPUBBytes        int64
	maxEPUBEntryBytes   uint64
	maxEPUBTotalBytes   uint64
	maxSVGBytes         int64
	maxMarkdownBytes    int64
	maxFontBytes        int64
	maxDirectoryEntries int
	maxCSVBytes         int64
	maxCSVRows          int
	maxCSVColumns       int
	maxCSVCellRunes     int
}

func NewService() *Service {
	return NewServiceWithTools(DiscoverTools())
}

func NewServiceWithTools(tools Tools) *Service {
	return &Service{
		tools:               tools,
		maxImageDimension:   DefaultMaxImageDimension,
		maxImageBytes:       DefaultMaxImageBytes,
		maxArchiveEntries:   DefaultMaxArchiveEntries,
		maxToolOutput:       DefaultMaxToolOutput,
		maxEPUBBytes:        DefaultMaxEPUBBytes,
		maxEPUBEntryBytes:   DefaultMaxEPUBEntryBytes,
		maxEPUBTotalBytes:   DefaultMaxEPUBTotalBytes,
		maxSVGBytes:         DefaultMaxSVGBytes,
		maxMarkdownBytes:    DefaultMaxMarkdownBytes,
		maxFontBytes:        DefaultMaxFontBytes,
		maxDirectoryEntries: DefaultMaxDirectoryEntries,
		maxCSVBytes:         DefaultMaxCSVBytes,
		maxCSVRows:          DefaultMaxCSVRows,
		maxCSVColumns:       DefaultMaxCSVColumns,
		maxCSVCellRunes:     DefaultMaxCSVCellRunes,
	}
}

func DiscoverTools() Tools {
	return Tools{
		PDFInfo:      lookPath("pdfinfo"),
		PDFToCairo:   lookPath("pdftocairo"),
		SevenZip:     lookPath("7z", "7zz"),
		LibreOffice:  lookPath("libreoffice", "soffice"),
		EbookConvert: lookPath("ebook-convert"),
		FFProbe:      lookPath("ffprobe"),
		FFmpeg:       lookPath("ffmpeg"),
	}
}

func lookPath(names ...string) string {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func (s *Service) Capabilities() Capabilities {
	return Capabilities{
		PDF:       s.tools.PDFInfo != "" && s.tools.PDFToCairo != "",
		SVG:       true,
		Archives:  s.tools.SevenZip != "",
		Documents: s.tools.LibreOffice != "" && s.tools.PDFInfo != "" && s.tools.PDFToCairo != "",
		EPUB:      true,
		Ebooks:    s.tools.EbookConvert != "" && s.tools.PDFInfo != "" && s.tools.PDFToCairo != "",
		Fonts:     true,
		Audio:     s.tools.FFProbe != "" && s.tools.FFmpeg != "",
		Video:     s.tools.FFProbe != "" && s.tools.FFmpeg != "",
	}
}

func (s *Service) Render(ctx context.Context, request Request) (Preview, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := filepath.Abs(request.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve preview path: %w", err)
	}
	if request.Facts.BuiltinClass == core.FileClassDirectory {
		return s.renderDirectory(ctx, path)
	}
	if request.Facts.Preview.DocumentFormat != nil {
		format := *request.Facts.Preview.DocumentFormat
		switch {
		case format == fileinfo.Pdf:
			return s.renderPDF(ctx, path)
		case format == fileinfo.Epub:
			return s.renderEPUB(ctx, path)
		case isOfficeDocument(format):
			return s.renderOfficeDocument(ctx, path, format)
		case isEbook(format):
			return s.renderEbook(ctx, path, format)
		}
	}
	if request.Facts.Preview.Kind == fileinfo.Markdown {
		return s.renderMarkdown(ctx, path, request.Width)
	}
	if request.Facts.Preview.Kind == fileinfo.Csv {
		return s.renderCSV(ctx, path)
	}
	if request.Facts.BuiltinClass == core.FileClassImage && strings.EqualFold(filepath.Ext(path), ".svg") {
		return s.renderSVG(ctx, path)
	}
	if request.Facts.BuiltinClass == core.FileClassFont {
		return s.renderFont(ctx, path)
	}
	if request.Facts.BuiltinClass == core.FileClassAudio {
		return s.renderAudio(ctx, path)
	}
	if request.Facts.BuiltinClass == core.FileClassVideo {
		return s.renderVideo(ctx, path)
	}
	if request.Facts.BuiltinClass == core.FileClassArchive {
		return s.listArchive(ctx, path)
	}
	return nil, ErrUnsupported
}
