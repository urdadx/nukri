package test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/preview/terminalimage"
)

func TestSixelPlaceAndDelete(t *testing.T) {
	var output bytes.Buffer
	renderer := terminalimage.NewSixelWithSupport(&output, true)
	placed, err := renderer.Place(sixelTestImage(t), terminalimage.Placement{
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
	if !terminalimage.IsSixelTerminal() {
		t.Fatal("WezTerm should enable Sixel detection")
	}
	if _, ok := terminalimage.NewRenderer(&bytes.Buffer{}).(*terminalimage.Sixel); !ok {
		t.Fatal("renderer selection should choose Sixel")
	}

	t.Setenv("KITTY_WINDOW_ID", "1")
	if _, ok := terminalimage.NewRenderer(&bytes.Buffer{}).(*terminalimage.Kitty); !ok {
		t.Fatal("renderer selection should prefer Kitty")
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
