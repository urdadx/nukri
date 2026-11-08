package test

import (
	"bytes"
	"testing"

	"github.com/urdadx/nukri/internal/clipboard"
)

func TestOSC52EncodesText(t *testing.T) {
	if got := clipboard.OSC52("héllo\n"); got != "\x1b]52;c;aMOpbGxvCg==\x07" {
		t.Fatalf("OSC52 = %q", got)
	}
}

func TestCopyFallsBackToTerminal(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")
	var terminal bytes.Buffer
	method, err := clipboard.Copy("path", &terminal)
	if err != nil || method != "terminal" || terminal.String() != clipboard.OSC52("path") {
		t.Fatalf("method = %q, err = %v, wrote %q", method, err, terminal.String())
	}
	if _, err := clipboard.Copy("path", nil); err == nil {
		t.Fatal("without a tool or terminal, Copy should fail")
	}
}
