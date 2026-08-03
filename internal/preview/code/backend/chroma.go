package backend

import (
	"context"

	chroma "github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	custombackend "github.com/urdadx/nukri/internal/preview/code/backend/custom"
)

type chromaHighlighter struct{}

func newChromaHighlighter() Highlighter {
	return chromaHighlighter{}
}

func (chromaHighlighter) Highlight(ctx context.Context, request Request) ([]Token, error) {
	lexer := lexers.Get(request.Language.CanonicalID)
	if lexer == nil {
		if tokens, supported, err := custombackend.Highlight(ctx, request.Language.CanonicalID, request.Source); supported {
			return customTokens(tokens), err
		}
		lexer = lexers.Fallback
	}

	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, request.Source)
	if err != nil {
		return nil, err
	}

	var tokens []Token
	for token := iterator(); token != chroma.EOF; token = iterator() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if token.Value == "" {
			continue
		}
		tokens = append(tokens, Token{
			Kind:  token.Type.String(),
			Value: token.Value,
		})
	}
	return tokens, nil
}
