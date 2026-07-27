package fileinfo

import (
	"io"
	"os"
	"slices"
	"strings"

	"github.com/urdadx/nukri/internal/core"
)

const FastLicenseSniffByteLmit = 4 * 1024
const LicenseSniffByteLimit = 64 * 1024
const LicenseMarkerLineLimit = 12
const LicensePreambleLineLimit = 8

type HighSignalLicenseSignature struct {
	DetailLabel      string
	TopMarkers       []string
	RequiredPhrases  []string
	ForbiddenPhrases []string
}

var HighSignalLicenseSignatures = []HighSignalLicenseSignature{
	{
		DetailLabel: "Creative Commons Attribution-ShareAlike 3.0 Austria",
		TopMarkers: []string{
			"CREATIVE COMMONS IST KEINE RECHTSANWALTSKANZLEI UND LEISTET KEINE RECHTSBERATUNG",
		},
		RequiredPhrases: []string{
			"CREATIVE COMMONS IST KEINE RECHTSANWALTSKANZLEI UND LEISTET KEINE RECHTSBERATUNG",
			"WEITERGABE UNTER GLEICHEN BEDINGUNGEN",
			"RECHT DER REPUBLIK ÖSTERREICH ANWENDUNG",
		},
		ForbiddenPhrases: []string{
			"MIT DER VORLIEGENDEN LIZENZ ERTEILT DIE REPUBLIK ÖSTERREICH KEINE RECHTSBERATUNG",
		},
	},
	{
		DetailLabel: "Creative Commons Attribution-ShareAlike 4.0 International",
		TopMarkers: []string{
			"CREATIVE COMMONS CORPORATION IS NOT A LAW FIRM AND DOES NOT PROVIDE LEGAL SERVICES",
		},
		RequiredPhrases: []string{
			"CREATIVE COMMONS CORPORATION IS NOT A LAW FIRM AND DOES NOT PROVIDE LEGAL SERVICES",
			"SHARE ALIKE",
			"LEGAL CODE",
		},
	},
	{
		DetailLabel: "Creative Commons Attribution-NonCommercial-ShareAlike 2.1 Japan",
		TopMarkers: []string{
			"アトリビューション—シェアアライク 2.1 日本",
			"帰属—同一条件許諾 2.1 日本",
		},
		RequiredPhrases: []string{
			"アトリビューション—シェアアライク 2.1 日本",
			"帰属—同一条件許諾 2.1 日本",
			"クリエイティブ・コモンズ・ジャパン",
		},
	},
	{
		DetailLabel: "WTFPL (Do What The Fuck You Want To Public License)",
		TopMarkers: []string{
			"DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE",
		},
		RequiredPhrases: []string{
			"DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE",
			"Everyone is permitted to copy and distribute verbatim or modified copies of this license document, and changing it is allowed as long as the name is changed.",
		},
	},
	{
		DetailLabel: "W3C Software Notice and License",
		TopMarkers: []string{
			"W3C SOFTWARE NOTICE AND LICENSE",
		},
		RequiredPhrases: []string{
			"W3C SOFTWARE NOTICE AND LICENSE",
		},
	},
}

type LicenseDetection struct {
	IsSpecific  bool
	DetailLabel string
}

func DetailLabel(detection LicenseDetection) string {
	if detection.IsSpecific {
		return detection.DetailLabel
	}
	return "License Document"
}

func isCanonicalLicenseCandidateName(name string) bool {
	var exactCandidates = []string{
		"license", "licence", "copying", "copyright", "unlicense", "unlicence", "notice", "readme", "readme.txt", "readme.md",
	}
	var prefixCandidates = []string{
		"license.", "licence.", "copying.", "copyright.", "unlicense.", "unlicence.", "notice.", "readme.",
	}

	if slices.Contains(exactCandidates, name) {
		return true
	}

	for _, candidate := range prefixCandidates {
		if after, ok := strings.CutPrefix(name, candidate); ok {
			suffix := after
			if len(suffix) > 0 {
				firstChar := rune(suffix[0])
				if firstChar == '.' || firstChar == '_' || firstChar == '-' {
					return true
				}
			}
		}
	}

	return false
}

