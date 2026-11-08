package test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/core"
	fileinfo "github.com/urdadx/nukri/internal/file_info"
	"github.com/urdadx/nukri/internal/preview"
)

// Windows (\r\n) and old Mac (\r) line endings and tabs must not reach the
// pane: a carriage return sends the cursor back over the panels to the left.
func TestTextPreviewNormalizesLineEndingsAndTabs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	content := "\tTitle\r\n\r\nfirst\rsecond\r\nab\tc\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectPath(path, core.File)
	result, err := preview.NewService().Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: 60})
	if err != nil {
		t.Fatal(err)
	}
	text, ok := result.(*preview.TextPreview)
	if !ok {
		t.Fatalf("result = %T, want *preview.TextPreview", result)
	}
	if strings.ContainsAny(text.Text, "\r\t") {
		t.Fatalf("text still holds \\r or \\t: %q", text.Text)
	}
	// Tabs stop every 4 columns: "ab" + 2 spaces, a leading tab is 4 spaces.
	want := "    Title\n\nfirst\nsecond\nab  c\n"
	if text.Text != want {
		t.Fatalf("text = %q, want %q", text.Text, want)
	}
}
