package data

import (
	"sync"

	registry "github.com/urdadx/nukri/internal/preview/code/registry/model"
)

var Groups = [][]registry.RegistryEntry{
	Languages,
	Web,
	Tooling,
	Shell,
	Formats,
}

// AllLanguages returns every registry entry in group order. The slice is shared
// and must not be modified.
var AllLanguages = sync.OnceValue(func() []registry.RegistryEntry {
	var all []registry.RegistryEntry
	for _, group := range Groups {
		all = append(all, group...)
	}
	return all
})
