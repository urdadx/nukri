package fileinfo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/core"
)

func TestReadLicenseTextPrefixReadsRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "LICENSE")
	if err := os.WriteFile(path, []byte("MIT License"), 0o600); err != nil {
		t.Fatal(err)
	}

	text, err := readLicenseTextPrefix(path, FastLicenseSniffByteLimit)
	if err != nil {
		t.Fatal(err)
	}
	if text != "MIT License" {
		t.Fatalf("text = %q, want %q", text, "MIT License")
	}
}

func TestCanonicalLicenseCandidateNames(t *testing.T) {
	for _, name := range []string{"license", "license.md", "copying.txt", "readme-license"} {
		if !isCanonicalLicenseCandidateName(name) {
			t.Errorf("%q should be a canonical candidate", name)
		}
	}
	if isCanonicalLicenseCandidateName("licensed.go") {
		t.Error("licensed.go should not be a canonical candidate")
	}
}

func TestLicensePhraseNormalization(t *testing.T) {
	if !containsPhrase(normalizeHighSignalText("MIT LICENSE"), "MIT License") {
		t.Error("phrase matching should be case-insensitive")
	}
	if !containsPhrase(normalizeHighSignalText("クリエイティブ・コモンズ・ジャパン"), "クリエイティブ・コモンズ・ジャパン") {
		t.Error("phrase matching should preserve Unicode letters")
	}
	if containsPhrase("anything", "") {
		t.Error("an empty phrase should not match")
	}
}

func TestSniffLicenseFileType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "LICENSE")
	text := "MIT License\n\nPermission is hereby granted, free of charge, to any person obtaining a copy."
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	base := plain(core.FileClassDocument, nil)

	facts := SniffLicenseFileType(path, "license", "", base)
	if facts.BuiltinClass != core.FileClassLicense {
		t.Fatalf("BuiltinClass = %v, want license", facts.BuiltinClass)
	}
	if facts.SpecificTypeLabel == nil || *facts.SpecificTypeLabel != "MIT License" {
		t.Fatalf("SpecificTypeLabel = %v, want MIT License", facts.SpecificTypeLabel)
	}
}

func TestCanonicalUnknownLicenseUsesGenericLabel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "NOTICE")
	if err := os.WriteFile(path, []byte("Project legal notices."), 0o600); err != nil {
		t.Fatal(err)
	}

	facts := SniffBrowserLicenseFileType(path, "notice", "", plain(core.FileClassDocument, nil))
	if facts.BuiltinClass != core.FileClassLicense {
		t.Fatalf("BuiltinClass = %v, want license", facts.BuiltinClass)
	}
	if facts.SpecificTypeLabel == nil || *facts.SpecificTypeLabel != "License Document" {
		t.Fatalf("SpecificTypeLabel = %v, want License Document", facts.SpecificTypeLabel)
	}
}

func TestDetectLicenseDocuments(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			"ISC",
			"Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted. The software is provided as is.",
			"ISC License",
		},
		{
			"WTFPL",
			"DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE\nEveryone is permitted to copy and distribute verbatim or modified copies of this license document, and changing it is allowed as long as the name is changed.",
			"WTFPL (Do What The Fuck You Want To Public License)",
		},
		{
			"SPDX",
			"// SPDX-License-Identifier: Apache-2.0\n",
			"Apache License 2.0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detection, ok := detectLicenseDocuments(test.text)
			if !ok || !detection.IsSpecific || detection.DetailLabel != test.want {
				t.Fatalf("detection = %#v, ok = %v, want %q", detection, ok, test.want)
			}
		})
	}
}
