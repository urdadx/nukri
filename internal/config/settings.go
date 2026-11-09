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
	// Preview is whether the preview pane is shown; nil when never set.
	Preview *bool `toml:"preview"`
}

// PreviewShown reports whether the preview pane starts shown: yes unless the
// user turned it off.
func (s Settings) PreviewShown() bool { return s.Preview == nil || *s.Preview }

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

// SaveTheme records name as the theme in config.toml at path.
func SaveTheme(path, name string) error {
	return saveSetting(path, "theme", fmt.Sprintf("%q", name))
}

// SavePreview records whether the preview pane is shown in config.toml at path.
func SavePreview(path string, shown bool) error {
	return saveSetting(path, "preview", fmt.Sprint(shown))
}

/*
saveSetting sets key to value, a TOML value, in config.toml at path. Only that
key's line is rewritten, so the rest of the file (comments included) is left
alone. A new key goes first, ahead of any table. The file and its folder are
created when missing.
*/
func saveSetting(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read config: %w", err)
	}
	keyLine := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(key) + `[ \t]*=.*$`)
	line := key + " = " + value
	var updated string
	if keyLine.Match(data) {
		updated = keyLine.ReplaceAllLiteralString(string(data), line)
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
