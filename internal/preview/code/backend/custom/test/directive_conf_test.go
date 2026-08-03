package test

import (
	"context"
	"strings"
	"testing"

	. "github.com/urdadx/nukri/internal/preview/code/backend/custom"
)

func TestHighlightDirectiveConfLine(t *testing.T) {
	line := "  listen = 8080 enabled /srv/www # public"
	tokens, _, err := Highlight(context.Background(), "config", line)
	if err != nil {
		t.Fatal(err)
	}
	wants := []struct {
		kind  string
		value string
		bold  bool
	}{
		{Property, "listen", true},
		{Operator, "=", false},
		{Number, "8080", false},
		{Keyword, "enabled", false},
		{String, "/srv/www", false},
		{Comment, "# public", false},
	}
	for _, want := range wants {
		if !hasDirectiveToken(tokens, want.kind, want.value, want.bold) {
			t.Errorf("missing token %#v in %#v", want, tokens)
		}
	}

	var source strings.Builder
	for _, token := range tokens {
		source.WriteString(token.Value)
	}
	if source.String() != line {
		t.Fatalf("token source = %q, want %q", source.String(), line)
	}
}

func TestDirectiveHexColorAndURL(t *testing.T) {
	tokens, _, err := Highlight(context.Background(), "config", "theme #ff00aa http://localhost // note")
	if err != nil {
		t.Fatal(err)
	}
	if !hasDirectiveToken(tokens, Number, "#ff00aa", false) {
		t.Fatalf("hex color not highlighted: %#v", tokens)
	}
	if !hasDirectiveToken(tokens, Comment, "// note", false) {
		t.Fatalf("comment not highlighted: %#v", tokens)
	}
	for _, token := range tokens {
		if token.Kind == Comment && strings.HasPrefix(token.Value, "//localhost") {
			t.Fatalf("URL was treated as a comment: %#v", tokens)
		}
	}
}

func TestDirectiveSectionAndComment(t *testing.T) {
	section, _, err := Highlight(context.Background(), "config", "[service]")
	if err != nil {
		t.Fatal(err)
	}
	if !hasDirectiveToken(section, Type, "[service]", true) {
		t.Fatalf("section tokens = %#v", section)
	}
	comment, _, err := Highlight(context.Background(), "config", "  ; disabled")
	if err != nil {
		t.Fatal(err)
	}
	if len(comment) != 2 || comment[1].Kind != Comment || !comment[1].Italic {
		t.Fatalf("comment tokens = %#v", comment)
	}
}

func hasDirectiveToken(tokens []Token, kind, value string, bold bool) bool {
	for _, token := range tokens {
		if token.Kind == kind && token.Value == value && token.Bold == bold {
			return true
		}
	}
	return false
}
