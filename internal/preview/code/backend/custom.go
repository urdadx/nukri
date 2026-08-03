package backend

import (
	"context"
	"fmt"

	custombackend "github.com/urdadx/nukri/internal/preview/code/backend/custom"
)

type builtinCustomHighlighter struct{}

func (builtinCustomHighlighter) Highlight(ctx context.Context, request Request) ([]Token, error) {
	tokens, supported, err := custombackend.Highlight(ctx, request.Language.CanonicalID, request.Source)
	if err != nil {
		return nil, err
	}
	if !supported {
		return nil, fmt.Errorf("%w for %q", ErrNoCustomHighlighter, request.Language.CanonicalID)
	}
	return customTokens(tokens), nil
}

func customTokens(tokens []custombackend.Token) []Token {
	result := make([]Token, 0, len(tokens))
	for _, token := range tokens {
		if token.Value == "" {
			continue
		}
		result = append(result, Token{
			Kind:   token.Kind,
			Value:  token.Value,
			Bold:   token.Bold,
			Italic: token.Italic,
		})
	}
	return result
}
