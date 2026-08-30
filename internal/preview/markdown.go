package preview

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

const markdownLineLimit = 800

func (s *Service) renderMarkdown(ctx context.Context, path string, _ int) (*MarkdownPreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Markdown: %w", err)
	}
	defer file.Close()

	source, err := io.ReadAll(io.LimitReader(file, s.maxMarkdownBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Markdown: %w", err)
	}
	if int64(len(source)) > s.maxMarkdownBytes {
		return nil, ErrOutputTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	text := sanitizeMarkdown(string(source))
	lines := strings.Split(text, "\n")
	if len(lines) > markdownLineLimit {
		text = strings.Join(lines[:markdownLineLimit], "\n")
	}
	return &MarkdownPreview{Text: text}, nil
}

func sanitizeMarkdown(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, value)
}
