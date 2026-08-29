/**
 * Package config defines the places shown by the file browser, resolves partial
 * configuration overrides, validates built-in and custom entries, and expands
 * custom paths that begin with the invoking user's home directory.
 */
package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

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

type PlacesConfigOverride struct {
	ShowDevices *bool `toml:"show_devices"`
	Entries     []any `toml:"entries"`
}

func (o PlacesConfigOverride) Resolve(defaults PlacesConfig) PlacesConfig {
	resolved := PlacesConfig{
		ShowDevices: defaults.ShowDevices,
		Entries:     append([]PlaceEntrySpec(nil), defaults.Entries...),
	}
	if o.ShowDevices != nil {
		resolved.ShowDevices = *o.ShowDevices
	}
	if o.Entries != nil {
		resolved.Entries = make([]PlaceEntrySpec, 0, len(o.Entries))
		for index, value := range o.Entries {
			entry, ok := placeEntryFromValue(value, fmt.Sprintf("places.entries[%d]", index))
			if ok {
				resolved.Entries = append(resolved.Entries, entry)
			}
		}
	}
	return resolved
}

func builtinPlaceEntry(place BuiltinPlace) PlaceEntrySpec {
	return PlaceEntrySpec{Builtin: &place}
}

func placeEntryFromValue(value any, fieldName string) (PlaceEntrySpec, bool) {
	switch value := value.(type) {
	case string:
		place, ok := parseBuiltinPlace(value)
		if !ok {
			return PlaceEntrySpec{}, false
		}
		return builtinPlaceEntry(place), true
	case map[string]any:
		icon := parsePlaceIcon(value["icon"], fieldName)
		if builtin, exists := value["builtin"]; exists {
			name, ok := nonEmptyString(builtin)
			if !ok {
				log.Printf("nukri: %s: builtin places require a non-empty string builtin name; skipping entry", fieldName)
				return PlaceEntrySpec{}, false
			}
			place, ok := parseBuiltinPlace(name)
			if !ok {
				return PlaceEntrySpec{}, false
			}
			if _, hasTitle := value["title"]; hasTitle || value["path"] != nil {
				log.Printf("nukri: %s: builtin places only support { builtin, icon }; ignoring extra fields", fieldName)
			}
			entry := builtinPlaceEntry(place)
			entry.Icon = icon
			return entry, true
		}

		title, ok := nonEmptyString(value["title"])
		if !ok {
			log.Printf("nukri: %s: custom places require a non-empty string title; skipping entry", fieldName)
			return PlaceEntrySpec{}, false
		}
		path, ok := nonEmptyString(value["path"])
		if !ok {
			log.Printf("nukri: %s: custom places require a non-empty string path; skipping entry", fieldName)
			return PlaceEntrySpec{}, false
		}
		expanded, err := ExpandCustomPlacePath(path)
		if err != nil {
			log.Printf("nukri: %s: %v; skipping entry", fieldName, err)
			return PlaceEntrySpec{}, false
		}
		return PlaceEntrySpec{Title: title, Path: expanded, Icon: icon}, true
	default:
		log.Printf("nukri: %s: expected a built-in name, { builtin, icon? }, or { title, path, icon? } object; skipping entry", fieldName)
		return PlaceEntrySpec{}, false
	}
}

func parsePlaceIcon(value any, fieldName string) string {
	if value == nil {
		return ""
	}
	icon, ok := value.(string)
	if !ok {
		log.Printf("nukri: %s: icon must be a string; using default", fieldName)
		return ""
	}
	icon = strings.TrimSpace(icon)
	if icon == "" {
		log.Printf("nukri: %s: icon must be a non-empty string; using default", fieldName)
	}
	return icon
}

func nonEmptyString(value any) (string, bool) {
	text, ok := value.(string)
	text = strings.TrimSpace(text)
	return text, ok && text != ""
}

func parseBuiltinPlace(name string) (BuiltinPlace, bool) {
	places := map[string]BuiltinPlace{
		"home": Home, "desktop": Desktop, "documents": Documents,
		"downloads": Downloads, "pictures": Pictures, "music": Music,
		"videos": Videos, "root": Root, "trash": Trash,
	}
	place, ok := places[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		log.Printf("nukri: unknown places entry %q; expected one of: home, desktop, documents, downloads, pictures, music, videos, root, trash (use semantic ids like %q, not localized folder names)", name, "downloads")
	}
	return place, ok
}

func ExpandCustomPlacePath(path string) (string, error) {
	expanded := path
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not resolve home directory: %w", err)
		}
		expanded = home
		if path != "~" {
			expanded = filepath.Join(home, path[2:])
		}
	}
	if !filepath.IsAbs(expanded) {
		return "", fmt.Errorf("custom place paths must be absolute or start with ~/")
	}
	return filepath.Clean(expanded), nil
}
