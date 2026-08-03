package custom

import (
	"context"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	Text      = "Text"
	Comment   = "Comment"
	Type      = "NameClass"
	Property  = "NameFunction"
	Parameter = "NameAttribute"
	Operator  = "Operator"
	String    = "LiteralString"
	Keyword   = "Keyword"
	Number    = "LiteralNumber"
	Error     = "Error"
)

// Token carries semantic styling for a future Lip Gloss renderer. Kind maps to
// a palette role; Bold and Italic map directly to lipgloss.Style modifiers.
type Token struct {
	Kind   string
	Value  string
	Bold   bool
	Italic bool
}

// Highlight handles data formats and the small set of registered languages
// without a Chroma lexer. The bool reports whether the language is supported.
func Highlight(ctx context.Context, languageID, source string) ([]Token, bool, error) {
	languageID = strings.ToLower(strings.TrimSpace(languageID))
	lineHighlighter, supported := highlighterFor(languageID)
	if !supported {
		return nil, false, nil
	}

	var tokens []Token
	inBlockComment := false
	for _, line := range strings.SplitAfter(source, "\n") {
		if err := ctx.Err(); err != nil {
			return nil, true, err
		}
		hasNewline := strings.HasSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\n")
		tokens = append(tokens, lineHighlighter(line, &inBlockComment)...)
		if hasNewline {
			tokens = append(tokens, Token{Kind: Text, Value: "\n"})
		}
	}
	return tokens, true, nil
}

type lineHighlighter func(string, *bool) []Token

func highlighterFor(languageID string) (lineHighlighter, bool) {
	switch languageID {
	case "toml":
		return func(line string, _ *bool) []Token { return highlightTOMLLine(line) }, true
	case "json":
		return func(line string, _ *bool) []Token { return highlightJSONLine(line) }, true
	case "jsonc", "json5":
		return highlightJSONCLine, true
	case "yaml":
		return func(line string, _ *bool) []Token { return highlightYAMLLine(line) }, true
	case "ini":
		return func(line string, _ *bool) []Token { return highlightINILine(line, false) }, true
	case "config":
		return func(line string, _ *bool) []Token { return highlightDirectiveConfLine(line) }, true
	case "dotenv":
		return func(line string, _ *bool) []Token { return highlightConfigLine(line) }, true
	case "log":
		return func(line string, _ *bool) []Token { return highlightLogLine(line) }, true
	case "less", "just":
		return func(line string, _ *bool) []Token { return highlightValueFragment(line) }, true
	default:
		return nil, false
	}
}

func highlightTOMLLine(line string) []Token {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	indent := line[:len(line)-len(trimmed)]
	if strings.HasPrefix(trimmed, "#") {
		return []Token{token(Text, indent), styled(Comment, trimmed, false, true)}
	}
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		return []Token{token(Text, indent), styled(Type, trimmed, true, false)}
	}
	if left, right, ok := splitUnquotedOnce(trimmed, '='); ok {
		tokens := []Token{
			token(Text, indent),
			styled(Property, strings.TrimRightFunc(left, unicode.IsSpace), true, false),
			token(Operator, " = "),
		}
		return append(tokens, highlightValueFragment(strings.TrimLeftFunc(right, unicode.IsSpace))...)
	}
	return highlightValueFragment(line)
}

