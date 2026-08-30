package preview

import (
	"bytes"
	"fmt"

	chroma "github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

func highlightSource(source, language, styleName string) (string, error) {
	lexer := lexers.Get(language)
	if lexer == nil {
		return source, nil
	}
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, source)
	if err != nil {
		return "", fmt.Errorf("tokenise %s: %w", language, err)
	}
	var output bytes.Buffer
	if err := formatters.TTY16m.Format(&output, style, iterator); err != nil {
		return "", fmt.Errorf("format %s: %w", language, err)
	}
	return output.String(), nil
}
