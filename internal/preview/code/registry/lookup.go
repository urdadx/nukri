package registry

import (
	"fmt"
	"strings"
	"sync"

	"github.com/urdadx/nukri/internal/preview/code/registry/data"
)

// index maps every alias kind to its language. It is built once because lookups
// run for every listed file.
type index struct {
	canonical  map[string]RegisteredLanguage
	extensions map[string]RegisteredLanguage
	filenames  map[string]RegisteredLanguage
	shebangs   map[string]RegisteredLanguage
	modelines  map[string]RegisteredLanguage
	fences     map[string]RegisteredLanguage
}

var languageIndex = sync.OnceValue(func() index {
	idx := index{
		canonical:  map[string]RegisteredLanguage{},
		extensions: map[string]RegisteredLanguage{},
		filenames:  map[string]RegisteredLanguage{},
		shebangs:   map[string]RegisteredLanguage{},
		modelines:  map[string]RegisteredLanguage{},
		fences:     map[string]RegisteredLanguage{},
	}
	entries := data.AllLanguages()
	for i := range entries {
		entry := &entries[i]
		addFirst(idx.canonical, entry.Language, entry.Language.CanonicalID)
		addFirst(idx.extensions, entry.Language, entry.Extensions...)
		addFirst(idx.filenames, entry.Language, entry.ExactFilenames...)
		addFirst(idx.shebangs, entry.Language, entry.ShebangInterpreters...)
		addFirst(idx.modelines, entry.Language, entry.Modelines...)
		addFirst(idx.fences, entry.Language, entry.MarkdownFences...)
	}
	return idx
})

// addFirst keeps the earliest registration, matching registry order.
func addFirst(values map[string]RegisteredLanguage, language RegisteredLanguage, keys ...string) {
	for _, key := range keys {
		if _, exists := values[key]; !exists {
			values[key] = language
		}
	}
}

func lookup(values map[string]RegisteredLanguage, value string) (RegisteredLanguage, bool) {
	language, ok := values[normalize(value)]
	return language, ok
}

func LanguageForExtension(value string) (RegisteredLanguage, bool) {
	return lookup(languageIndex().extensions, value)
}

func LanguageForExactFilename(value string) (RegisteredLanguage, bool) {
	normalized := normalize(value)
	if isEnvName(normalized) {
		return LanguageForCodeSyntax("dotenv")
	}
	return lookup(languageIndex().filenames, normalized)
}

func LanguageForShebangInterpreter(value string) (RegisteredLanguage, bool) {
	return lookup(languageIndex().shebangs, value)
}

func LanguageForModeline(value string) (RegisteredLanguage, bool) {
	return lookup(languageIndex().modelines, value)
}

func LanguageForMarkdownFence(value string) (RegisteredLanguage, bool) {
	return lookup(languageIndex().fences, value)
}

func LanguageForCodeSyntax(codeSyntax string) (RegisteredLanguage, bool) {
	return lookup(languageIndex().canonical, codeSyntax)
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