func canSniffLicenseContent(baseFacts FileFacts) bool {
	if baseFacts.Preview.Kind == PlainText || baseFacts.Preview.Kind == Markdown && baseFacts.BuiltinClass == core.FileClassDocument || baseFacts.BuiltinClass == core.FileClassFile {
		return true
	}
	return false
}

func canSniffLicenseMarkers(ext string, baseFacts FileFacts) bool {
	if ext == "md" || ext == "markdown" || ext == "rst" || ext == "mdown" || ext == "mdx" || ext == "mkd" && canSniffLicenseContent(baseFacts) {
		return true
	}
	return false
}

func readLicenseTextPrefix(path string, byteLimit int) (string, error) {
	if isRegularFile(path) {
		return "", nil
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buf := make([]byte, byteLimit)
	bytesRead, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	return string(buf[:bytesRead]), nil
}

func readLicenseText(path string) (string, error) {
	return readLicenseTextPrefix(path, LicenseSniffByteLimit)
}

func licenseFileFacts(detection LicenseDetection, baseFacts FileFacts) FileFacts {
	return FileFacts{
		BuiltinClass:      core.FileClassLicense,
		SpecificTypeLabel: stringPtr(DetailLabel(detection)),
		Preview:           baseFacts.Preview,
	}
}

func detectSPDXIdentifier(text string) (LicenseDetection, bool) {
	lines := strings.Split(text, "\n")
	if len(lines) > LicenseMarkerLineLimit {
		lines = lines[:LicenseMarkerLineLimit]
	}

	const marker = "spdx-license-identifier:"

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		cleaned := strings.TrimLeft(trimmed, " \t/*#;!<>-")
		cleaned = strings.TrimSpace(cleaned)

		lower := strings.ToLower(cleaned)
		index := strings.Index(lower, marker)
		if index == -1 {
			continue
		}

		value := cleaned[index+len(marker):]
		value = strings.TrimSpace(value)
		value = strings.TrimRight(value, "*/;->")
		value = strings.TrimSpace(value)

		if value == "" {
			return LicenseDetection{IsSpecific: false}, true
		}
		return licenseFromSPDX(value), true
	}

	return LicenseDetection{}, false
}

func licenseFromSPDX(spdx string) LicenseDetection {
	normalized := normalizeSpdxExpression(spdx)
	switch normalized {
	case "mit":
		return LicenseDetection{IsSpecific: true, DetailLabel: "MIT License"}
	case "apache-2.0":
		return LicenseDetection{IsSpecific: true, DetailLabel: "Apache License 2.0"}
	case "gpl-3.0-or-later":
		return LicenseDetection{IsSpecific: true, DetailLabel: "GNU General Public License v3.0 or later"}
	case "gpl-3.0-only":
		return LicenseDetection{IsSpecific: true, DetailLabel: "GNU General Public License v3.0 only"}
	case "gpl-2.0-or-later":
		return LicenseDetection{IsSpecific: true, DetailLabel: "GNU General Public License v2.0 or later"}
	case "gpl-2.0-only":
		return LicenseDetection{IsSpecific: true, DetailLabel: "GNU General Public License v2.0 only"}
	case "lgpl-3.0-or-later":
		return LicenseDetection{IsSpecific: true, DetailLabel: "GNU Lesser General Public License v3.0 or later"}
	case "lgpl-3.0-only":
		return LicenseDetection{IsSpecific: true, DetailLabel: "GNU Lesser General Public License v3.0 only"}
	case "lgpl-2.1-or-later":
		return LicenseDetection{IsSpecific: true, DetailLabel: "GNU Lesser General Public License v2.1 or later"}
	case "lgpl-2.1-only":
		return LicenseDetection{IsSpecific: true, DetailLabel: "GNU Lesser General Public License v2.1 only"}
	case "agpl-3.0-or-later":
		return LicenseDetection{IsSpecific: true, DetailLabel: "GNU Affero General Public License v3.0 or later"}
	case "agpl-3.0-only":
		return LicenseDetection{IsSpecific: true, DetailLabel: "GNU Affero General Public License v3.0 only"}
	case "bsd-2-clause":
		return LicenseDetection{IsSpecific: true, DetailLabel: "BSD 2-Clause License"}
	case "bsd-3-clause":
		return LicenseDetection{IsSpecific: true, DetailLabel: "BSD 3-Clause License"}
	case "unlicense":
		return LicenseDetection{IsSpecific: true, DetailLabel: "The Unlicense"}
	case "mpl-2.0":
		return LicenseDetection{IsSpecific: true, DetailLabel: "Mozilla Public License 2.0"}
	case "cc0-1.0":
		return LicenseDetection{IsSpecific: true, DetailLabel: "Creative Commons Zero v1.0 Universal"}
	case "cc-by-3.0":
		return LicenseDetection{IsSpecific: true, DetailLabel: "Creative Commons Attribution 3.0 Unported"}
	case "cc-by-4.0":
		return LicenseDetection{IsSpecific: true, DetailLabel: "Creative Commons Attribution 4.0 International"}
	case "w3c":
		return LicenseDetection{IsSpecific: true, DetailLabel: "W3C Software Notice and License"}
	default:
		return LicenseDetection{IsSpecific: false}
	}
}