func highlightJSONLine(line string) []Token {
	var tokens []Token
	for index := 0; index < len(line); {
		r, size := utf8.DecodeRuneInString(line[index:])
		if unicode.IsSpace(r) {
			start := index
			for index < len(line) {
				r, size = utf8.DecodeRuneInString(line[index:])
				if !unicode.IsSpace(r) {
					break
				}
				index += size
			}
			tokens = append(tokens, token(Text, line[start:index]))
			continue
		}
		if r == '"' || r == '\'' {
			end := scanQuotedSegment(line, index)
			kind := String
			if nextNonSpace(line[end:]) == ':' {
				kind = Property
			}
			tokens = append(tokens, token(kind, line[index:end]))
			index = end
			continue
		}
		if strings.ContainsRune("{}[]:,", r) {
			tokens = append(tokens, token(Operator, line[index:index+size]))
			index += size
			continue
		}
		start := index
		for index < len(line) {
			r, size = utf8.DecodeRuneInString(line[index:])
			if unicode.IsSpace(r) || strings.ContainsRune("{}[]:,", r) {
				break
			}
			index += size
		}
		tokens = append(tokens, scalarToken(line[start:index]))
	}
	return tokens
}

func highlightJSONCLine(line string, inBlockComment *bool) []Token {
	var tokens []Token
	for _, segment := range splitJSONCSegments(line, inBlockComment) {
		if segment.comment {
			tokens = append(tokens, styled(Comment, segment.text, false, true))
		} else {
			tokens = append(tokens, highlightJSONLine(segment.text)...)
		}
	}
	return tokens
}

func highlightYAMLLine(line string) []Token {
	body, comment := splitComment(line)
	trimmed := strings.TrimLeftFunc(body, unicode.IsSpace)
	indent := body[:len(body)-len(trimmed)]
	tokens := []Token{token(Text, indent)}
	content := trimmed
	if rest, ok := strings.CutPrefix(trimmed, "- "); ok {
		tokens = append(tokens, token(Operator, "- "))
		content = rest
	}
	if left, right, ok := splitUnquotedOnce(content, ':'); ok {
		tokens = append(tokens,
			styled(Property, strings.TrimRightFunc(left, unicode.IsSpace), true, false),
			token(Operator, ":"),
		)
		if right != "" {
			tokens = append(tokens, token(Text, " "))
			tokens = append(tokens, highlightTokenStream(strings.TrimLeftFunc(right, unicode.IsSpace))...)
		}
	} else {
		tokens = append(tokens, highlightTokenStream(content)...)
	}
	return appendComment(tokens, body, comment)
}

func highlightConfigLine(line string) []Token {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	indent := line[:len(line)-len(trimmed)]
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return []Token{token(Text, indent), styled(Comment, trimmed, false, true)}
	}
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		return []Token{token(Text, indent), styled(Type, trimmed, true, false)}
	}
	for _, separator := range []rune{'=', ':'} {
		if left, right, ok := splitUnquotedOnce(trimmed, separator); ok {
			tokens := []Token{token(Text, indent), styled(Property, strings.TrimSpace(left), true, false)}
			tokens = append(tokens, token(Operator, " "+string(separator)+" "))
			return append(tokens, highlightValueFragment(strings.TrimSpace(right))...)
		}
	}
	return highlightValueFragment(line)
}

func highlightValueFragment(value string) []Token {
	body, comment := splitComment(value)
	return appendComment(highlightTokenStream(body), body, comment)
}

func highlightTokenStream(input string) []Token {
	var tokens []Token
	for index := 0; index < len(input); {
		r, size := utf8.DecodeRuneInString(input[index:])
		if unicode.IsSpace(r) {
			start := index
			for index < len(input) {
				r, size = utf8.DecodeRuneInString(input[index:])
				if !unicode.IsSpace(r) {
					break
				}
				index += size
			}
			tokens = append(tokens, token(Text, input[start:index]))
			continue
		}
		if r == '"' || r == '\'' {
			end := scanQuotedSegment(input, index)
			tokens = append(tokens, token(String, input[index:end]))
			index = end
			continue
		}
		if strings.ContainsRune("[]{}(),:", r) {
			tokens = append(tokens, token(Operator, input[index:index+size]))
			index += size
			continue
		}
		start := index
		for index < len(input) {
			r, size = utf8.DecodeRuneInString(input[index:])
			if unicode.IsSpace(r) || strings.ContainsRune("[]{}(),:#\"'", r) {
				break
			}
			index += size
		}
		if start == index {
			index += size
			tokens = append(tokens, token(Text, input[start:index]))
			continue
		}
		tokens = append(tokens, scalarToken(input[start:index]))
	}
	return tokens
}

