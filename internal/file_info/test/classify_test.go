package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/core"
	. "github.com/urdadx/nukri/internal/file_info"
)

func inspectTemporaryFile(t *testing.T, name string, content []byte) FileFacts {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return InspectPath(path, core.File)
}

func TestInspectPathSniffsExtensionlessShebang(t *testing.T) {
	facts := inspectTemporaryFile(t, ".script", []byte("#!/usr/bin/env -S bash\necho ok\n"))
	if facts.BuiltinClass != core.FileClassCode || facts.SpecificTypeLabel == nil || *facts.SpecificTypeLabel != "Bash script" {
		t.Fatalf("facts = %#v, want Bash code", facts)
	}
}

func TestInspectPathSniffsImageSignature(t *testing.T) {
	facts := inspectTemporaryFile(t, "image", []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	if facts.BuiltinClass != core.FileClassImage {
		t.Fatalf("BuiltinClass = %v, want image", facts.BuiltinClass)
	}
}

func TestInspectPathUsesConfigModeline(t *testing.T) {
	facts := inspectTemporaryFile(t, "app.conf", []byte("# -*- mode: ini -*-\n[server]\nport=8080\n"))
	if facts.BuiltinClass != core.FileClassConfig || facts.Preview.CodeSyntax == nil || *facts.Preview.CodeSyntax != "ini" {
		t.Fatalf("facts = %#v, want INI config", facts)
	}
}

func TestInspectPathUsesRegistryForPythonShebang(t *testing.T) {
	facts := inspectTemporaryFile(t, "script", []byte("#!/usr/bin/env python3\nprint('ok')\n"))
	if facts.BuiltinClass != core.FileClassCode || facts.Preview.CodeSyntax == nil || *facts.Preview.CodeSyntax != "python" {
		t.Fatalf("facts = %#v, want Python code", facts)
	}
}

func TestInspectPathDetectsLicenseContent(t *testing.T) {
	text := "MIT License\n\nPermission is hereby granted, free of charge, to any person obtaining a copy."
	facts := inspectTemporaryFile(t, "LICENSE.md", []byte(text))
	if facts.BuiltinClass != core.FileClassLicense || facts.SpecificTypeLabel == nil || *facts.SpecificTypeLabel != "MIT License" {
		t.Fatalf("facts = %#v, want MIT license", facts)
	}
}

func TestInspectPathDoesNotClassifyOrdinaryReadmeAsLicense(t *testing.T) {
	facts := inspectTemporaryFile(t, "README.md", []byte("# Project\n\nBuild and usage instructions."))
	if facts.BuiltinClass != core.FileClassDocument {
		t.Fatalf("facts = %#v, want document", facts)
	}
}
