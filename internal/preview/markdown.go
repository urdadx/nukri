package preview

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/glamour"
)

func (s *Service) renderMarkdown(ctx context.Context, path string, width int) (*MarkdownPreview, error) {
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
	width = markdownWidth(width)
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
	)
	if err != nil {
		return nil, fmt.Errorf("create Markdown renderer: %w", err)
	}
	text, err := renderer.Render(sanitizeMarkdown(string(source)))
	if err != nil {
		return nil, fmt.Errorf("render Markdown: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(text)) > s.maxToolOutput {
		return nil, ErrOutputTooLarge
	}
	return &MarkdownPreview{Text: text}, nil
}

func markdownWidth(width int) int {
	if width <= 0 {
		return DefaultMarkdownWidth
	}
	if width < 20 {
		return 20
	}
	if width > 300 {
		return 300
	}
	return width
}

func sanitizeMarkdown(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, value)
}
