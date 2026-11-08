package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/theme"
	nukriconfig "github.com/urdadx/nukri/nukri_config"
)

func TestBundledThemeLoads(t *testing.T) {
	path := filepath.Join("..", "..", "..", "nukri_config", "themes", "catppuccin-mocha.toml")
	loaded, err := theme.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FilePanelBG == "" || loaded.SidebarFG == "" || loaded.FooterFG == "" {
		t.Fatalf("theme did not load pane colors: %#v", loaded)
	}
	if loaded.DirectoryIconColor == "" {
		t.Fatal("empty directory icon color should inherit the panel directory color")
	}
}

func TestNamedThemeUsesBundledThemeAnywhere(t *testing.T) {
	t.Chdir(t.TempDir()) // themes must not depend on the working directory
	loaded, err := theme.Named("nord", "", nukriconfig.Themes)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FullScreenBG == "" {
		t.Fatalf("bundled theme did not load: %#v", loaded)
	}
}

func TestNamedThemePrefersUserTheme(t *testing.T) {
	directory := t.TempDir()
	custom := "full_screen_fg = \"#010101\"\nfull_screen_bg = \"#020202\"\n"
	if err := os.WriteFile(filepath.Join(directory, "nord.toml"), []byte(custom), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := theme.Named("nord", directory, nukriconfig.Themes)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FullScreenFG != "#010101" {
		t.Fatalf("user theme was not used: %q", loaded.FullScreenFG)
	}
}

func TestNamedThemeRejectsPaths(t *testing.T) {
	for _, name := range []string{"", "..", "../../etc/passwd", "a/b", "missing"} {
		if _, err := theme.Named(name, t.TempDir(), nukriconfig.Themes); err == nil {
			t.Errorf("Named(%q) succeeded", name)
		}
	}
}
