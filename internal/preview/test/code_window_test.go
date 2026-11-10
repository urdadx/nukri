package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/fileinfo"
	"github.com/urdadx/nukri/internal/preview"
)

func TestBuildViewContextHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := preview.BuildViewContext(ctx, codePreview(), preview.ViewOptions{}); err != context.Canceled {
		t.Fatalf("BuildViewContext error = %v, want context.Canceled", err)
	}
}

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
	view, err := preview.BuildView(codePreview(), preview.ViewOptions{Width: 60, SyntaxStyle: "catppuccin-mocha", CodeWindow: 40})
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
	view, err := preview.BuildView(codePreview(), preview.ViewOptions{Width: 60, SyntaxStyle: "catppuccin-mocha", CodeWindow: 0})
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
	small, err := preview.BuildView(codePreview(), preview.ViewOptions{Width: 60, SyntaxStyle: "catppuccin-mocha", CodeWindow: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(small.Lines) != 50 {
		t.Fatalf("small window = %d lines, want 50", len(small.Lines))
	}

	wide, err := preview.BuildView(codePreview(), preview.ViewOptions{Width: 60, SyntaxStyle: "catppuccin-mocha", CodeWindow: 500})
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

// On a cache miss only the lines around the window are highlighted. They must
// match a whole-file highlight even when the window ends inside a multi-line
// comment or string, which the prefix would otherwise see as unterminated. The
// style must color comments and operators differently for a mismatch to show.
func TestCodeWindowHighlightMatchesWholeFile(t *testing.T) {
	sources := map[string]string{
		"edge.go": "/* block comment\nline two\nline three */\nvar s = `raw\nstring\nhere`\n",
		"edge.py": "x = \"\"\"doc\nstring\nlines\"\"\"\ndef f():\n    pass\n",
		"edge.js": "/* block\ncomment\nend */\nconst t = `tmpl\nliteral`;\n",
	}
	for name, construct := range sources {
		var source strings.Builder
		for i := range 38 {
			fmt.Fprintf(&source, "v%d = %d\n", i, i)
		}
		source.WriteString(construct)
		// Long enough that the window is a strict prefix of the file.
		for i := range 600 {
			fmt.Fprintf(&source, "w%d = %d\n", i, i)
		}
		// Windows ending just before, inside and after the construct. Each uses
		// unique content and renders before its whole-file reference, so neither
		// the cache nor the background highlight can answer for it.
		for window := 38; window <= 44; window++ {
			path := filepath.Join(t.TempDir(), name)
			content := source.String() + fmt.Sprintf("last = %d\n", window)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			value, err := preview.NewService().Render(context.Background(), preview.Request{Path: path, Facts: fileinfo.InspectPath(path, core.File), Width: 60})
			if err != nil {
				t.Fatal(err)
			}
			view, err := preview.BuildViewContext(context.Background(), value, preview.ViewOptions{Width: 56, SyntaxStyle: "catppuccin-mocha", CodeWindow: window})
			if err != nil {
				t.Fatal(err)
			}
			full, err := preview.BuildViewContext(context.Background(), value, preview.ViewOptions{Width: 56, SyntaxStyle: "catppuccin-mocha"})
			if err != nil {
				t.Fatal(err)
			}
			if view.TotalLines != full.TotalLines || len(view.Lines) != window {
				t.Fatalf("%s window %d: %d lines of %d, want %d of %d", name, window, len(view.Lines), view.TotalLines, window, full.TotalLines)
			}
			for line := range view.Lines {
				if view.Lines[line] != full.Lines[line] {
					t.Fatalf("%s window %d: line %d is %q, whole file gives %q", name, window, line, view.Lines[line], full.Lines[line])
				}
			}
		}
	}
}

// Markdown uses the same windowed highlighting as code: a long document returns
// only the requested lines plus the total, so the pane can extend on scroll.
func TestMarkdownPreviewIsWindowed(t *testing.T) {
	var source strings.Builder
	for i := range 300 {
		// Three lines per section, so line 800 (the limit) is not blank.
		fmt.Fprintf(&source, "## Section %d\nSome *emphasis* and `code`.\n\n", i)
	}
	path := filepath.Join(t.TempDir(), "long.md")
	if err := os.WriteFile(path, []byte(source.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := preview.NewService().Render(context.Background(), preview.Request{Path: path, Facts: fileinfo.InspectPath(path, core.File), Width: 60})
	if err != nil {
		t.Fatal(err)
	}
	view, err := preview.BuildViewContext(context.Background(), value, preview.ViewOptions{Width: 56, SyntaxStyle: "catppuccin-mocha", CodeWindow: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Lines) != 50 || view.TotalLines != 800 {
		t.Fatalf("got %d lines of %d, want 50 of 800 (the Markdown line limit)", len(view.Lines), view.TotalLines)
	}
	if !strings.Contains(view.Lines[0], "\x1b[") || !strings.Contains(view.Lines[0], "Section 0") {
		t.Fatalf("first line is not highlighted: %q", view.Lines[0])
	}
}
