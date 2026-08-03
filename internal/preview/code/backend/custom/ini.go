package custom

import (
	"strings"
	"unicode"
)

func highlightINILine(line string, desktopEntryMode bool) []Token {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	indent := line[:len(line)-len(trimmed)]

	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return []Token{
			token(Text, indent),
			styled(Comment, trimmed, false, true),
		}
	}

	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		kind := Type
		if desktopEntryMode && trimmed == "[Desktop Entry]" {
			kind = Keyword
		}
		return []Token{
			token(Text, indent),
			styled(kind, trimmed, true, false),
		}
	}

	if left, right, ok := splitUnquotedOnce(trimmed, '='); ok {
		key := strings.TrimRightFunc(left, unicode.IsSpace)
		spacing := left[len(key):]
		kind := Parameter
		if desktopEntryMode && isDesktopEntryKey(key) {
			kind = Property
		}
		tokens := []Token{
			token(Text, indent),
			styled(kind, key, true, false),
			token(Text, spacing),
			token(Operator, "="),
		}
		if right != "" {
			tokens = append(tokens, highlightValueFragment(right)...)
		}
		return tokens
	}

	return highlightValueFragment(line)
}

func isDesktopEntryKey(key string) bool {
	switch key {
	case "Name", "Exec", "Icon", "Type", "Terminal", "Categories":
		return true
	default:
		return false
	}
}
