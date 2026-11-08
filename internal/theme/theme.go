package theme

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// Theme mirrors the semantic colors exposed by Nukri's theme files.
type Theme struct {
	CodeSyntaxHighlight       string `toml:"code_syntax_highlight"`
	FullScreenFG              string `toml:"full_screen_fg"`
	FullScreenBG              string `toml:"full_screen_bg"`
	DirectoryIconColor        string `toml:"directory_icon_color"`
	FilePanelFG               string `toml:"file_panel_fg"`
	FilePanelBG               string `toml:"file_panel_bg"`
	FilePanelBorder           string `toml:"file_panel_border"`
	FilePanelBorderActive     string `toml:"file_panel_border_active"`
	FilePanelTopDirectoryIcon string `toml:"file_panel_top_directory_icon"`
	FilePanelTopPath          string `toml:"file_panel_top_path"`
	FilePanelItemSelectedFG   string `toml:"file_panel_item_selected_fg"`
	FilePanelItemSelectedBG   string `toml:"file_panel_item_selected_bg"`
	FooterFG                  string `toml:"footer_fg"`
	FooterBG                  string `toml:"footer_bg"`
	SidebarFG                 string `toml:"sidebar_fg"`
	SidebarBG                 string `toml:"sidebar_bg"`
	SidebarTitle              string `toml:"sidebar_title"`
	SidebarBorder             string `toml:"sidebar_border"`
	SidebarItemSelectedFG     string `toml:"sidebar_item_selected_fg"`
	SidebarItemSelectedBG     string `toml:"sidebar_item_selected_bg"`
	SidebarDivider            string `toml:"sidebar_divider"`
	ModalFG                   string `toml:"modal_fg"`
	ModalBG                   string `toml:"modal_bg"`
	ModalBorderActive         string `toml:"modal_border_active"`
	Correct                   string `toml:"correct"`
	Cancel                    string `toml:"cancel"`
}

func Load(path string) (Theme, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Theme{}, fmt.Errorf("load theme %q: %w", path, err)
	}
	return parse(data, path)
}

/*
Named loads the theme called name. A file <name>.toml in userDirectory (the
user's theme folder) wins; otherwise the theme comes from bundled, which holds
themes/<name>.toml. A missing user folder is not an error.
*/
func Named(name, userDirectory string, bundled fs.FS) (Theme, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return Theme{}, fmt.Errorf("invalid theme name %q", name)
	}
	if userDirectory != "" {
		path := filepath.Join(userDirectory, name+".toml")
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}
	data, err := fs.ReadFile(bundled, "themes/"+name+".toml")
	if errors.Is(err, fs.ErrNotExist) {
		return Theme{}, fmt.Errorf("theme %q not found", name)
	}
	if err != nil {
		return Theme{}, fmt.Errorf("load theme %q: %w", name, err)
	}
	return parse(data, name)
}

// Names lists the themes Named can load: the bundled ones and any <name>.toml
// in userDirectory, sorted and without duplicates.
func Names(userDirectory string, bundled fs.FS) []string {
	seen := map[string]bool{}
	add := func(file string) {
		if name, ok := strings.CutSuffix(filepath.Base(file), ".toml"); ok && name != "" {
			seen[name] = true
		}
	}
	if files, err := fs.Glob(bundled, "themes/*.toml"); err == nil {
		for _, file := range files {
			add(file)
		}
	}
	if userDirectory != "" {
		if files, err := filepath.Glob(filepath.Join(userDirectory, "*.toml")); err == nil {
			for _, file := range files {
				add(file)
			}
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

func parse(data []byte, source string) (Theme, error) {
	var value Theme
	if _, err := toml.Decode(string(data), &value); err != nil {
		return Theme{}, fmt.Errorf("load theme %q: %w", source, err)
	}
	if value.FullScreenFG == "" || value.FullScreenBG == "" {
		return Theme{}, fmt.Errorf("load theme %q: full_screen_fg and full_screen_bg are required", source)
	}
	value.resolveEmptyColors()
	return value, nil
}

func (t *Theme) resolveEmptyColors() {
	if t.DirectoryIconColor == "" {
		t.DirectoryIconColor = t.FilePanelTopDirectoryIcon
	}
	if t.SidebarBorder == "" {
		t.SidebarBorder = t.FilePanelBorder
	}
	if t.FooterBG == "" {
		t.FooterBG = t.FullScreenBG
	}
}
