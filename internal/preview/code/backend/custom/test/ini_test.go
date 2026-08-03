package test

import (
	"context"
	"testing"

	. "github.com/urdadx/nukri/internal/preview/code/backend/custom"
)

func TestHighlightINILine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantKind   string
		wantValue  string
		wantBold   bool
		wantItalic bool
	}{
		{"comment", "  ; note", Comment, "; note", false, true},
		{"section", "[server]", Type, "[server]", true, false},
		{"key", "port=8080", Parameter, "port", true, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, _, err := Highlight(context.Background(), "ini", test.line)
			if err != nil {
				t.Fatal(err)
			}
			for _, token := range tokens {
				if token.Kind == test.wantKind && token.Value == test.wantValue &&
					token.Bold == test.wantBold && token.Italic == test.wantItalic {
					return
				}
			}
			t.Fatalf("tokens = %#v", tokens)
		})
	}
}
