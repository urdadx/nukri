package registry

import "github.com/urdadx/nukri/internal/preview/code/registry/model"

type RegisteredLanguage = model.RegisteredLanguage

type RegistryEntry = model.RegistryEntry

type CodeBackend = model.CodeBackend

type StructuredFormat = model.StructuredFormat

const (
	Plain  = model.Plain
	Chroma = model.Chroma
	Custom = model.Custom

	StructuredJSON  = model.StructuredJSON
	StructuredJSONC = model.StructuredJSONC
	StructuredJSON5 = model.StructuredJSON5
	StructuredTOML  = model.StructuredTOML
	StructuredYAML  = model.StructuredYAML
)

var Language = model.Language

var Entry = model.Entry
