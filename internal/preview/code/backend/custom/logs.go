package custom

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func highlightLogLine(line string) []Token {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	tokens := []Token{token(Text, line[:len(line)-len(trimmed)])}
	rest := trimmed

	if timestamp, remaining, ok := splitLogTimestamp(rest); ok {
		tokens = append(tokens, token(Comment, timestamp))
		rest = remaining
		if whitespace, remaining, ok := splitLeadingWhitespace(rest); ok {
			tokens = append(tokens, token(Text, whitespace))
			rest = remaining
		}
	}

	if level, remaining, ok := splitLogLevel(rest); ok {
		tokens = append(tokens, styled(logLevelKind(level), level, true, false))
		rest = remaining
		if whitespace, remaining, ok := splitLeadingWhitespace(rest); ok {
			tokens = append(tokens, token(Text, whitespace))
			rest = remaining
		}
	}

	return append(tokens, highlightLogMessage(rest)...)
}

func highlightLogMessage(line string) []Token {
	var tokens []Token
	plainStart := 0
	for start := 0; start < len(line); {
		end := start
		for end < len(line) {
			r, size := utf8.DecodeRuneInString(line[end:])
			end += size
			if unicode.IsSpace(r) {
				for end < len(line) {
					r, size = utf8.DecodeRuneInString(line[end:])
					if !unicode.IsSpace(r) {
						break
					}
					end += size
				}
				break
			}
		}

		part := line[start:end]
		word := strings.TrimRightFunc(part, unicode.IsSpace)
		suffix := part[len(word):]
		kind, styledWord := logWordKind(word)
		if styledWord {
			if plainStart < start {
				tokens = append(tokens, token(Text, line[plainStart:start]))
			}
			if left, right, ok := splitUnquotedOnce(word, '='); ok {
				tokens = append(tokens,
					styled(Parameter, left, true, false),
					token(Operator, "="),
				)
				tokens = append(tokens, highlightValueFragment(right)...)
			} else {
				tokens = append(tokens, token(kind, word))
			}
			if suffix != "" {
				tokens = append(tokens, token(Text, suffix))
			}
			plainStart = end
		}
		start = end
	}
	if plainStart < len(line) {
		tokens = append(tokens, token(Text, line[plainStart:]))
	}
	return tokens
}

func logWordKind(word string) (string, bool) {
	if _, _, ok := splitUnquotedOnce(word, '='); ok {
		return Parameter, true
	}
	trimmed := strings.Trim(word, "[](),;")
	if looksNumeric(trimmed) {
		return Number, true
	}
	if strings.HasPrefix(word, "[") && strings.HasSuffix(word, "]") {
		return Type, true
	}
	if strings.HasSuffix(word, ":") && len(word) > 1 {
		return Property, true
	}
	return Text, false
}

func splitLogTimestamp(input string) (string, string, bool) {
	end := 0
	separators := 0
	for index, r := range input {
		if unicode.IsDigit(r) || strings.ContainsRune("-:TZ.+/ ,", r) && r != ' ' {
			end = index + utf8.RuneLen(r)
			if strings.ContainsRune("-:T/", r) {
				separators++
			}
			continue
		}
		break
	}
	if end == 0 || separators < 2 {
		return "", input, false
	}
	return input[:end], input[end:], true
}

func splitLeadingWhitespace(input string) (string, string, bool) {
	end := 0
	for index, r := range input {
		if !unicode.IsSpace(r) {
			break
		}
		end = index + utf8.RuneLen(r)
	}
	if end == 0 {
		return "", input, false
	}
	return input[:end], input[end:], true
}

func splitLogLevel(input string) (string, string, bool) {
	trimmed := strings.TrimLeftFunc(input, unicode.IsSpace)
	offset := len(input) - len(trimmed)
	if trimmed == "" {
		return "", input, false
	}

	consumed := 0
	if trimmed[0] == '[' {
		end := strings.IndexByte(trimmed, ']')
		if end < 0 {
			return "", input, false
		}
		consumed = end + 1
	} else {
		consumed = len(trimmed)
		for index, r := range trimmed {
			if unicode.IsSpace(r) || strings.ContainsRune(":,;", r) {
				consumed = index
				break
			}
		}
	}

	level := trimmed[:consumed]
	normalized := strings.ToUpper(strings.Trim(level, "[]"))
	switch normalized {
	case "TRACE", "DEBUG", "INFO", "NOTICE", "WARN", "WARNING", "ERROR", "ERR", "FATAL":
		start := offset
		end := offset + consumed
		return input[start:end], input[end:], true
	default:
		return "", input, false
	}
}

func logLevelKind(level string) string {
	switch strings.ToUpper(strings.Trim(level, "[]")) {
	case "TRACE":
		return Comment
	case "DEBUG":
		return Number
	case "INFO", "NOTICE":
		return Property
	case "WARN", "WARNING":
		return Keyword
	case "ERROR", "ERR", "FATAL":
		return Error
	default:
		return Text
	}
}
