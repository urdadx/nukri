package model

type CodeBackend int

const (
	Plain CodeBackend = iota
	Chroma
	Custom
)

type StructuredFormat string

const (
	StructuredJSON  StructuredFormat = "json"
	StructuredJSONC StructuredFormat = "jsonc"
	StructuredJSON5 StructuredFormat = "json5"
	StructuredTOML  StructuredFormat = "toml"
	StructuredYAML  StructuredFormat = "yaml"
)

type RegisteredLanguage struct {
	CanonicalID      string
	DisplayLabel     string
	Backend          CodeBackend
	StructuredFormat *StructuredFormat
}

type RegistryEntry struct {
	Language            RegisteredLanguage
	Extensions          []string
	ExactFilenames      []string
	ShebangInterpreters []string
	Modelines           []string
	MarkdownFences      []string
}

func Language(canonicalID, displayLabel string, backend CodeBackend, structuredFormat *StructuredFormat) RegisteredLanguage {
	return RegisteredLanguage{
		CanonicalID:      canonicalID,
		DisplayLabel:     displayLabel,
		Backend:          backend,
		StructuredFormat: structuredFormat,
	}
}

func Entry(
	language RegisteredLanguage,
	extensions, exactFilenames, shebangInterpreters, modelines, markdownFences []string,
) RegistryEntry {
	return RegistryEntry{
		Language:            language,
		Extensions:          extensions,
		ExactFilenames:      exactFilenames,
		ShebangInterpreters: shebangInterpreters,
		Modelines:           modelines,
		MarkdownFences:      markdownFences,
	}
}
