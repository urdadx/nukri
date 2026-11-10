package test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/preview/imageproto"
)

func TestSixelPlaceAndDelete(t *testing.T) {
	var output bytes.Buffer
	renderer := imageproto.NewSixelWithSupport(&output, true)
	placed, err := renderer.Place(sixelTestImage(t), imageproto.Placement{
		X: 3, Y: 2, Columns: 8, Rows: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if placed.ImageID != 1 || placed.PlacementID != 1 || placed.X != 3 || placed.Y != 2 {
		t.Fatalf("placed image = %#v", placed)
	}
	sequence := output.String()
	if !strings.Contains(sequence, "\x1b7\x1b[3;4H") || !strings.Contains(sequence, "\x1bP") || !strings.Contains(sequence, "\x1b8") {
		t.Fatalf("Sixel protocol = %q", sequence)
	}

	output.Reset()
	if err := renderer.DeletePlacement(placed); err != nil {
		t.Fatal(err)
	}
	cleared := output.String()
	if !strings.Contains(cleared, "\x1b[3;4H        ") || !strings.Contains(cleared, "\x1b[6;4H        ") {
		t.Fatalf("clear protocol = %q", cleared)
	}
}

func TestSixelDetectionAndRendererSelection(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM_PROGRAM", "WezTerm")
	t.Setenv("TERM", "xterm-256color")
	if !imageproto.IsSixelTerminal() {
		t.Fatal("WezTerm should enable Sixel detection")
	}
	if _, ok := imageproto.NewRenderer(&bytes.Buffer{}).(*imageproto.Sixel); !ok {
		t.Fatal("renderer selection should choose Sixel")
	}

	t.Setenv("KITTY_WINDOW_ID", "1")
	if _, ok := imageproto.NewRenderer(&bytes.Buffer{}).(*imageproto.Kitty); !ok {
		t.Fatal("renderer selection should prefer Kitty")
	}
}

// foot draws sixel and says so only through TERM (foot or foot-extra).
func TestSixelDetectsFoot(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("NUKRI_SIXEL", "")
	t.Setenv("TERM_PROGRAM", "")
	for _, term := range []string{"foot", "foot-extra"} {
		t.Setenv("TERM", term)
		if !imageproto.IsSixelTerminal() {
			t.Fatalf("TERM=%s should enable Sixel detection", term)
		}
	}
	t.Setenv("TERM", "xterm-256color")
	if imageproto.IsSixelTerminal() {
		t.Fatal("a plain xterm-256color terminal should not be taken for Sixel")
	}
}

func sixelTestImage(t *testing.T) preview.Image {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, 2, 2))
	value.Set(0, 0, color.RGBA{R: 255, A: 255})
	value.Set(1, 0, color.RGBA{G: 255, A: 255})
	value.Set(0, 1, color.RGBA{B: 255, A: 255})
	value.Set(1, 1, color.White)
	var data bytes.Buffer
	if err := png.Encode(&data, value); err != nil {
		t.Fatal(err)
	}
	return preview.Image{MediaType: "image/png", Data: data.Bytes(), Width: 2, Height: 2}
}

// A large image must be scaled into the placement box. The Sixel encoder's own
// size limits only crop, which would cut the image off.
func TestSixelScalesLargeImageToPlacement(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 1600, 1000))
	for y := range 1000 {
		for x := range 1600 {
			source.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, source); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	renderer := imageproto.NewSixelWithSupport(&output, true)
	// 10x5 cells of 8x16 pixels is an 80x80 box, so 1600x1000 scales to 80x50.
	placed, err := renderer.Place(preview.Image{MediaType: "image/png", Data: data.Bytes()}, imageproto.Placement{Columns: 10, Rows: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "\"1;1;80;50") {
		t.Fatalf("raster attributes = %q, want 80x50", output.String()[:min(40, output.Len())])
	}

	output.Reset()
	if _, err := renderer.Move(placed, imageproto.Placement{X: 4, Y: 2, Columns: 10, Rows: 5}); err != nil {
		t.Fatal(err)
	}
	moved := output.String()
	if !strings.Contains(moved, "\x1b[1;1H          ") || !strings.Contains(moved, "\x1b[3;5H\x1bP") {
		t.Fatalf("move should clear the old area and redraw at the new one: %q", moved[:min(120, len(moved))])
	}
}