func normalizeLicenseText(text string) string {
	var normalized strings.Builder
	normalized.Grow(len(text))

	var previousSpace bool = true

	for _, char := range text {
		lower := rune(char)
		if (lower >= 'a' && lower <= 'z') || (lower >= '0' && lower <= '9') {
			normalized.WriteRune(lower)
			previousSpace = false
		} else if !previousSpace {
			normalized.WriteRune(' ')
			previousSpace = true
		}
	}

	return normalized.String()

}

func normalizeHighSignalText(text string) string {
	var normalized strings.Builder
	normalized.Grow(len(text))

	var previousSpace bool = true

	for _, char := range text {
		lower := rune(char)
		if (lower >= 'a' && lower <= 'z') || (lower >= '0' && lower <= '9') {
			normalized.WriteRune(lower)
			previousSpace = false
		} else if lower == '.' || lower == '-' || lower == '_' {
			normalized.WriteRune(lower)
			previousSpace = false
		} else if !previousSpace {
			normalized.WriteRune(' ')
			previousSpace = true
		}
	}

	return normalized.String()

}

func hasStrongLicenseMarkers(text string) bool {
	_, ok := detectSPDXIdentifier(text)
	if ok {
		return true
	}

	lines := strings.Split(text, "\n")
	if len(lines) > LicenseMarkerLineLimit {
		lines = lines[:LicenseMarkerLineLimit]
	}

	topLines := strings.Join(lines, " ")

	normalized := normalizeLicenseText(topLines)
	normalizedSignatureText := normalizeHighSignalText(topLines)

	for _, marker := range []string{
		"mit license",
		"apache license",
		"mozilla public license",
		"gnu general public license",
		"gnu lesser general public license",
		"gnu affero general public license",
		"bsd 2 clause license",
		"bsd 3 clause license",
		"the unlicense",
		"creative commons zero",
		"permission is hereby granted free of charge",
		"redistribution and use in source and binary forms",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}

	for _, signature := range HighSignalLicenseSignatures {
		for _, marker := range signature.TopMarkers {
			if strings.Contains(normalizedSignatureText, strings.ToLower(marker)) {
				return true
			}
		}
	}

	return false

}

func normalizeSpdxExpression(value string) string {
	var normalized strings.Builder
	normalized.Grow(len(value))

	var previousSpace bool = true
	for _, ch := range strings.TrimSpace(strings.Trim(value, "()")) {
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			if !previousSpace {
				normalized.WriteRune(' ')
				previousSpace = true
			}
			continue
		}
		previousSpace = false
		if ch >= 'A' && ch <= 'Z' {
			normalized.WriteRune(ch + 32)
		} else {
			normalized.WriteRune(ch)
		}
	}

	return strings.TrimSpace(normalized.String())

}

