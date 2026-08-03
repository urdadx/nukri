package test

import (
	"context"
	"strings"
	"testing"

	. "github.com/urdadx/nukri/internal/preview/code/backend/custom"
)

func TestDataHighlightersPreserveSource(t *testing.T) {
	tests := []struct {
		language string
		source   string
	}{
		{"toml", "[server]\nport = 8080 # local\n"},
		{"json", "{\"enabled\": true, \"count\": 3}\n"},
		{"jsonc", "{/* open\nstill */ \"key\": null // end\n}\n"},
		{"yaml", "items:\n  - name: value # note\n"},
		{"ini", "[section]\nkey = value ; retained\n"},
	}

	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			tokens, supported, err := Highlight(context.Background(), test.language, test.source)
			if err != nil {
				t.Fatal(err)
			}
			if !supported {
				t.Fatal("language is not supported")
			}
			var source strings.Builder
			for _, token := range tokens {
				source.WriteString(token.Value)
			}
			if source.String() != test.source {
				t.Fatalf("token source = %q, want %q", source.String(), test.source)
			}
		})
	}
}

func TestJSONCSupportsMultilineComments(t *testing.T) {
	tokens, _, err := Highlight(context.Background(), "jsonc", "{\n/* first\nsecond */\n\"key\": false\n}")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(tokens, Comment, "/* first") || !contains(tokens, Comment, "second */") {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestSemanticModifiers(t *testing.T) {
	tokens, _, _ := Highlight(context.Background(), "toml", "# note\n[table]\nkey = 1")
	if !containsModified(tokens, Comment, true) {
		t.Fatalf("comment is not italic: %#v", tokens)
	}
	if !containsBold(tokens, Type) || !containsBold(tokens, Property) {
		t.Fatalf("table or property is not bold: %#v", tokens)
	}
}

func contains(tokens []Token, kind, value string) bool {
	for _, token := range tokens {
		if token.Kind == kind && token.Value == value {
			return true
		}
	}
	return false
}

func containsModified(tokens []Token, kind string, italic bool) bool {
	for _, token := range tokens {
		if token.Kind == kind && token.Italic == italic {
			return true
		}
	}
	return false
}

func containsBold(tokens []Token, kind string) bool {
	for _, token := range tokens {
		if token.Kind == kind && token.Bold {
			return true
		}
	}
	return false
}
