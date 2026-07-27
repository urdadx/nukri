package registry

import (
	"slices"
	"strings"

	"github.com/urdadx/nukri/internal/preview/code/registry/data"
)

func LanguageForExtension(value string) (RegisteredLanguage, bool) {
	return LanguageForAlias(value, func(entry *RegistryEntry) []string {
		return entry.Extensions
	})
}

func LanguageForExactFilename(value string) (RegisteredLanguage, bool) {
	normalized := normalize(value)
	if isEnvName(normalized) {
		return LanguageForCodeSyntax("dotenv")

	}
	data := data.AllLanguages()
	for i := range data {
		entry := &data[i]
		if contains(entry.ExactFilenames, normalized) {
			return entry.Language, true
		}
	}
	return RegisteredLanguage{}, false
}

func LanguageForShebangInterpreter(value string) (RegisteredLanguage, bool) {
	return LanguageForAlias(value, func(entry *RegistryEntry) []string {
		return entry.ShebangInterpreters
	})
}

func LanguageForModeline(value string) (RegisteredLanguage, bool) {
	return LanguageForAlias(value, func(entry *RegistryEntry) []string {
		return entry.Modelines
	})
}

func LanguageForMarkdownFence(value string) (RegisteredLanguage, bool) {
	return LanguageForAlias(value, func(entry *RegistryEntry) []string {
		return entry.MarkdownFences
	})
}

func DisplayLabelForCodeSyntax(codeSyntax string) (string, bool) {
	language, ok := LanguageForCodeSyntax(codeSyntax)
	if !ok {
		return "", false
	}
	return language.DisplayLabel, true
}

func LanguageForCodeSyntax(codeSyntax string) (RegisteredLanguage, bool) {
	normalized := normalize(codeSyntax)
	entries := data.AllLanguages()
	for i := range entries {
		entry := &entries[i]
		if entry.Language.CanonicalID == normalized {
			return entry.Language, true
		}
	}
	return RegisteredLanguage{}, false
}

func LanguageForAlias(
	value string,
	aliases func(*RegistryEntry) []string,
) (RegisteredLanguage, bool) {
	normalized := normalize(value)
	entries := data.AllLanguages()
	for i := range entries {
		entry := &entries[i]
		if contains(aliases(entry), normalized) {
			return entry.Language, true
		}
	}
	return RegisteredLanguage{}, false
}

func contains(values []string, value string) bool {
	return slices.Contains(values, value)
}

func normalize(values string) string {
	values = strings.ToLower(values)
	return strings.TrimSpace(values)
}

func isEnvName(name string) bool {
	return strings.HasPrefix(name, ".env") || strings.HasPrefix(name, "env.")
}
