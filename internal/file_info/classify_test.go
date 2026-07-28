package fileinfo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/core"
	registrydata "github.com/urdadx/nukri/internal/preview/code/registry/data"
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

func TestInspectPathUsesRegistryForPythonShebang(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script")
	if err := os.WriteFile(path, []byte("#!/usr/bin/env python3\nprint('ok')\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	facts := InspectPath(path, core.File)
	if facts.BuiltinClass != core.FileClassCode || facts.Preview.CodeSyntax == nil || *facts.Preview.CodeSyntax != "python" {
		t.Fatalf("facts = %#v, want Python code", facts)
	}
}

func TestExactNameMetadataResolvesInRegistry(t *testing.T) {
	for name, metadata := range exactNames {
		facts, ok := inspectExactName(name)
		if !ok {
			t.Errorf("%q has metadata %#v but no registry language", name, metadata)
			continue
		}
		if facts.BuiltinClass != metadata.class {
			t.Errorf("%q class = %v, want %v", name, facts.BuiltinClass, metadata.class)
		}
	}
}

func TestInspectPathDetectsLicenseContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "LICENSE.md")
	text := "MIT License\n\nPermission is hereby granted, free of charge, to any person obtaining a copy."
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	facts := InspectPath(path, core.File)
	if facts.BuiltinClass != core.FileClassLicense || facts.SpecificTypeLabel == nil || *facts.SpecificTypeLabel != "MIT License" {
		t.Fatalf("facts = %#v, want MIT license", facts)
	}
}

func TestInspectEntryFastDetectsCanonicalLicense(t *testing.T) {
	path := filepath.Join(t.TempDir(), "NOTICE")
	if err := os.WriteFile(path, []byte("Project legal notices."), 0o600); err != nil {
		t.Fatal(err)
	}

	facts := InspectEntryFast(&core.Entry{Name: "NOTICE", Path: path, Kind: core.File})
	if facts.BuiltinClass != core.FileClassLicense || facts.SpecificTypeLabel == nil || *facts.SpecificTypeLabel != "License Document" {
		t.Fatalf("facts = %#v, want generic license", facts)
	}
}

func TestInspectPathDoesNotClassifyOrdinaryReadmeAsLicense(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte("# Project\n\nBuild and usage instructions."), 0o600); err != nil {
		t.Fatal(err)
	}

	facts := InspectPath(path, core.File)
	if facts.BuiltinClass != core.FileClassDocument {
		t.Fatalf("facts = %#v, want document", facts)
	}
}

func TestRegistryExtensionsProduceMatchingPreviews(t *testing.T) {
	for _, entry := range registrydata.AllLanguages() {
		for _, extension := range entry.Extensions {
			preview := previewForExtension(extension)
			if preview.CodeSyntax == nil || *preview.CodeSyntax != entry.Language.CanonicalID {
				t.Errorf("extension %q preview = %#v, want syntax %q", extension, preview, entry.Language.CanonicalID)
			}
		}
	}
}
