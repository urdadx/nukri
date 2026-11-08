package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/ui/browser"
)

func TestVisualStateWritesOnlyOnFlush(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty") // select the Kitty protocol
	var terminal bytes.Buffer
	state := &browser.VisualState{}
	image := &preview.Image{MediaType: "image/png", Data: []byte("png data"), Width: 10, Height: 10}

	state.Show(image, 2, 3, 10, 5)
	if terminal.Len() != 0 {
		t.Fatal("Show must not write; rendering stays free of I/O")
	}
	state.Flush(&terminal)
	if !strings.Contains(terminal.String(), "a=t") || !strings.Contains(terminal.String(), "a=p") {
		t.Fatalf("first flush should transmit and place the image: %q", terminal.String())
	}

	terminal.Reset()
	state.Show(image, 2, 3, 10, 5)
	state.Flush(&terminal)
	if terminal.Len() != 0 {
		t.Fatalf("an unchanged image must not be re-sent: %q", terminal.String())
	}

	state.Show(image, 4, 3, 8, 4)
	state.Flush(&terminal)
	if strings.Contains(terminal.String(), "a=t") || !strings.Contains(terminal.String(), "a=p,c=8") {
		t.Fatalf("a moved image should be re-placed, not re-sent: %q", terminal.String())
	}

	terminal.Reset()
	state.Clear()
	state.Flush(&terminal)
	if !strings.Contains(terminal.String(), "a=d") {
		t.Fatalf("clearing should delete the image: %q", terminal.String())
	}
	terminal.Reset()
	state.Flush(&terminal)
	if terminal.Len() != 0 {
		t.Fatalf("nothing to delete twice: %q", terminal.String())
	}
}
