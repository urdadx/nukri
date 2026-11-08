package preview

import (
	"context"
	"fmt"
	"strings"
	"sync"

	chroma "github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/urdadx/nukri/internal/preview/code/backend"
	"github.com/urdadx/nukri/internal/preview/code/registry"
)

// codeHighlightService dispatches registered languages to their highlighting
// backend (Chroma or the built-in custom tokenizers).
var codeHighlightService = backend.New()

/*
highlightCached returns the highlighted lines [start, start+count) of source and
the total number of lines; count <= 0 means every line.

A complete highlighted copy is kept in the disk cache, so revisits, resizes and
scrolling never re-run the highlighter. On a miss only the lines up to the
window (plus highlightLookahead) are highlighted: lexing runs top to bottom,
so those lines come out as they would inside the whole file, and large files
appear in a fraction of the time. The full file is then highlighted in the background to fill the
cache.
*/
func highlightCached(ctx context.Context, codeSyntax, source, styleName string, start, count int) ([]string, int, error) {
	language, ok := registry.LanguageForCodeSyntax(codeSyntax)
	// use chroma lexer for unknown languages so we can still highlight them with the fallback lexer.
	if !ok {
		language = registry.Language(codeSyntax, codeSyntax, registry.Chroma, nil)
	}
	key := codeCacheService.key(source, language.CanonicalID, styleName)

	if content, ok := codeCacheService.get(key); ok {
		return codeWindow(content, start, count), lineCount(content), nil
	}

	prefix := source
	if count > 0 {
		prefix = firstLines(source, start+count+highlightLookahead)
	}
	if len(prefix) < len(source) {
		content, err := highlightLanguage(ctx, language, prefix, styleName)
		if err != nil {
			return nil, 0, fmt.Errorf("highlight %q: %w", codeSyntax, err)
		}
		backgroundHighlights.schedule(key, func() {
			if content, err := highlightLanguage(context.Background(), language, source, styleName); err == nil && content != "" {
				codeCacheService.put(key, content)
			}
		})
		return codeWindow(content, start, count), lineCount(source), nil
	}

	content, err := highlightLanguage(ctx, language, source, styleName)
	if err != nil {
		return nil, 0, fmt.Errorf("highlight %q: %w", codeSyntax, err)
	}
	if content != "" {
		codeCacheService.put(key, content)
	}
	return codeWindow(content, start, count), lineCount(content), nil
}

/*
highlightLookahead is how many lines past the window are lexed on a cache miss.
A construct cut off by the end of the prefix, like a block comment or a
multi-line string, would be lexed as unterminated and colored wrongly; lexing
further lets anything opened inside the window close first. A construct longer
than this is only miscolored until the background highlight replaces it.
*/
const highlightLookahead = 200

func highlightLanguage(ctx context.Context, language registry.RegisteredLanguage, source, styleName string) (string, error) {
	result, err := codeHighlightService.Highlight(ctx, backend.Request{Language: language, Source: source})
	if err != nil {
		return "", err
	}
	return renderTokenString(result.Tokens, styleName), nil
}

// firstLines returns source up to and including its nth newline, or all of it
// when it has fewer lines.
func firstLines(source string, n int) string {
	end := 0
	for range n {
		next := strings.IndexByte(source[end:], '\n')
		if next < 0 {
			return source
		}
		end += next + 1
	}
	return source[:end]
}

/*
highlightQueue runs full-file highlights in the background, one at a time. Only
the newest request waits: while browsing quickly, a file passed over is not
worth highlighting, and it is redone on demand if revisited.
*/
type highlightQueue struct {
	mu      sync.Mutex
	running bool
	pending func()
	keys    map[string]bool // queued or running, to skip duplicates
	pendKey string
}

var backgroundHighlights = &highlightQueue{keys: map[string]bool{}}

func (q *highlightQueue) schedule(key string, job func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.keys[key] {
		return
	}
	if q.pending != nil {
		delete(q.keys, q.pendKey)
	}
	q.keys[key] = true
	q.pending, q.pendKey = job, key
	if !q.running {
		q.running = true
		go q.run()
	}
}

func (q *highlightQueue) run() {
	for {
		q.mu.Lock()
		job, key := q.pending, q.pendKey
		q.pending, q.pendKey = nil, ""
		if job == nil {
			q.running = false
			q.mu.Unlock()
			return
		}
		q.mu.Unlock()
		job()
		q.mu.Lock()
		delete(q.keys, key)
		q.mu.Unlock()
	}
}

/*
renderTokenString builds the highlighted token output as a single string with
lines separated by newlines and a trailing newline, without allocating a line
slice. This lets huge files be cached and windowed without materializing every
line.
*/
func renderTokenString(tokens []backend.Token, styleName string) string {
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	// Formatting depends only on the token kind and emphasis, so it is built
	// once per combination rather than once per token.
	formats := map[tokenFormatKey]string{}
	var output strings.Builder
	for _, token := range tokens {
		if token.Value == "" {
			continue
		}
		formatKey := tokenFormatKey{token.Kind, token.Bold, token.Italic}
		formatting, ok := formats[formatKey]
		if !ok {
			formatting = styleForToken(token, style)
			formats[formatKey] = formatting
		}
		writeHighlightToken(&output, token.Value, formatting)
	}
	return output.String()
}

type tokenFormatKey struct {
	kind         string
	bold, italic bool
}

/*
codeWindow returns the lines [start, start+count) of a newline-delimited
rendered block (an optional trailing newline terminates the final line rather
than creating an empty extra line). It scans the block to isolate only the
requested window, so a large cached file never needs its full line list
materialized. count <= 0 returns every line.
*/
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

// writeHighlightToken writes text wrapped in formatting, closing and reopening
// the formatting around each line break so every line stands alone.
func writeHighlightToken(output *strings.Builder, text, formatting string) {
	if formatting == "" {
		output.WriteString(text)
		return
	}
	for text != "" {
		line, rest, found := strings.Cut(text, "\n")
		lineBreak := "\n"
		if strings.HasSuffix(line, "\r") && found {
			line, lineBreak = line[:len(line)-1], "\r\n"
		}
		if line != "" {
			output.WriteString(formatting)
			output.WriteString(line)
			output.WriteString("\x1b[0m")
		}
		if !found {
			return
		}
		output.WriteString(lineBreak)
		text = rest
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
