package preview

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	chroma "github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/urdadx/nukri/internal/preview/code/backend"
	"github.com/urdadx/nukri/internal/preview/code/registry"
)

// codeHighlightService dispatches registered languages to their highlighting
// backend (Chroma or the built-in custom tokenizers).
var codeHighlightService = backend.New()

var codeNewlinePattern = regexp.MustCompile(`\r?\n`)

// defaultCodeWindow is the number of leading lines rendered into the initial
// View for a highlighted code file. Rendering only a window keeps the first
// paint cheap for very large sources; the UI extends the window on scroll by
// re-requesting with a larger count, which cheaply reuses the disk cache.
const defaultCodeWindow = 256

// highlightCodeLines returns the source code rendered as ANSI-coloured lines,
// styled with the named Chroma style. Lines are returned plain when no lexer
// or token style applies to them.
func highlightCodeLines(ctx context.Context, codeSyntax, source, styleName string) ([]string, error) {
	language, ok := registry.LanguageForCodeSyntax(codeSyntax)
	if !ok {
		language = registry.Language(codeSyntax, codeSyntax, registry.Chroma, nil)
	}
	result, err := codeHighlightService.Highlight(ctx, backend.Request{Language: language, Source: source})
	if err != nil {
		return nil, fmt.Errorf("highlight %q: %w", codeSyntax, err)
	}
	return renderTokenLines(result.Tokens, styleName), nil
}

// highlightCached renders highlighted code, serving previously-computed results
// from the on-disk cache so re-renders (pane resize, extended scroll window,
// revisit) never re-run the highlighter. It returns the rendered lines window
// [start, start+count) and the total number of lines available. The complete
// highlighted result is written back to the cache on the first (cache-miss)
// render. When count <= 0 the whole file is returned.
func highlightCached(ctx context.Context, codeSyntax, source, styleName string, start, count int) ([]string, int, error) {
	language, ok := registry.LanguageForCodeSyntax(codeSyntax)
	if !ok {
		language = registry.Language(codeSyntax, codeSyntax, registry.Chroma, nil)
	}
	key := codeCacheService.key(source, language.CanonicalID, styleName)

	if content, ok := codeCacheService.get(key); ok {
		return codeWindow(content, start, count), lineCount(content), nil
	}

	result, err := codeHighlightService.Highlight(ctx, backend.Request{Language: language, Source: source})
	if err != nil {
		return nil, 0, fmt.Errorf("highlight %q: %w", codeSyntax, err)
	}
	content := renderTokenString(result.Tokens, styleName)
	if content != "" {
		codeCacheService.put(key, content)
	}
	return codeWindow(content, start, count), lineCount(content), nil
}

// renderTokenLines builds token output and splits it into individual lines.
func renderTokenLines(tokens []backend.Token, styleName string) []string {
	output := renderTokenString(tokens, styleName)
	if output == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(output, "\n"), "\n")
}

// renderTokenString builds the highlighted token output as a single string with
// lines separated by newlines and a trailing newline, without allocating a line
// slice. This lets huge files be cached and windowed without materializing every
// line.
func renderTokenString(tokens []backend.Token, styleName string) string {
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	var output strings.Builder
	for _, token := range tokens {
		writeHighlightToken(&output, token, style)
	}
	return output.String()
}

// codeWindow returns the lines [start, start+count) of a newline-delimited
// rendered block (an optional trailing newline terminates the final line rather
// than creating an empty extra line). It scans the block to isolate only the
// requested window, so a large cached file never needs its full line list
// materialized. count <= 0 returns every line.
func codeWindow(content string, start, count int) []string {
	if content == "" {
		return []string{}
	}
	total := lineCount(content)
	if start < 0 {
		start = 0
	}
	if start >= total {
		return []string{}
	}
	if count <= 0 {
		count = total - start
	} else if count > total-start {
		count = total - start
	}
	if count == 0 {
		return []string{}
	}

	window := make([]string, 0, count)
	i := 0
	lineIdx := 0
	for lineIdx < start+count && i < len(content) {
		end := strings.IndexByte(content[i:], '\n')
		var line string
		if end < 0 {
			line = content[i:]
			i = len(content)
		} else {
			line = content[i : i+end]
			i += end + 1
		}
		if lineIdx >= start {
			window = append(window, line)
		}
		lineIdx++
	}
	return window
}

// lineCount counts the lines in a newline-delimited block, where an optional
// trailing newline terminates the last line rather than adding an empty one.
func lineCount(content string) int {
	if content == "" {
		return 0
	}
	n := strings.Count(content, "\n")
	if strings.HasSuffix(content, "\n") {
		return n
	}
	return n + 1
}

func writeHighlightToken(output *strings.Builder, token backend.Token, style *chroma.Style) {
	if token.Value == "" {
		return
	}
	formatting := styleForToken(token, style)
	if formatting == "" {
		output.WriteString(token.Value)
		return
	}
	text := token.Value
	indices := codeNewlinePattern.FindAllStringIndex(text, -1)
	afterLastNewline := 0
	for _, match := range indices {
		start, end := match[0], match[1]
		output.WriteString(formatting)
		output.WriteString(text[afterLastNewline:start])
		output.WriteString("\x1b[0m")
		output.WriteString(text[start:end])
		afterLastNewline = end
	}
	if afterLastNewline < len(text) {
		output.WriteString(formatting)
		output.WriteString(text[afterLastNewline:])
		output.WriteString("\x1b[0m")
	}
}

func styleForToken(token backend.Token, style *chroma.Style) string {
	if isPlainToken(token) {
		return ""
	}
	tokenType, err := chroma.TokenTypeString(token.Kind)
	if err != nil {
		return ""
	}
	entry := style.Get(tokenType)
	if entry.IsZero() {
		return ""
	}
	var formatting strings.Builder
	if token.Bold || entry.Bold == chroma.Yes {
		formatting.WriteString("\x1b[1m")
	}
	if token.Italic || entry.Italic == chroma.Yes {
		formatting.WriteString("\x1b[3m")
	}
	if entry.Colour.IsSet() {
		fmt.Fprintf(&formatting, "\x1b[38;2;%d;%d;%dm",
			entry.Colour.Red(), entry.Colour.Green(), entry.Colour.Blue())
	}
	return formatting.String()
}

func isPlainToken(token backend.Token) bool {
	if token.Kind == "" || token.Kind == "Text" {
		return true
	}
	tokenType, err := chroma.TokenTypeString(token.Kind)
	if err != nil {
		return true
	}
	return tokenType == chroma.Text || tokenType.Category() == chroma.Text
}