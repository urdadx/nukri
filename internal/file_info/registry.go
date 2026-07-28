package fileinfo

import "github.com/urdadx/nukri/internal/preview/code/registry"

func previewForLanguage(language registry.RegisteredLanguage) PreviewSpec {
	backend := Plain
	switch language.Backend {
	case registry.Chroma:
		backend = Chroma
	case registry.Custom:
		backend = Custom
	}

	var structuredFormat *StructuredFormat
	if language.StructuredFormat != nil {
		format := StructuredFormat(*language.StructuredFormat)
		structuredFormat = &format
	}
	return CodePreview(language.CanonicalID, backend, structuredFormat)
}
