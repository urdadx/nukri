package registry

import (
	fileinfo "github.com/urdadx/nukri/internal/file_info"
	"github.com/urdadx/nukri/internal/preview/code/registry/model"
)

type RegisteredLanguage = model.RegisteredLanguage

type RegistryEntry = model.RegistryEntry

func Language(
	canonicalID string,
	displayLabel string,
	backend fileinfo.CodeBackend,
	structuredFormat *fileinfo.StructuredFormat,
) RegisteredLanguage {
	return model.Language(canonicalID, displayLabel, backend, structuredFormat)
}

func Entry(
	language RegisteredLanguage,
	extensions []string,
	exactFilenames []string,
	shebangInterpreters []string,
	modelines []string,
	markdownFences []string,
) RegistryEntry {
	return model.Entry(language, extensions, exactFilenames, shebangInterpreters, modelines, markdownFences)
}
