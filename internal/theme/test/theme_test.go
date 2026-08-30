package test

import (
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/theme"
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