func scalarToken(value string) Token {
	trimmed := strings.TrimSpace(value)
	switch {
	case trimmed == "":
		return token(Text, value)
	case isKeyword(trimmed):
		return token(Keyword, value)
	case looksNumeric(trimmed):
		return token(Number, value)
	default:
		return token(Text, value)
	}
}

func isKeyword(value string) bool {
	switch strings.ToLower(value) {
	case "true", "false", "null", "nil", "none", "debug", "info", "warn", "warning", "error", "fatal", "panic":
		return true
	default:
		return false
	}
}

func looksNumeric(value string) bool {
	value = strings.ReplaceAll(value, "_", "")
	if _, err := strconv.ParseInt(value, 0, 64); err == nil {
		return true
	}
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

func scanQuotedSegment(input string, start int) int {
	quote, size := utf8.DecodeRuneInString(input[start:])
	escaped := false
	for index := start + size; index < len(input); {
		r, runeSize := utf8.DecodeRuneInString(input[index:])
		index += runeSize
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == quote {
			return index
		}
	}
	return len(input)
}

func splitUnquotedOnce(input string, separator rune) (string, string, bool) {
	var quote rune
	escaped := false
	for index, r := range input {
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' || r == '\'' {
			switch quote {
			case 0:
				quote = r
			case r:
				quote = 0
			}
			continue
		}
		if quote == 0 && r == separator {
			return input[:index], input[index+utf8.RuneLen(r):], true
		}
	}
	return "", "", false
}

func splitComment(input string) (string, string) {
	left, right, ok := splitUnquotedOnce(input, '#')
	if !ok {
		return input, ""
	}
	return strings.TrimRightFunc(left, unicode.IsSpace), "#" + right
}

type jsoncSegment struct {
	text    string
	comment bool
}

func splitJSONCSegments(line string, inBlockComment *bool) []jsoncSegment {
	var segments []jsoncSegment
	for index := 0; index < len(line); {
		if *inBlockComment {
			end := strings.Index(line[index:], "*/")
			if end < 0 {
				return append(segments, jsoncSegment{line[index:], true})
			}
			end += index + 2
			segments = append(segments, jsoncSegment{line[index:end], true})
			*inBlockComment = false
			index = end
			continue
		}
		start := index
		for index < len(line) {
			if line[index] == '"' || line[index] == '\'' {
				index = scanQuotedSegment(line, index)
				continue
			}
			if strings.HasPrefix(line[index:], "//") {
				if start < index {
					segments = append(segments, jsoncSegment{line[start:index], false})
				}
				return append(segments, jsoncSegment{line[index:], true})
			}
			if strings.HasPrefix(line[index:], "/*") {
				if start < index {
					segments = append(segments, jsoncSegment{line[start:index], false})
				}
				*inBlockComment = true
				break
			}
			_, size := utf8.DecodeRuneInString(line[index:])
			index += size
		}
		if !*inBlockComment {
			if start < len(line) {
				segments = append(segments, jsoncSegment{line[start:], false})
			}
			break
		}
	}
	return segments
}

func nextNonSpace(input string) rune {
	for _, r := range input {
		if !unicode.IsSpace(r) {
			return r
		}
	}
	return 0
}

func appendComment(tokens []Token, body, comment string) []Token {
	if comment == "" {
		return tokens
	}
	if body != "" {
		tokens = append(tokens, token(Text, " "))
	}
	return append(tokens, styled(Comment, comment, false, true))
}

func token(kind, value string) Token {
	return Token{Kind: kind, Value: value}
}

func styled(kind, value string, bold, italic bool) Token {
	return Token{Kind: kind, Value: value, Bold: bold, Italic: italic}
}
