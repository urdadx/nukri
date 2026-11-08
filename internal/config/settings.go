package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
)

// Settings is the user's config.toml.
type Settings struct {
	Theme string `toml:"theme"`
}

// LoadSettings reads config.toml at path. A missing file gives empty settings.
func LoadSettings(path string) (Settings, error) {
	var settings Settings
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, fmt.Errorf("read config: %w", err)
	}
	if _, err := toml.Decode(string(data), &settings); err != nil {
		return settings, fmt.Errorf("parse %s: %w", path, err)
	}
	return settings, nil
}

var themeLine = regexp.MustCompile(`(?m)^[ \t]*theme[ \t]*=.*$`)

/*
SaveTheme records name as the theme in config.toml at path. Only the theme line
is rewritten, so the rest of the file (comments included) is left alone; the
file and its folder are created when missing.
*/
func SaveTheme(path, name string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read config: %w", err)
	}
	line := fmt.Sprintf("theme = %q", name)
	var updated string
	if themeLine.Match(data) {
		updated = themeLine.ReplaceAllLiteralString(string(data), line)
	} else {
		updated = line + "\n" + string(data)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".config.toml.tmp-*")
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	_, err = file.WriteString(updated)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), path)
	}
	if err != nil {
		_ = os.Remove(file.Name())
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}
