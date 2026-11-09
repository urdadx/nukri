package test

import (
	"io"
	"regexp"
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

func TestAllowedOutsideMultiplexers(t *testing.T) {
	for _, name := range []string{"TMUX", "ZELLIJ"} {
		t.Setenv(name, "")
	}
	t.Setenv("SSH_CONNECTION", "10.0.0.1 22 10.0.0.2 22")
	if !kittydnd.Allowed() {
		t.Fatal("drag and drop not allowed over SSH")
	}
	for _, name := range []string{"TMUX", "ZELLIJ"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "1")
			if kittydnd.Allowed() {
				t.Fatalf("drag and drop allowed with %s set", name)
			}
		})
	}
}

func TestHashMachineIDHasProtocolForm(t *testing.T) {
	id := kittydnd.HashMachineID("0123456789abcdef0123456789abcdef")
	if !regexp.MustCompile(`^1:[0-9a-f]{64}$`).MatchString(id) {
		t.Fatalf("machine id = %q, want 1: and 64 hex digits", id)
	}
	if id != kittydnd.HashMachineID("0123456789abcdef0123456789abcdef") || id == kittydnd.HashMachineID("another") {
		t.Fatal("machine id must depend only on the raw id")
	}
}
