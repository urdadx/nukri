package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/config"
)

func TestLoadSettingsWithoutFile(t *testing.T) {
	settings, err := config.LoadSettings(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil || settings.Theme != "" {
		t.Fatalf("settings = %#v, err = %v; want empty", settings, err)
	}
}

func TestSaveThemeCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nukri", "config.toml")
	if err := config.SaveTheme(path, "nord"); err != nil {
		t.Fatal(err)
	}
	settings, err := config.LoadSettings(path)
	if err != nil || settings.Theme != "nord" {
		t.Fatalf("settings = %#v, err = %v", settings, err)
	}
}

func TestSaveThemeKeepsTheRestOfTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "# my settings\ntheme = \"dracula\" # dark\nother = 1\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveTheme(path, "nord"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "# my settings\ntheme = \"nord\"\nother = 1\n" {
		t.Fatalf("config = %q", data)
	}
}

func TestSaveThemeAddsLineWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("# comment\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveTheme(path, "nord"); err != nil {
		t.Fatal(err)
	}
	if settings, err := config.LoadSettings(path); err != nil || settings.Theme != "nord" {
		t.Fatalf("settings = %#v, err = %v", settings, err)
	}
}

func TestPreviewIsShownUnlessTurnedOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if settings, _ := config.LoadSettings(path); !settings.PreviewShown() {
		t.Fatal("preview should be shown when never set")
	}
	if err := config.SavePreview(path, false); err != nil {
		t.Fatal(err)
	}
	if settings, err := config.LoadSettings(path); err != nil || settings.PreviewShown() {
		t.Fatalf("settings = %#v, err = %v; want the preview off", settings, err)
	}
	if err := config.SavePreview(path, true); err != nil {
		t.Fatal(err)
	}
	if settings, err := config.LoadSettings(path); err != nil || !settings.PreviewShown() {
		t.Fatalf("settings = %#v, err = %v; want the preview on", settings, err)
	}
}

// Saving one setting rewrites only its line, and a new setting goes ahead of
// any table so it stays a top-level key.
func TestSavePreviewKeepsOtherSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "# my settings\ntheme = \"nord\"\n\n[places]\nhome = true\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.SavePreview(path, false); err != nil {
		t.Fatal(err)
	}
	if err := config.SavePreview(path, false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "preview = false\n"+original {
		t.Fatalf("config = %q", data)
	}
	if settings, err := config.LoadSettings(path); err != nil || settings.Theme != "nord" || settings.PreviewShown() {
		t.Fatalf("settings = %#v, err = %v", settings, err)
	}
}
