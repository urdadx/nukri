package preview

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	fileinfo "github.com/urdadx/nukri/internal/file_info"
)

func (s *Service) renderText(ctx context.Context, path string, facts fileinfo.FileFacts) (*TextPreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open text preview: %w", err)
	}
	defer file.Close()
	source, err := io.ReadAll(io.LimitReader(file, s.maxMarkdownBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read text preview: %w", err)
	}
	if int64(len(source)) > s.maxMarkdownBytes {
		return nil, ErrOutputTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	title := "Text"
	if facts.SpecificTypeLabel != nil && *facts.SpecificTypeLabel != "" {
		title = *facts.SpecificTypeLabel
	} else if facts.Preview.CodeSyntax != nil && *facts.Preview.CodeSyntax != "" {
		title = *facts.Preview.CodeSyntax
	}
	return &TextPreview{Title: title, Text: sanitizeText(string(source))}, nil
}

func sanitizeText(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, value)
}
