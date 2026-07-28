package data

import (
	registry "github.com/urdadx/nukri/internal/preview/code/registry/model"
)

var Groups = [][]registry.RegistryEntry{
	Languages,
	Web,
	Tooling,
	Shell,
	Formats,
}

func AllLanguages() []registry.RegistryEntry {
	var all []registry.RegistryEntry
	for _, group := range Groups {
		all = append(all, group...)
	}
	return all
}
