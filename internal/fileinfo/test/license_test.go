package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/fileinfo"
)

func TestInspectEntryFastDetectsCanonicalLicense(t *testing.T) {
	path := filepath.Join(t.TempDir(), "NOTICE")
	if err := os.WriteFile(path, []byte("Project legal notices."), 0o600); err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectEntryFast(&core.Entry{Name: "NOTICE", Path: path, Kind: core.File})
	if facts.BuiltinClass != core.FileClassLicense || facts.SpecificTypeLabel == nil || *facts.SpecificTypeLabel != "License Document" {
		t.Fatalf("facts = %#v, want generic license", facts)
	}
}

// apacheExcerpt keeps the words ("permitted", "submit", "no-charge",
// "royalty-free") that used to make Apache match the loose MIT check.
const apacheExcerpt = `                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   Subject to the terms and conditions of this License, each Contributor
   hereby grants to You a perpetual, worldwide, non-exclusive, no-charge,
   royalty-free, irrevocable copyright license. Permission is granted as
   permitted by law for any Contribution you submit.

       http://www.apache.org/licenses/LICENSE-2.0
`

func TestDetectLicenseDocuments(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"ISC", "Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted. The software is provided as is.", "ISC License"},
		{"WTFPL", "DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE\nEveryone is permitted to copy and distribute verbatim or modified copies of this license document, and changing it is allowed as long as the name is changed.", "WTFPL (Do What The Fuck You Want To Public License)"},
		{"SPDX", "// SPDX-License-Identifier: Apache-2.0\n", "Apache License 2.0"},
		{"MIT", "MIT License\n\nCopyright (c) 2026 Someone\n\nPermission is hereby granted, free of charge, to any person obtaining a copy of this software, to whom the Software is furnished to do so.\n\nTHE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND.", "MIT License"},
		{"Apache full text", apacheExcerpt, "Apache License 2.0"},
		{"GPLv3 full text", "GNU GENERAL PUBLIC LICENSE\n Version 3, 29 June 2007\n\nEveryone is permitted to copy and distribute verbatim copies. Free software, no charge, permission granted. Use the GNU Lesser General Public License instead.", "GNU General Public License v3.0"},
		{"MPL full text", "Mozilla Public License Version 2.0\n\nExhibit A\nThis Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at http://mozilla.org/MPL/2.0/.", "Mozilla Public License 2.0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "LICENSE")
			if err := os.WriteFile(path, []byte(test.text), 0o600); err != nil {
				t.Fatal(err)
			}
			facts := fileinfo.InspectPath(path, core.File)
			if facts.BuiltinClass != core.FileClassLicense || facts.SpecificTypeLabel == nil || *facts.SpecificTypeLabel != test.want {
				t.Fatalf("facts = %#v, want %q", facts, test.want)
			}
		})
	}
}

func TestReadmeMentioningLicenseIsNotLicense(t *testing.T) {
	text := "# Project\n\nA tool.\n\n## Install\n\nRun it.\n\n## Usage\n\nUse it.\n\n## Notes\n\nNone.\n\n## License\n\nReleased under the MIT License.\n"
	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectPath(path, core.File)
	if facts.BuiltinClass == core.FileClassLicense {
		t.Fatalf("facts = %#v, want a non-license README", facts)
	}
}
