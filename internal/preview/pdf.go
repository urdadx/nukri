package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

/*
pdfSupersample is the resolution multiplier applied to the PDF page on
rasterization. The page is rendered larger than the on-screen pixel size and
the terminal scales it down, which keeps text and vector detail crisp.
Rendering directly at the display size would force the terminal to upscale,
producing a blurry preview.
*/
const pdfSupersample = 2

/*
renderPDF renders the first page of a PDF to a PNG preview. Rendering is
delegated to the external pdftocairo tool at a supersampled resolution of the
display target size. The rendered page (and its metadata) is cached on disk
keyed by the source file identity and target size, so revisiting a PDF — or
rendering the same size again — skips the (relatively expensive)
rasterization pass entirely.
*/
func (s *Service) renderPDF(ctx context.Context, path string, cellWidth int) (*PDFPreview, error) {
	if s.tools.PDFInfo == "" || s.tools.PDFToCairo == "" {
		return nil, fmt.Errorf("PDF preview: %w", ToolUnavailable("pdftocairo/pdfinfo"))
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat PDF preview: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	targetSize := s.imageTargetSize(cellWidth)
	renderSize := min(targetSize*pdfSupersample, s.maxImageDimension)
	cacheKey := s.imageCache.Key(path, info.Size(), info.ModTime(), targetSize)
	if cached, ok := s.imageCache.Get(cacheKey); ok {
		metadata, err := s.decodeMeta(cached.Metadata)
		if err != nil {
			return nil, fmt.Errorf("decode cached PDF metadata: %w", err)
		}
		return &PDFPreview{
			Page: Image{
				MediaType: "image/png",
				Data:      append([]byte(nil), cached.Data...),
				Width:     cached.Width,
				Height:    cached.Height,
			},
			Metadata: metadata,
		}, nil
	}

	metadata, err := s.pdfMetadata(ctx, path)
	if err != nil {
		return nil, err
	}

	directory, err := os.MkdirTemp("", "nukri-pdf-*")
	if err != nil {
		return nil, fmt.Errorf("create PDF preview directory: %w", err)
	}
	defer os.RemoveAll(directory)
	prefix := filepath.Join(directory, "page")
	_, err = runCommand(ctx, 64<<10, s.tools.PDFToCairo,
		"-png", "-singlefile", "-f", "1", "-l", "1",
		"-scale-to", strconv.Itoa(renderSize), path, prefix,
	)
	if err != nil {
		return nil, fmt.Errorf("render PDF first page: %w", err)
	}
	image, err := readPNG(prefix+".png", s.maxImageBytes, s.maxImageDimension)
	if err != nil {
		return nil, fmt.Errorf("read PDF first page: %w", err)
	}
	encodedMeta, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("encode PDF metadata: %w", err)
	}
	if image.Width > 0 && image.Height > 0 {
		s.imageCache.Put(cacheKey, image.Data, image.Width, image.Height, "png", image.Width, image.Height, info.Size(), encodedMeta)
	}
	return &PDFPreview{Page: image, Metadata: metadata}, nil
}

// decodeMeta deserializes metadata previously stored via cache Put. A nil or
// empty payload yields an empty (non-nil) metadata slice.
func (s *Service) decodeMeta(data []byte) ([]Field, error) {
	if len(data) == 0 {
		return []Field{}, nil
	}
	var fields []Field
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

/*
pdfMetadata runs pdfinfo to gather PDF metadata. This is cheap relative to the
page rasterization, and its result is stored in the page cache so it is not
re-run on a cache hit.
*/
func (s *Service) pdfMetadata(ctx context.Context, path string) ([]Field, error) {
	metadataOutput, err := runCommand(ctx, 256<<10, s.tools.PDFInfo, path)
	if err != nil {
		return nil, fmt.Errorf("read PDF metadata: %w", err)
	}
	return parsePDFInfo(string(metadataOutput)), nil
}

func parsePDFInfo(output string) []Field {
	fields := make([]Field, 0, 16)
	for _, line := range strings.Split(output, "\n") {
		name, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		fields = append(fields, Field{Name: safeText(strings.TrimSpace(name)), Value: safeText(strings.TrimSpace(value))})
	}
	return fields
}

/*
readPNG reads a PNG file from disk and returns its contents as an Image. It
enforces maximum size and dimension limits to avoid loading excessively large
images into memory.
*/
func readPNG(path string, maximumBytes int64, maximumDimension int) (Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return Image{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Image{}, err
	}
	if info.Size() > maximumBytes {
		return Image{}, ErrOutputTooLarge
	}
	config, err := png.DecodeConfig(file)
	if err != nil {
		return Image{}, fmt.Errorf("decode PNG header: %w", err)
	}
	if config.Width > maximumDimension || config.Height > maximumDimension {
		return Image{}, ErrOutputTooLarge
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Image{}, err
	}
	return Image{MediaType: "image/png", Data: data, Width: config.Width, Height: config.Height}, nil
}
