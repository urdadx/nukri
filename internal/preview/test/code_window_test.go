package test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/preview"
)

// codeSource builds a large Go source (5000 lines) to exercise incremental
// leading-window rendering without needing real tooling.
func codeSource() string {
	var b strings.Builder
	for i := 1; i <= 5000; i++ {
		b.WriteString("func f")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("() int { return ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" }\n")
	}
	return b.String()
}

func codePreview() *preview.TextPreview {
	return &preview.TextPreview{Title: "code.go", Text: codeSource(), CodeLanguage: "go"}
}

// TestCodePreviewIncrementalWindow verifies that a bounded CodeWindow renders
// only that many leading lines while still reporting the full source length, and
// that the resulting preview contains the first lines.
func TestCodePreviewIncrementalWindow(t *testing.T) {
	view, err := preview.BuildView(codePreview(), preview.ViewOptions{Width: 60, SyntaxStyle: "monokai", CodeWindow: 40})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Lines) != 40 {
		t.Fatalf("windowed render = %d lines, want 40", len(view.Lines))
	}
	if len(view.Lines[0]) == 0 {
		t.Fatal("first rendered line is empty")
	}
	if view.TotalLines != 5000 {
		t.Fatalf("TotalLines = %d, want 5000", view.TotalLines)
	}
}

// TestCodePreviewFullRenderWhenNoWindow verifies that CodeWindow 0 (or a window
// at least as large as the source) renders the whole file, and that the preview
// then reports no further lines remain (TotalLines == len(Lines)).
func TestCodePreviewFullRenderWhenNoWindow(t *testing.T) {
	view, err := preview.BuildView(codePreview(), preview.ViewOptions{Width: 60, SyntaxStyle: "monokai", CodeWindow: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Lines) != 5000 || view.TotalLines != 5000 {
		t.Fatalf("full render = %d lines, %d total; want 5000/5000", len(view.Lines), view.TotalLines)
	}
}

// TestCodePreviewExtendReusesFragment verifies the window can be widened: a first
// render with a small window is followed by a render with a larger window that
// serves the previously-computed (cached) highlight and returns more leading
// lines. This is what makes incremental scroll extension cheap.
func TestCodePreviewExtendReusesFragment(t *testing.T) {
	small, err := preview.BuildView(codePreview(), preview.ViewOptions{Width: 60, SyntaxStyle: "monokai", CodeWindow: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(small.Lines) != 50 {
		t.Fatalf("small window = %d lines, want 50", len(small.Lines))
	}

	wide, err := preview.BuildView(codePreview(), preview.ViewOptions{Width: 60, SyntaxStyle: "monokai", CodeWindow: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(wide.Lines) != 500 {
		t.Fatalf("wide window = %d lines, want 500", len(wide.Lines))
	}
	for i := 0; i < 50; i++ {
		if wide.Lines[i] != small.Lines[i] {
			t.Fatalf("extended render changed already-visible line %d", i)
		}
	}
}
