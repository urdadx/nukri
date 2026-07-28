package registry

import (
	"fmt"
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
	if name == ".env" || name == "env" {
		return true
	}
	var suffix string
	if after, ok := strings.CutPrefix(name, ".env."); ok {
		suffix = after
	} else if after, ok := strings.CutPrefix(name, "env."); ok {
		suffix = after
	} else {
		return false
	}
	if suffix == "" {
		return false
	}
	_, knownExtension := LanguageForExtension(suffix)
	return !knownExtension
}

func Validate() error {
	entries := data.AllLanguages()
	canonicalIDs := make(map[string]struct{}, len(entries))
	aliasGroups := []struct {
		name    string
		aliases func(*RegistryEntry) []string
	}{
		{"extension", func(entry *RegistryEntry) []string { return entry.Extensions }},
		{"exact filename", func(entry *RegistryEntry) []string { return entry.ExactFilenames }},
		{"shebang interpreter", func(entry *RegistryEntry) []string { return entry.ShebangInterpreters }},
		{"modeline", func(entry *RegistryEntry) []string { return entry.Modelines }},
		{"Markdown fence", func(entry *RegistryEntry) []string { return entry.MarkdownFences }},
	}

	for i := range entries {
		entry := &entries[i]
		id := normalize(entry.Language.CanonicalID)
		if id == "" {
			return fmt.Errorf("registry contains an empty canonical ID")
		}
		if _, exists := canonicalIDs[id]; exists {
			return fmt.Errorf("duplicate canonical ID %q", id)
		}
		canonicalIDs[id] = struct{}{}
	}

	for _, group := range aliasGroups {
		owners := make(map[string]string)
		for i := range entries {
			entry := &entries[i]
			for _, rawAlias := range group.aliases(entry) {
				alias := normalize(rawAlias)
				if alias == "" {
					return fmt.Errorf("%s alias for %q is empty", group.name, entry.Language.CanonicalID)
				}
				if owner, exists := owners[alias]; exists && owner != entry.Language.CanonicalID {
					return fmt.Errorf("duplicate %s alias %q for %q and %q", group.name, alias, owner, entry.Language.CanonicalID)
				}
				owners[alias] = entry.Language.CanonicalID
			}
		}
	}
	return nil
}
