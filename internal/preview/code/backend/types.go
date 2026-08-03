package backend

import (
	"context"

	"github.com/urdadx/nukri/internal/preview/code/registry"
)

// Token is backend-neutral highlighted source. Kind is a semantic token name,
// such as "Keyword" or "LiteralString", and Value is the original source text.
type Token struct {
	Kind   string
	Value  string
	Bold   bool
	Italic bool
}

type Request struct {
	Language registry.RegisteredLanguage
	Source   string
}

type Result struct {
	Language registry.RegisteredLanguage
	Tokens   []Token
}

// Highlighter is implemented by Chroma and by language-specific custom backends.
type Highlighter interface {
	Highlight(context.Context, Request) ([]Token, error)
}

type HighlighterFunc func(context.Context, Request) ([]Token, error)

func (f HighlighterFunc) Highlight(ctx context.Context, request Request) ([]Token, error) {
	return f(ctx, request)
}
