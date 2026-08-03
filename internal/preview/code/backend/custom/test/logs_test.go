package test

import (
	"context"
	"strings"
	"testing"

	. "github.com/urdadx/nukri/internal/preview/code/backend/custom"
)

func TestHighlightLogLine(t *testing.T) {
	line := "  2026-08-03T12:30:45Z ERROR request: status=500 [worker]"
	tokens, _, err := Highlight(context.Background(), "log", line)
	if err != nil {
		t.Fatal(err)
	}

	wants := []struct {
		kind  string
		value string
		bold  bool
	}{
		{Comment, "2026-08-03T12:30:45Z", false},
		{Error, "ERROR", true},
		{Property, "request:", false},
		{Parameter, "status", true},
		{Number, "500", false},
		{Type, "[worker]", false},
	}
	for _, want := range wants {
		found := false
		for _, token := range tokens {
			if token.Kind == want.kind && token.Value == want.value && token.Bold == want.bold {
				found = true
				break
			}
		}
		if !found {
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

func TestBracketedLogLevels(t *testing.T) {
	tokens, _, err := Highlight(context.Background(), "log", "[WARN] disk nearly full")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) < 2 || tokens[1].Kind != Keyword || !tokens[1].Bold {
		t.Fatalf("tokens = %#v", tokens)
	}
}