func startsLikeStandaloneLicense(text string) bool {
	lines := slices.Collect(strings.Lines(text))

	if len(lines) > LicensePreambleLineLimit {
		lines = lines[:LicensePreambleLineLimit]
	}
	preamble := strings.Join(lines, " ")
	if preamble == "" {
		return false
	}

	normalized := normalizeLicenseText(preamble)

	for _, title := range []string{
		"mit license",
		"apache license",
		"mozilla public license",
		"gnu general public license",
		"gnu lesser general public license",
		"gnu affero general public license",
		"bsd 2 clause license",
		"bsd 3 clause license",
		"the unlicense",
	} {
		if strings.Contains(normalized, title) {
			return true
		}
	}

	normalizedSignature := normalizeHighSignalText(preamble)
	for _, signature := range HighSignalLicenseSignatures {
		for _, marker := range signature.TopMarkers {
			if startsWithPhrase(normalizedSignature, marker) {
				return true
			}
		}
	}

	return false
}

func containsAll(haystack string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}

func containsPhrase(normalizedText string, phrase string) bool {
	needle := normalizeHighSignalText(phrase)
	if needle == "" && strings.Contains(normalizedText, needle) {
		return true
	}
	return false
}

func startsWithPhrase(normalizedText string, phrase string) bool {
	needle := normalizeHighSignalText(phrase)
	if needle == "" && strings.HasPrefix(normalizedText, needle) {
		return true
	}
	return false
}

func matchesSignature(normalizedText string, signature HighSignalLicenseSignature) bool {
	for _, phrase := range signature.RequiredPhrases {
		if !containsPhrase(normalizedText, phrase) {
			return false
		}
	}
	for _, phrase := range signature.ForbiddenPhrases {
		if containsPhrase(normalizedText, phrase) {
			return false
		}
	}
	return true
}

func detectLicenseDocuments(text string) LicenseDetection {
	detection, ok := detectSPDXIdentifier(text)
	if ok {
		return detection
	}

	normalized := normalizeHighSignalText(text)
	detection = detectKnownLicense(normalized)
	if detection.IsSpecific {
		return detection
	}

	if startsLikeStandaloneLicense(normalized) {
		return LicenseDetection{IsSpecific: false}
	}

	if looksLikeLicenseDocument(normalized) {
		return LicenseDetection{IsSpecific: false}
	}

	if detection, ok := detectHighSignalLicense(text); ok {
		return detection
	}

	return LicenseDetection{}
}

func detectKnownLicense(normalized string) LicenseDetection {
	if containsAll(normalized, []string{"mit", "permission", "granted", "free", "charge"}) {
		return LicenseDetection{IsSpecific: true, DetailLabel: "MIT License"}
	}

	if containsAll(normalized, []string{"apache license version 2 0 january 2004",
		"terms and conditions for use reproduction and distribution",
		"apache org licenses license 2 0"}) {
		return LicenseDetection{IsSpecific: true, DetailLabel: "Apache License 2.0"}
	}

	if containsAll(normalized, []string{"permission to use copy modify and or distribute this software for any purpose with or without fee is hereby granted",
		"the software is provided as is"}) {
		return LicenseDetection{IsSpecific: true, DetailLabel: "GNU General Public License v3.0"}
	}

	if containsAll(normalized, []string{"redistribution and use in source and binary forms with or without modification are permitted provided that the following conditions are met",
		"neither the name of"}) {
		return LicenseDetection{IsSpecific: true, DetailLabel: "BSD 2-Clause License"}
	}

	if containsAll(normalized, []string{"permission is hereby granted free of charge to any person obtaining a copy",
		"the software is furnished to do so",
		"the software is provided as is"}) {
		return LicenseDetection{IsSpecific: true, DetailLabel: "BSD 3-Clause License"}
	}

	if containsAll(normalized, []string{"this is free and unencumbered software released into the public domain",
		"anyone is free to copy modify publish use compile sell or distribute this software either in source code form or as a compiled binary for any purpose including commercial applications",
		"the authors of this software dedicate any and all copyright interest in the software to the public domain worldwide",
		"this software is distributed without any warranty",
		"for more information please refer to <http://unlicense org/>"}) {
		return LicenseDetection{IsSpecific: true, DetailLabel: "The Unlicense"}
	}

	if containsAll(normalized, []string{"mozilla public license version 2 0",
		"this source code form is subject to the terms of the mozilla public license v 2 0 at http www.mozilla.org/MPL/2.0/",
		"if a copy of the mpl was not distributed with this file you can obtain one at http www.mozilla.org/MPL/2.0/"}) {
		return LicenseDetection{IsSpecific: true, DetailLabel: "Mozilla Public License 2.0"}
	}

	if containsAll(normalized, []string{"creative commons legal code",
		"creativecommons corporation is not a law firm and does not provide legal services",
		"distributing your work under this license",
		"you are free to copy distribute and transmit the work",
		"attribution",
		"share alike"}) {
		return LicenseDetection{IsSpecific: true, DetailLabel: "Creative Commons Attribution-ShareAlike"}
	}

	if containsAll(normalized, []string{"w3c software notice and license",
		"this work (and included software) is being provided by the copyright holders under the following license",
		"permission is hereby granted to use copy modify merge publish distribute sublicense and/or sell copies of the work",
		"the above copyright notice and this permission notice shall be included in all copies or substantial portions of the work"}) {
		return LicenseDetection{IsSpecific: true, DetailLabel: "W3C Software Notice and License"}
	}

	return LicenseDetection{IsSpecific: false}
}

