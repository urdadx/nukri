package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/core"
	. "github.com/urdadx/nukri/internal/file_info"
)

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

func TestDetectLicenseDocuments(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"ISC", "Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted. The software is provided as is.", "ISC License"},
		{"WTFPL", "DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE\nEveryone is permitted to copy and distribute verbatim or modified copies of this license document, and changing it is allowed as long as the name is changed.", "WTFPL (Do What The Fuck You Want To Public License)"},
		{"SPDX", "// SPDX-License-Identifier: Apache-2.0\n", "Apache License 2.0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "LICENSE")
			if err := os.WriteFile(path, []byte(test.text), 0o600); err != nil {
				t.Fatal(err)
			}
			facts := InspectPath(path, core.File)
			if facts.BuiltinClass != core.FileClassLicense || facts.SpecificTypeLabel == nil || *facts.SpecificTypeLabel != test.want {
				t.Fatalf("facts = %#v, want %q", facts, test.want)
			}
		})
	}
}
