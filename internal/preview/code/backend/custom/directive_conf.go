package custom

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// highlightDirectiveConfLine handles configs whose first word is a directive,
// including nginx, Apache, SSH, and similar daemon configuration files.
func highlightDirectiveConfLine(line string) []Token {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	indent := line[:len(line)-len(trimmed)]
	if trimmed == "" {
		return []Token{token(Text, line)}
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return []Token{token(Text, indent), styled(Comment, trimmed, false, true)}
	}
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		return []Token{token(Text, indent), styled(Type, trimmed, true, false)}
	}

	keyEnd := scanDirectiveKeyEnd(trimmed)
	if keyEnd == 0 {
		return highlightDirectiveValueFragment(line)
	}
	tokens := []Token{token(Text, indent), styled(Property, trimmed[:keyEnd], true, false)}
	index := keyEnd

	if end := scanWhitespace(trimmed, index); end > index {
		tokens = append(tokens, token(Text, trimmed[index:end]))
		index = end
	}
	if index < len(trimmed) && trimmed[index] == '=' {
		tokens = append(tokens, token(Operator, "="))
		index++
		if end := scanWhitespace(trimmed, index); end > index {
			tokens = append(tokens, token(Text, trimmed[index:end]))
			index = end
		}
	}
	if index < len(trimmed) {
		tokens = append(tokens, highlightDirectiveValueFragment(trimmed[index:])...)
	}
	return tokens
}

func scanDirectiveKeyEnd(input string) int {
	index := 0
	for index < len(input) {
		r, size := utf8.DecodeRuneInString(input[index:])
		if unicode.IsSpace(r) || strings.ContainsRune("=#;\"'", r) {
			break
		}
		index += size
	}
	return index
}

func highlightDirectiveValueFragment(input string) []Token {
	var tokens []Token
	for index := 0; index < len(input); {
		r, size := utf8.DecodeRuneInString(input[index:])
		if unicode.IsSpace(r) {
			end := scanWhitespace(input, index)
			tokens = append(tokens, token(Text, input[index:end]))
			index = end
			continue
		}
		if r == '"' || r == '\'' {
			end := scanQuotedSegment(input, index)
			tokens = append(tokens, token(String, input[index:end]))
			index = end
			continue
		}
		if end, ok := scanHexColor(input, index); ok {
			tokens = append(tokens, token(Number, input[index:end]))
			index = end
			continue
		}
		if isDirectiveCommentStart(input, index) {
			tokens = append(tokens, styled(Comment, input[index:], false, true))
			break
		}
		if strings.ContainsRune("[]{}(),:=", r) {
			tokens = append(tokens, token(Operator, input[index:index+size]))
			index += size
			continue
		}

		start := index
		index += size
		for index < len(input) {
			r, size = utf8.DecodeRuneInString(input[index:])
			if unicode.IsSpace(r) || strings.ContainsRune("[]{}(),:=\"'#", r) {
				break
			}
			index += size
		}
		tokens = append(tokens, directiveToken(input[start:index]))
	}
	return tokens
}

func directiveToken(value string) Token {
	switch {
	case isDirectiveKeyword(value):
		return token(Keyword, value)
	case looksNumeric(value):
		return token(Number, value)
	case looksPathLike(value):
		return token(String, value)
	default:
		return token(Text, value)
	}
}

func isDirectiveKeyword(value string) bool {
	switch strings.ToLower(value) {
	case "auto", "disabled", "enabled", "false", "inherit", "no", "none", "null", "off", "on", "true", "yes":
		return true
	default:
		return false
	}
}

func looksPathLike(value string) bool {
	return strings.HasPrefix(value, "~/") ||
		strings.HasPrefix(value, "./") ||
		strings.HasPrefix(value, "../") ||
		strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "file:")
}

func scanHexColor(input string, start int) (int, bool) {
	if start >= len(input) || input[start] != '#' {
		return 0, false
	}
	index := start + 1
	for index < len(input) && isHexDigit(input[index]) {
		index++
	}
	digits := index - start - 1
	if digits != 3 && digits != 4 && digits != 6 && digits != 8 {
		return 0, false
	}
	if index < len(input) {
		r, _ := utf8.DecodeRuneInString(input[index:])
		if !unicode.IsSpace(r) && !strings.ContainsRune(",;)]}", r) {
			return 0, false
		}
	}
	return index, true
}

func isHexDigit(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
}

func isDirectiveCommentStart(input string, index int) bool {
	if strings.HasPrefix(input[index:], "#") || strings.HasPrefix(input[index:], ";") {
		return true
	}
	if !strings.HasPrefix(input[index:], "//") {
		return false
	}
	if index == 0 {
		return true
	}
	previous, _ := utf8.DecodeLastRuneInString(input[:index])
	return previous != ':'
}

func scanWhitespace(input string, start int) int {
	index := start
	for index < len(input) {
		r, size := utf8.DecodeRuneInString(input[index:])
		if !unicode.IsSpace(r) {
			break
		}
		index += size
	}
	return index
}
