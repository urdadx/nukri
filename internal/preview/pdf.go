package preview

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Service) renderPDF(ctx context.Context, path string) (*PDFPreview, error) {
	if s.tools.PDFInfo == "" || s.tools.PDFToCairo == "" {
		return nil, fmt.Errorf("PDF preview: %w", ErrToolUnavailable)
	}
	metadataOutput, err := runCommand(ctx, 256<<10, s.tools.PDFInfo, path)
	if err != nil {
		return nil, fmt.Errorf("read PDF metadata: %w", err)
	}
	metadata := parsePDFInfo(string(metadataOutput))

	directory, err := os.MkdirTemp("", "nukri-pdf-*")
	if err != nil {
		return nil, fmt.Errorf("create PDF preview directory: %w", err)
	}
	defer os.RemoveAll(directory)
	prefix := filepath.Join(directory, "page")
	_, err = runCommand(ctx, 64<<10, s.tools.PDFToCairo,
		"-png", "-singlefile", "-f", "1", "-l", "1",
		"-scale-to", strconv.Itoa(s.maxImageDimension), path, prefix,
	)
	if err != nil {
		return nil, fmt.Errorf("render PDF first page: %w", err)
	}
	image, err := readPNG(prefix+".png", s.maxImageBytes, s.maxImageDimension)
	if err != nil {
		return nil, fmt.Errorf("read PDF first page: %w", err)
	}
	return &PDFPreview{Page: image, Metadata: metadata}, nil
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