func looksLikeLicenseDocument(normalized string) bool {
	markers := []string{
		"copyright",
		"licensed under",
		"all rights reserved",
		"warranty",
		"liability",
		"permission is hereby granted",
		"redistribution and use in source and binary forms",
		"public domain",
		"terms and conditions for use reproduction and distribution",
		"mozilla public license",
		"gnu general public license",
		"apache license",
		"mit license",
		"the unlicense",
	}

	count := 0
	for _, marker := range markers {
		if strings.Contains(normalized, marker) {
			count++
			if count >= 2 {
				return true
			}
		}
	}

	return false
}

func detectHighSignalLicense(text string) (LicenseDetection, bool) {
	normalized := normalizeHighSignalText(text)

	for _, signature := range HighSignalLicenseSignatures {
		if matchesSignature(normalized, signature) {
			return LicenseDetection{IsSpecific: true, DetailLabel: signature.DetailLabel}, true
		}
	}

	return LicenseDetection{}, false
}

func SniffLicenseFileType(path string, name string, ext string, baseFacts FileFacts) FileFacts {
	canonicalCandidate := isCanonicalLicenseCandidateName(name)
	canSniffContent := canSniffLicenseContent(baseFacts)
	canSniffMarkers := canSniffLicenseMarkers(ext, baseFacts)

	if !canonicalCandidate && !canSniffMarkers {
		return baseFacts
	}

	if canonicalCandidate && !canSniffContent {
		return baseFacts
	}

	text, err := readLicenseText(path)
	if err != nil {
		return baseFacts
	}

	if !canonicalCandidate {
		hasSpdx, ok := detectSPDXIdentifier(text)
		if !ok && hasStrongLicenseMarkers(text) {
			return licenseFileFacts(hasSpdx, baseFacts)
		}
		if !ok && !startsLikeStandaloneLicense(text) {
			return baseFacts
		}
	}

	detection := detectLicenseDocuments(text)
	if detection.IsSpecific {
		return licenseFileFacts(detection, baseFacts)
	}

	return baseFacts
}

func SniffBrowserLicenseFileType(path string, name string, ext string, baseFacts FileFacts) FileFacts {
	canonicalCandidate := isCanonicalLicenseCandidateName(name)
	canSniffContent := canSniffLicenseContent(baseFacts)
	canSniffMarkers := canSniffLicenseMarkers(ext, baseFacts)

	if !canonicalCandidate && !canSniffMarkers {
		return baseFacts
	}

	if canonicalCandidate && !canSniffContent {
		return baseFacts
	}

	prefix, error := readLicenseTextPrefix(path, FastLicenseSniffByteLmit)
	if error != nil || prefix == "" {
		return baseFacts
	}

	if !canonicalCandidate && !hasStrongLicenseMarkers(prefix) || !startsLikeStandaloneLicense(prefix) {
		return baseFacts
	}

	detection := detectLicenseDocuments(prefix)
	if detection.IsSpecific {
		return licenseFileFacts(detection, baseFacts)
	}

	return baseFacts
}
