package test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/preview/imageproto"
)

func TestKittyPlaceChunksAndPositionsPNG(t *testing.T) {
	var output bytes.Buffer
	renderer := imageproto.NewKittyWithSupport(&output, true)
	image := preview.Image{MediaType: "image/png", Data: bytes.Repeat([]byte{0xab}, 4_000), Width: 100, Height: 50}
	placed, err := renderer.Place(image, imageproto.Placement{X: 4, Y: 2, Columns: 20, Rows: 10, ZIndex: -1})
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
	renderer := imageproto.NewKittyWithSupport(&output, true)
	placed, err := renderer.Place(
		preview.Image{MediaType: "image/png", Data: []byte("png")},
		imageproto.Placement{Columns: 1, Rows: 1},
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
	renderer := imageproto.NewKittyWithSupport(&bytes.Buffer{}, false)
	_, err := renderer.Place(preview.Image{MediaType: "image/png", Data: []byte("png")}, imageproto.Placement{Columns: 1, Rows: 1})
	if !errors.Is(err, imageproto.ErrUnsupportedTerminal) {
		t.Fatalf("error = %v, want unsupported terminal", err)
	}

	renderer = imageproto.NewKittyWithSupport(&bytes.Buffer{}, true)
	_, err = renderer.Place(preview.Image{MediaType: "image/jpeg", Data: []byte("jpg")}, imageproto.Placement{Columns: 1, Rows: 1})
	if !errors.Is(err, imageproto.ErrInvalidImage) {
		t.Fatalf("error = %v, want invalid image", err)
	}
	_, err = renderer.Place(preview.Image{MediaType: "image/png", Data: []byte("png")}, imageproto.Placement{})
	if !errors.Is(err, imageproto.ErrInvalidPlacement) {
		t.Fatalf("error = %v, want invalid placement", err)
	}
}

func TestIsKittyTerminal(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "1")
	t.Setenv("TERM", "xterm-256color")
	if !imageproto.IsKittyTerminal() {
		t.Fatal("KITTY_WINDOW_ID should identify Kitty")
	}
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM", "xterm-kitty")
	if !imageproto.IsKittyTerminal() {
		t.Fatal("TERM=xterm-kitty should identify Kitty")
	}
}

type countingWriter struct {
	bytes.Buffer
	writes int
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes++
	return w.Buffer.Write(p)
}

func (w *countingWriter) WriteString(s string) (int, error) {
	w.writes++
	return w.Buffer.WriteString(s)
}

func TestKittyTransmitsInOneWrite(t *testing.T) {
	var output countingWriter
	renderer := imageproto.NewKittyWithSupport(&output, true)
	image := preview.Image{MediaType: "image/png", Data: bytes.Repeat([]byte{0xab}, 40_000)}
	if _, err := renderer.Place(image, imageproto.Placement{Columns: 20, Rows: 10}); err != nil {
		t.Fatal(err)
	}
	// One write for the chunked image data, one for the placement.
	if output.writes != 2 {
		t.Fatalf("writes = %d, want 2", output.writes)
	}
	if chunks := strings.Count(output.String(), "\x1b_G"); chunks < 10 {
		t.Fatalf("graphics commands = %d, want the image split into chunks", chunks)
	}
}

func TestKittyMoveReplacesPlacementWithoutRetransmitting(t *testing.T) {
	var output bytes.Buffer
	renderer := imageproto.NewKittyWithSupport(&output, true)
	image := preview.Image{MediaType: "image/png", Data: []byte("png")}
	placed, err := renderer.Place(image, imageproto.Placement{X: 1, Y: 1, Columns: 10, Rows: 5})
	if err != nil {
		t.Fatal(err)
	}
	output.Reset()
	moved, err := renderer.Move(placed, imageproto.Placement{X: 2, Y: 3, Columns: 30, Rows: 15})
	if err != nil {
		t.Fatal(err)
	}
	if moved.ImageID != placed.ImageID || moved.PlacementID != placed.PlacementID || moved.Columns != 30 || moved.Y != 3 {
		t.Fatalf("moved = %#v", moved)
	}
	sequence := output.String()
	if strings.Contains(sequence, "a=t") {
		t.Fatalf("move re-sent image data: %q", sequence)
	}
	if !strings.Contains(sequence, "\x1b[4;3H") || !strings.Contains(sequence, "a=p,c=30,i=1,p=2,q=2,r=15") {
		t.Fatalf("move protocol = %q", sequence)
	}
}
