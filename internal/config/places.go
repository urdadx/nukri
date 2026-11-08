// Package config defines the places shown by the file browser.
package config

type PlacesConfig struct {
	ShowDevices bool
	Entries     []PlaceEntrySpec
}

func DefaultPlacesConfig() PlacesConfig {
	return PlacesConfig{
		ShowDevices: true,
		Entries: []PlaceEntrySpec{
			builtinPlaceEntry(Home),
			builtinPlaceEntry(Desktop),
			builtinPlaceEntry(Documents),
			builtinPlaceEntry(Downloads),
			builtinPlaceEntry(Pictures),
			builtinPlaceEntry(Music),
			builtinPlaceEntry(Videos),
			builtinPlaceEntry(Root),
			builtinPlaceEntry(Trash),
		},
	}
}

type BuiltinPlace int

const (
	Home BuiltinPlace = iota
	Desktop
	Documents
	Downloads
	Pictures
	Music
	Videos
	Root
	Trash
)

type PlaceEntrySpec struct {
	Builtin *BuiltinPlace
	Title   string
	Path    string
	Icon    string
}

func builtinPlaceEntry(place BuiltinPlace) PlaceEntrySpec {
	return PlaceEntrySpec{Builtin: &place}
}
