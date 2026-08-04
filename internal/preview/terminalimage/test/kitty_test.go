package test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/preview/terminalimage"
)

func TestKittyPlaceChunksAndPositionsPNG(t *testing.T) {
	var output bytes.Buffer
	renderer := terminalimage.NewKittyWithSupport(&output, true)
	image := preview.Image{MediaType: "image/png", Data: bytes.Repeat([]byte{0xab}, 4_000), Width: 100, Height: 50}
	placed, err := renderer.Place(image, terminalimage.Placement{X: 4, Y: 2, Columns: 20, Rows: 10, ZIndex: -1})
	if err != nil {
		t.Fatal(err)
	}
	if placed.ImageID != 1 || placed.PlacementID != 2 || placed.Columns != 20 || placed.Rows != 10 {
		t.Fatalf("placed image = %#v", placed)
	}
	sequence := output.String()
	for _, value := range []string{
		"\x1b_Ga=t,f=100,i=1,q=2,t=d,m=1;",
		"\x1b_Gm=0;",
		"\x1b7\x1b[3;5H",
		"\x1b_Ga=p,c=20,i=1,p=2,q=2,r=10,z=-1;",
		"\x1b8",
	} {
		if !strings.Contains(sequence, value) {
			t.Errorf("protocol does not contain %q", value)
		}
	}
	if strings.Count(sequence, "\x1b_G") != 3 {
		t.Fatalf("graphics command count = %d, want 3", strings.Count(sequence, "\x1b_G"))
	}
}

func TestKittyDeleteLifecycle(t *testing.T) {
	var output bytes.Buffer
	renderer := terminalimage.NewKittyWithSupport(&output, true)
	placed, err := renderer.Place(
		preview.Image{MediaType: "image/png", Data: []byte("png")},
		terminalimage.Placement{Columns: 1, Rows: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := renderer.DeletePlacement(placed); err != nil {
		t.Fatal(err)
	}
	if err := renderer.Delete(placed); err != nil {
		t.Fatal(err)
	}
	if err := renderer.DeleteAll(); err != nil {
		t.Fatal(err)
	}
	sequence := output.String()
	if !strings.Contains(sequence, "a=d,d=p,i=1,p=2,q=2") ||
		!strings.Contains(sequence, "a=d,d=I,i=1,q=2") || !strings.Contains(sequence, "a=d,d=A,q=2") {
		t.Fatalf("delete protocol = %q", sequence)
	}
}

func TestKittyValidation(t *testing.T) {
	renderer := terminalimage.NewKittyWithSupport(&bytes.Buffer{}, false)
	_, err := renderer.Place(preview.Image{MediaType: "image/png", Data: []byte("png")}, terminalimage.Placement{Columns: 1, Rows: 1})
	if !errors.Is(err, terminalimage.ErrUnsupportedTerminal) {
		t.Fatalf("error = %v, want unsupported terminal", err)
	}

	renderer = terminalimage.NewKittyWithSupport(&bytes.Buffer{}, true)
	_, err = renderer.Place(preview.Image{MediaType: "image/jpeg", Data: []byte("jpg")}, terminalimage.Placement{Columns: 1, Rows: 1})
	if !errors.Is(err, terminalimage.ErrInvalidImage) {
		t.Fatalf("error = %v, want invalid image", err)
	}
	_, err = renderer.Place(preview.Image{MediaType: "image/png", Data: []byte("png")}, terminalimage.Placement{})
	if !errors.Is(err, terminalimage.ErrInvalidPlacement) {
		t.Fatalf("error = %v, want invalid placement", err)
	}
}

func TestIsKittyTerminal(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "1")
	t.Setenv("TERM", "xterm-256color")
	if !terminalimage.IsKittyTerminal() {
		t.Fatal("KITTY_WINDOW_ID should identify Kitty")
	}
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM", "xterm-kitty")
	if !terminalimage.IsKittyTerminal() {
		t.Fatal("TERM=xterm-kitty should identify Kitty")
	}
}
