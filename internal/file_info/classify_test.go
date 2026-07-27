package fileinfo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/core"
)

func TestInspectPathSniffsExtensionlessShebang(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".script")
	if err := os.WriteFile(path, []byte("#!/usr/bin/env -S bash\necho ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	facts := InspectPath(path, core.File)
	if facts.BuiltinClass != core.FileClassCode {
		t.Fatalf("BuiltinClass = %v, want code", facts.BuiltinClass)
	}
	if facts.SpecificTypeLabel == nil || *facts.SpecificTypeLabel != "Bash script" {
		t.Fatalf("SpecificTypeLabel = %v, want Bash script", facts.SpecificTypeLabel)
	}
}

func TestInspectPathSniffsImageSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image")
	if err := os.WriteFile(path, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, 0o600); err != nil {
		t.Fatal(err)
	}

	facts := InspectPath(path, core.File)
	if facts.BuiltinClass != core.FileClassImage {
		t.Fatalf("BuiltinClass = %v, want image", facts.BuiltinClass)
	}
}

func TestInspectPathUsesConfigModeline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.conf")
	if err := os.WriteFile(path, []byte("# -*- mode: ini -*-\n[server]\nport=8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	facts := InspectPath(path, core.File)
	if facts.BuiltinClass != core.FileClassConfig || facts.Preview.CodeSyntax == nil || *facts.Preview.CodeSyntax != "ini" {
		t.Fatalf("facts = %#v, want INI config", facts)
	}
}

func TestScoreConfigPrefix(t *testing.T) {
	ini, shell := scoreConfigPrefix("; comment\n[server]\nport=8080\nhost=localhost\n")
	if ini != 7 || shell != 2 {
		t.Fatalf("INI scores = (%d, %d), want (7, 2)", ini, shell)
	}

	ini, shell = scoreConfigPrefix("export ROOT=/tmp\nif [[ ${READY} ]]; then\nfi\n")
	if ini != 0 || shell != 6 {
		t.Fatalf("shell scores = (%d, %d), want (0, 6)", ini, shell)
	}
}
