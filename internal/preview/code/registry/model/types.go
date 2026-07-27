package model

import fileinfo "github.com/urdadx/nukri/internal/file_info"

type RegisteredLanguage struct {
	CanonicalID      string
	DisplayLabel     string
	Backend          fileinfo.CodeBackend
	StructuredFormat *fileinfo.StructuredFormat
}

func (l RegisteredLanguage) PreviewSpec() fileinfo.PreviewSpec {
	return fileinfo.CodePreview(l.CanonicalID, l.Backend, l.StructuredFormat)
}

type RegistryEntry struct {
	Language            RegisteredLanguage
	Extensions          []string
	ExactFilenames      []string
	ShebangInterpreters []string
	Modelines           []string
	MarkdownFences      []string
}

func Language(canonicalID, displayLabel string, backend fileinfo.CodeBackend, structuredFormat *fileinfo.StructuredFormat) RegisteredLanguage {
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
