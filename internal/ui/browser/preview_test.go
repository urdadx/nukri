package browser

import (
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/preview"
)

// imageData is a selected picture's preview, for a terminal set up by the test.
func imageData() Data {
	return Data{
		Selected: 0,
		Entries:  []Entry{{Entry: core.Entry{Name: "photo.png", Kind: core.File}, Icon: "i"}},
		Preview: preview.View{
			Title: "Image", Detail: "640x480",
			Visual: &preview.Image{MediaType: "image/png", Data: []byte{1}, Width: 640, Height: 480},
		},
	}
}

func setTerminal(t *testing.T, term, program string) {
	t.Helper()
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("NUKRI_SIXEL", "")
	t.Setenv("TERM", term)
	t.Setenv("TERM_PROGRAM", program)
}

// A terminal without kitty graphics or sixel, such as GNOME Terminal, gets a
// notice in place of the picture, with the file's details still shown.
func TestPreviewSaysWhenImagesAreUnsupported(t *testing.T) {
	setTerminal(t, "xterm-256color", "")
	styles := Styles{PanelFG: "#ffffff", PanelBG: "#000000", PanelBorder: "#555555", Path: "#00aaff", Muted: "#777777"}
	visual := &VisualState{}
	view := RenderPreview(60, 24, 0, 0, visual, styles, imageData())
	for _, want := range []string{"can't show images", "kitty graphics", "NUKRI_SIXEL=1", "640x480"} {
		if !strings.Contains(view, want) {
			t.Fatalf("preview is missing %q:\n%s", want, view)
		}
	}
	if visual.want != nil {
		t.Fatal("no image should be queued for a terminal that can't draw it")
	}
}

func TestPreviewDrawsImagesWhereSupported(t *testing.T) {
	setTerminal(t, "xterm-kitty", "")
	styles := Styles{PanelFG: "#ffffff", PanelBG: "#000000", PanelBorder: "#555555", Path: "#00aaff", Muted: "#777777"}
	visual := &VisualState{}
	view := RenderPreview(60, 24, 0, 0, visual, styles, imageData())
	if strings.Contains(view, "can't show images") {
		t.Fatal("a kitty terminal should not get the notice")
	}
	if visual.want == nil {
		t.Fatal("the image should be queued for drawing")
	}
}
