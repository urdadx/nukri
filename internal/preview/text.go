package preview

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/urdadx/nukri/internal/fileinfo"
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
	var codeLanguage string
	if facts.SpecificTypeLabel != nil && *facts.SpecificTypeLabel != "" {
		title = *facts.SpecificTypeLabel
	} else if facts.Preview.CodeSyntax != nil && *facts.Preview.CodeSyntax != "" {
		title = *facts.Preview.CodeSyntax
	}
	if facts.Preview.CodeSyntax != nil && *facts.Preview.CodeSyntax != "" &&
		(facts.Preview.CodeBackend == fileinfo.Chroma || facts.Preview.CodeBackend == fileinfo.Custom) {
		codeLanguage = *facts.Preview.CodeSyntax
	}
	text := string(source)
	if bytes.IndexByte(source, 0) >= 0 {
		text = "Binary file"
	}
	return &TextPreview{Title: title, Text: sanitizeText(text), CodeLanguage: codeLanguage}, nil
}

// textTabWidth is the tab stop spacing used when expanding tabs.
const textTabWidth = 4

/*
sanitizeText makes file text safe to draw in the pane. Windows (\r\n) and old
Mac (\r) line endings become \n: a carriage return sent to the terminal moves
the cursor back to column 0, and the rest of the line then overwrites the
panels to its left. Tabs are expanded, since the terminal moves to the next tab
stop while layout counts a tab as zero width. Other control characters are
dropped.
*/
func sanitizeText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, value)
	if !strings.ContainsRune(value, '\t') {
		return value
	}
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		lines[index] = expandTabs(line, textTabWidth)
	}
	return strings.Join(lines, "\n")
}

// expandTabs replaces tabs in line with spaces up to the next tab stop.
func expandTabs(line string, width int) string {
	if !strings.ContainsRune(line, '\t') {
		return line
	}
	var expanded strings.Builder
	column := 0
	for _, r := range line {
		if r == '\t' {
			spaces := width - column%width
			expanded.WriteString(strings.Repeat(" ", spaces))
			column += spaces
			continue
		}
		expanded.WriteRune(r)
		column++
	}
	return expanded.String()
}
