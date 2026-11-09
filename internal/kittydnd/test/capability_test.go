package test

import (
	"io"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/kittydnd"
)

const (
	queryReply       = "\x1b]72;t=q;\x1b\\"
	deviceAttributes = "\x1b[?62;22;52c"
)

func TestProbeDetectsSupport(t *testing.T) {
	supported, rest := kittydnd.Probe(strings.NewReader(queryReply + deviceAttributes))
	if !supported || len(rest) != 0 {
		t.Fatalf("supported = %v, rest = %q; want supported with nothing left", supported, rest)
	}
}

// Terminals without the protocol answer only the device attributes query.
func TestProbeDetectsNoSupport(t *testing.T) {
	supported, rest := kittydnd.Probe(strings.NewReader(deviceAttributes))
	if supported || len(rest) != 0 {
		t.Fatalf("supported = %v, rest = %q; want unsupported", supported, rest)
	}
}

// The probe stops at the device attributes: input after them is not read.
func TestProbeStopsAtDeviceAttributes(t *testing.T) {
	src := io.MultiReader(strings.NewReader(deviceAttributes), strings.NewReader(queryReply+"later"))
	supported, _ := kittydnd.Probe(src)
	if supported {
		t.Fatal("a query reply after the device attributes counted as support")
	}
	if left, _ := io.ReadAll(src); string(left) != queryReply+"later" {
		t.Fatalf("probe read past the device attributes, leaving %q", left)
	}
}

// Keys typed while the terminal is probed belong to the program.
func TestProbeKeepsOtherInput(t *testing.T) {
	src := io.MultiReader(strings.NewReader("ab\x1b]72;t=q"), strings.NewReader(";\x1b\\c"), strings.NewReader("\x1b[?1;2cd"))
	supported, rest := kittydnd.Probe(src)
	if !supported || string(rest) != "abcd" {
		t.Fatalf("supported = %v, rest = %q; want supported with \"abcd\" left", supported, rest)
	}
}

// A terminal that never answers, or a probe that times out, means no support.
func TestProbeWithoutAnswerIsUnsupported(t *testing.T) {
	supported, rest := kittydnd.Probe(strings.NewReader("x"))
	if supported || string(rest) != "x" {
		t.Fatalf("supported = %v, rest = %q; want unsupported with \"x\" left", supported, rest)
	}
}

func TestAllowedOnlyOutsideMultiplexersAndSSH(t *testing.T) {
	for _, name := range []string{"TMUX", "ZELLIJ", "SSH_CONNECTION", "SSH_TTY"} {
		t.Setenv(name, "")
	}
	if !kittydnd.Allowed() {
		t.Fatal("drag and drop not allowed in a local terminal")
	}
	for _, name := range []string{"TMUX", "ZELLIJ", "SSH_CONNECTION", "SSH_TTY"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "1")
			if kittydnd.Allowed() {
				t.Fatalf("drag and drop allowed with %s set", name)
			}
		})
	}
}
