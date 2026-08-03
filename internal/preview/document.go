package preview

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/urdadx/nukri/internal/file_info"
)

func isOfficeDocument(format fileinfo.DocumentFormat) bool {
	switch format {
	case fileinfo.Doc, fileinfo.Docx, fileinfo.Docm, fileinfo.Odt,
		fileinfo.Xls, fileinfo.Xlsx, fileinfo.Xlsm,
		fileinfo.Pptx, fileinfo.Pptm, fileinfo.Odp, fileinfo.Ods,
		fileinfo.Pages, fileinfo.Document:
		return true
	default:
		return false
	}
}

func isEbook(format fileinfo.DocumentFormat) bool {
	return format == fileinfo.Mobi || format == fileinfo.Azw3
}

func (s *Service) renderOfficeDocument(ctx context.Context, path string, format fileinfo.DocumentFormat) (*OfficePreview, error) {
	if s.tools.LibreOffice == "" {
		return nil, fmt.Errorf("document preview: %w", ErrToolUnavailable)
	}
	directory, err := os.MkdirTemp("", "nukri-office-*")
	if err != nil {
		return nil, fmt.Errorf("create document preview directory: %w", err)
	}
	defer os.RemoveAll(directory)
	profile := filepath.Join(directory, "profile")
	profileURL := (&url.URL{Scheme: "file", Path: profile}).String()
	_, err = runCommand(ctx, s.maxToolOutput, s.tools.LibreOffice,
		"-env:UserInstallation="+profileURL,
		"--headless", "--convert-to", "pdf", "--outdir", directory, path,
	)
	if err != nil {
		return nil, fmt.Errorf("convert document to PDF: %w", err)
	}
	output := filepath.Join(directory, trimExtension(filepath.Base(path))+".pdf")
	if _, err := os.Stat(output); err != nil {
		return nil, fmt.Errorf("locate converted document: %w", err)
	}
	pdf, err := s.renderPDF(ctx, output)
	if err != nil {
		return nil, err
	}
	return &OfficePreview{Format: format, Page: pdf.Page, Metadata: pdf.Metadata}, nil
}

func (s *Service) renderEbook(ctx context.Context, path string, format fileinfo.DocumentFormat) (*EbookPreview, error) {
	if s.tools.EbookConvert == "" {
		return nil, fmt.Errorf("ebook preview: %w", ErrToolUnavailable)
	}
	directory, err := os.MkdirTemp("", "nukri-ebook-*")
	if err != nil {
		return nil, fmt.Errorf("create ebook preview directory: %w", err)
	}
	defer os.RemoveAll(directory)
	output := filepath.Join(directory, "book.pdf")
	if _, err := runCommand(ctx, s.maxToolOutput, s.tools.EbookConvert, path, output); err != nil {
		return nil, fmt.Errorf("convert ebook to PDF: %w", err)
	}
	pdf, err := s.renderPDF(ctx, output)
	if err != nil {
		return nil, err
	}
	return &EbookPreview{Format: format, Page: pdf.Page, Metadata: pdf.Metadata}, nil
}

func trimExtension(name string) string {
	return name[:len(name)-len(filepath.Ext(name))]
}
