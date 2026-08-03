package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/urdadx/nukri/internal/preview/code/registry"
)

var (
	ErrInvalidLanguage     = errors.New("invalid code language")
	ErrNoCustomHighlighter = errors.New("no custom highlighter registered")
	ErrUnsupportedBackend  = errors.New("unsupported code backend")
)

// Service dispatches registered languages to their configured highlighting backend.
// It is safe to highlight and register custom highlighters concurrently.
type Service struct {
	chroma Highlighter

	mu     sync.RWMutex
	custom map[string]Highlighter
}

func New() *Service {
	return &Service{
		chroma: newChromaHighlighter(),
		custom: make(map[string]Highlighter),
	}
}

func (s *Service) RegisterCustom(languageID string, highlighter Highlighter) error {
	languageID = normalizeLanguageID(languageID)
	if languageID == "" {
		return ErrInvalidLanguage
	}
	if highlighter == nil {
		return fmt.Errorf("register custom highlighter for %q: highlighter is nil", languageID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.custom[languageID] = highlighter
	return nil
}

func (s *Service) Highlight(ctx context.Context, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	languageID := normalizeLanguageID(request.Language.CanonicalID)
	if languageID == "" {
		return Result{}, ErrInvalidLanguage
	}
	request.Language.CanonicalID = languageID

	var highlighter Highlighter
	switch request.Language.Backend {
	case registry.Plain:
		return Result{
			Language: request.Language,
			Tokens:   plainTokens(request.Source),
		}, nil
	case registry.Chroma:
		highlighter = s.chroma
	case registry.Custom:
		s.mu.RLock()
		highlighter = s.custom[languageID]
		s.mu.RUnlock()
		if highlighter == nil {
			highlighter = builtinCustomHighlighter{}
		}
	default:
		return Result{}, fmt.Errorf("%w: %d", ErrUnsupportedBackend, request.Language.Backend)
	}

	tokens, err := highlighter.Highlight(ctx, request)
	if err != nil {
		return Result{}, fmt.Errorf("highlight %q: %w", languageID, err)
	}
	return Result{Language: request.Language, Tokens: tokens}, nil
}

func normalizeLanguageID(languageID string) string {
	return strings.ToLower(strings.TrimSpace(languageID))
}

func plainTokens(source string) []Token {
	if source == "" {
		return nil
	}
	return []Token{{Kind: "Text", Value: source}}
}
