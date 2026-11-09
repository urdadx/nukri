package test

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
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

func TestEveryBundledThemeLoads(t *testing.T) {
	files, err := fs.Glob(nukriconfig.Themes, "themes/*.toml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no bundled themes: %v", err)
	}
	for _, file := range files {
		name := strings.TrimSuffix(path.Base(file), ".toml")
		if _, err := theme.Named(name, "", nukriconfig.Themes); err != nil {
			t.Errorf("theme %s: %v", name, err)
		}
	}
}

func TestNamesListsBundledAndUserThemes(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"mine.toml", "nord.toml", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(directory, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	names := theme.Names(directory, nukriconfig.Themes)
	count := map[string]int{}
	for _, name := range names {
		count[name]++
	}
	if count["mine"] != 1 || count["nord"] != 1 || count["dracula"] != 1 || count["notes"] != 0 {
		t.Fatalf("names = %v", names)
	}
	if !slices.IsSorted(names) {
		t.Fatalf("names are not sorted: %v", names)
	}
}

// A selected row whose text matches its background is unreadable; loading the
// theme must swap in a readable text color, and leave readable pairs alone.
func TestThemeKeepsSelectedRowsReadable(t *testing.T) {
	directory := t.TempDir()
	write := func(name, selectedFG, selectedBG string) theme.Theme {
		t.Helper()
		content := "full_screen_fg = \"#db7ddd\"\nfull_screen_bg = \"#111147\"\n" +
			"file_panel_fg = \"#db7ddd\"\nfile_panel_bg = \"#111147\"\n" +
			"file_panel_item_selected_fg = \"" + selectedFG + "\"\nfile_panel_item_selected_bg = \"" + selectedBG + "\"\n" +
			"sidebar_item_selected_fg = \"" + selectedFG + "\"\nsidebar_item_selected_bg = \"" + selectedBG + "\"\n"
		if err := os.WriteFile(filepath.Join(directory, name+".toml"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		loaded, err := theme.Named(name, directory, nukriconfig.Themes)
		if err != nil {
			t.Fatal(err)
		}
		return loaded
	}
	clash := write("clash", "#53b397", "#53b397")
	if clash.FilePanelItemSelectedFG != "#111147" || clash.SidebarItemSelectedFG != "#111147" {
		t.Fatalf("selected text = %q / %q, want the readable panel background #111147", clash.FilePanelItemSelectedFG, clash.SidebarItemSelectedFG)
	}
	fine := write("fine", "#ffffff", "#111147")
	if fine.FilePanelItemSelectedFG != "#ffffff" {
		t.Fatalf("a readable pair was changed to %q", fine.FilePanelItemSelectedFG)
	}
	// An unset background shows the panel's, so contrast is judged against it.
	unset := write("unset", "#d65d0e", "")
	if unset.FilePanelItemSelectedFG != "#d65d0e" {
		t.Fatalf("a readable color on an unset background was changed to %q", unset.FilePanelItemSelectedFG)
	}
}

// When the selected row shares the panel background, its text color is what
// marks it; a replacement must not be the panel's normal text color.
func TestThemeSelectionStaysDistinctFromNormalRows(t *testing.T) {
	directory := t.TempDir()
	content := "full_screen_fg = \"#4c4f69\"\nfull_screen_bg = \"#eff1f5\"\nfile_panel_fg = \"#4c4f69\"\nfile_panel_bg = \"#eff1f5\"\n" +
		"file_panel_item_selected_fg = \"#e8eaef\"\nfile_panel_item_selected_bg = \"#eff1f5\"\n"
	if err := os.WriteFile(filepath.Join(directory, "pale.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := theme.Named("pale", directory, nukriconfig.Themes)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.FilePanelItemSelectedFG; got == "#e8eaef" || got == "#4c4f69" {
		t.Fatalf("selected text = %q: unreadable, or the same as normal rows", got)
	}
}
