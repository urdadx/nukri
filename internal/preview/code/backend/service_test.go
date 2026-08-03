package backend

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/preview/code/registry"
	"github.com/urdadx/nukri/internal/preview/code/registry/data"
)

func TestChromaHighlight(t *testing.T) {
	language, ok := registry.LanguageForCodeSyntax("go")
	if !ok {
		t.Fatal("Go is not registered")
	}

	result, err := New().Highlight(context.Background(), Request{
		Language: language,
		Source:   "package main\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	var source strings.Builder
	foundKeyword := false
	for _, token := range result.Tokens {
		source.WriteString(token.Value)
		foundKeyword = foundKeyword || token.Kind == "KeywordNamespace"
	}
	if source.String() != "package main\n" {
		t.Fatalf("token source = %q", source.String())
	}
	if !foundKeyword {
		t.Fatalf("tokens = %#v, want a package keyword", result.Tokens)
	}
}

func TestEveryRegisteredChromaLanguageCanHighlight(t *testing.T) {
	service := New()
	for _, entry := range data.AllLanguages() {
		if entry.Language.Backend != registry.Chroma {
			continue
		}
		t.Run(entry.Language.CanonicalID, func(t *testing.T) {
			_, err := service.Highlight(context.Background(), Request{Language: entry.Language})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEveryRegisteredCustomLanguageCanHighlight(t *testing.T) {
	service := New()
	for _, entry := range data.AllLanguages() {
		if entry.Language.Backend != registry.Custom {
			continue
		}
		t.Run(entry.Language.CanonicalID, func(t *testing.T) {
			_, err := service.Highlight(context.Background(), Request{Language: entry.Language})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUnsupportedChromaLanguageUsesCustomFallback(t *testing.T) {
	language, _ := registry.LanguageForCodeSyntax("log")
	result, err := New().Highlight(context.Background(), Request{Language: language, Source: "ERROR request failed"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasToken(result.Tokens, "Error", "ERROR") {
		t.Fatalf("tokens = %#v, want highlighted log level", result.Tokens)
	}
}

func TestCustomHighlighter(t *testing.T) {
	service := New()
	called := false
	err := service.RegisterCustom("JSON", HighlighterFunc(func(_ context.Context, request Request) ([]Token, error) {
		called = request.Language.StructuredFormat != nil
		return []Token{{Kind: "Property", Value: request.Source}}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	language, _ := registry.LanguageForCodeSyntax("json")
	result, err := service.Highlight(context.Background(), Request{Language: language, Source: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if !called || len(result.Tokens) != 1 || result.Tokens[0].Value != "{}" {
		t.Fatalf("result = %#v, called = %v", result, called)
	}
}

func TestBuiltinCustomHighlighter(t *testing.T) {
	language, _ := registry.LanguageForCodeSyntax("yaml")
	result, err := New().Highlight(context.Background(), Request{Language: language, Source: "key: value # note"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasToken(result.Tokens, "NameFunction", "key") || !hasToken(result.Tokens, "Comment", "# note") {
		t.Fatalf("tokens = %#v", result.Tokens)
	}
}

func TestMissingCustomHighlighter(t *testing.T) {
	language := registry.Language("unknown", "Unknown", registry.Custom, nil)
	_, err := New().Highlight(context.Background(), Request{Language: language, Source: "value"})
	if !errors.Is(err, ErrNoCustomHighlighter) {
		t.Fatalf("error = %v, want ErrNoCustomHighlighter", err)
	}
}

func TestHighlightHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	language, _ := registry.LanguageForCodeSyntax("go")
	_, err := New().Highlight(ctx, Request{Language: language, Source: "package main"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func hasToken(tokens []Token, kind, value string) bool {
	for _, token := range tokens {
		if token.Kind == kind && token.Value == value {
			return true
		}
	}
	return false
}
