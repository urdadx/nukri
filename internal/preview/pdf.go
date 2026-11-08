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
	return s.renderConvertedPDF(ctx, path, cellWidth, nil)
}

// pdfConverter turns a source document into a PDF inside directory and returns
// the PDF's path.
type pdfConverter func(ctx context.Context, directory string) (string, error)

/*
renderConvertedPDF renders the first page of source, first converting it to a
PDF when convert is non-nil (office documents, ebooks). The page is cached by
the source file's identity, so a cache hit skips the conversion too — which for
LibreOffice or ebook-convert is by far the slowest step.
*/
func (s *Service) renderConvertedPDF(ctx context.Context, path string, cellWidth int, convert pdfConverter) (*PDFPreview, error) {
	if !s.canRenderPDF() {
		return nil, fmt.Errorf("PDF preview: %w", ToolUnavailable("pdftoppm/pdfinfo"))
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

	directory, err := os.MkdirTemp("", "nukri-pdf-*")
	if err != nil {
		return nil, fmt.Errorf("create PDF preview directory: %w", err)
	}
	defer os.RemoveAll(directory)
	pdfPath := path
	if convert != nil {
		if pdfPath, err = convert(ctx, directory); err != nil {
			return nil, err
		}
	}

	metadata, err := s.pdfMetadata(ctx, pdfPath)
	if err != nil {
		return nil, err
	}
	image, err := s.rasterizeFirstPage(ctx, pdfPath, renderSize, directory)
	if err != nil {
		return nil, err
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

// canRenderPDF reports whether the poppler tools needed for PDF pages exist.
func (s *Service) canRenderPDF() bool {
	return s.tools.PDFInfo != "" && (s.tools.PDFToPPM != "" || s.tools.PDFToCairo != "")
}

/*
rasterizeFirstPage renders page one of pdfPath to a PNG whose longest side is
size. pdftoppm writes raw pixels that are PNG-encoded here with fast
compression: poppler's own PNG output compresses hard and costs several times
the rendering itself. pdftocairo is the fallback when pdftoppm is missing.
*/
func (s *Service) rasterizeFirstPage(ctx context.Context, pdfPath string, size int, directory string) (Image, error) {
	scale := strconv.Itoa(size)
	if s.tools.PDFToPPM != "" {
		limit := int64(s.maxImageDimension)*int64(s.maxImageDimension)*3 + 64
		raw, err := runCommand(ctx, limit, s.tools.PDFToPPM, "-singlefile", "-f", "1", "-l", "1", "-scale-to", scale, pdfPath) // no output root: PPM on stdout
		if err != nil {
			return Image{}, fmt.Errorf("render PDF first page: %w", err)
		}
		page, err := decodePPM(raw, s.maxImageDimension)
		if err != nil {
			return Image{}, fmt.Errorf("render PDF first page: %w", err)
		}
		output := &limitedBuffer{limit: s.maxImageBytes}
		encoder := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := encoder.Encode(output, page); err != nil {
			return Image{}, fmt.Errorf("encode PDF first page: %w", err)
		}
		bounds := page.Bounds()
		return Image{MediaType: "image/png", Data: output.buffer.Bytes(), Width: bounds.Dx(), Height: bounds.Dy()}, nil
	}
	prefix := filepath.Join(directory, "page")
	if _, err := runCommand(ctx, 64<<10, s.tools.PDFToCairo, "-png", "-singlefile", "-f", "1", "-l", "1", "-scale-to", scale, pdfPath, prefix); err != nil {
		return Image{}, fmt.Errorf("render PDF first page: %w", err)
	}
	// pdftocairo can round one pixel past -scale-to (1601 for 1600).
	image, err := readPNG(prefix+".png", s.maxImageBytes, s.maxImageDimension+1)
	if err != nil {
		return Image{}, fmt.Errorf("read PDF first page: %w", err)
	}
	return image, nil
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
