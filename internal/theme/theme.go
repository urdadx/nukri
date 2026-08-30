package theme

import (
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Theme mirrors the semantic colors exposed by Nukri's theme files.
type Theme struct {
	CodeSyntaxHighlight       string   `toml:"code_syntax_highlight"`
	FullScreenFG              string   `toml:"full_screen_fg"`
	FullScreenBG              string   `toml:"full_screen_bg"`
	GradientColor             []string `toml:"gradient_color"`
	DirectoryIconColor        string   `toml:"directory_icon_color"`
	FilePanelFG               string   `toml:"file_panel_fg"`
	FilePanelBG               string   `toml:"file_panel_bg"`
	FilePanelBorder           string   `toml:"file_panel_border"`
	FilePanelBorderActive     string   `toml:"file_panel_border_active"`
	FilePanelTopDirectoryIcon string   `toml:"file_panel_top_directory_icon"`
	FilePanelTopPath          string   `toml:"file_panel_top_path"`
	FilePanelItemSelectedFG   string   `toml:"file_panel_item_selected_fg"`
	FilePanelItemSelectedBG   string   `toml:"file_panel_item_selected_bg"`
	FooterFG                  string   `toml:"footer_fg"`
	FooterBG                  string   `toml:"footer_bg"`
	FooterBorder              string   `toml:"footer_border"`
	FooterBorderActive        string   `toml:"footer_border_active"`
	SidebarFG                 string   `toml:"sidebar_fg"`
	SidebarBG                 string   `toml:"sidebar_bg"`
	SidebarTitle              string   `toml:"sidebar_title"`
	SidebarBorder             string   `toml:"sidebar_border"`
	SidebarBorderActive       string   `toml:"sidebar_border_active"`
	SidebarItemSelectedFG     string   `toml:"sidebar_item_selected_fg"`
	SidebarItemSelectedBG     string   `toml:"sidebar_item_selected_bg"`
	SidebarDivider            string   `toml:"sidebar_divider"`
	ModalFG                   string   `toml:"modal_fg"`
	ModalBG                   string   `toml:"modal_bg"`
	ModalBorderActive         string   `toml:"modal_border_active"`
	ModalCancelFG             string   `toml:"modal_cancel_fg"`
	ModalCancelBG             string   `toml:"modal_cancel_bg"`
	ModalConfirmFG            string   `toml:"modal_confirm_fg"`
	ModalConfirmBG            string   `toml:"modal_confirm_bg"`
	HelpMenuHotkey            string   `toml:"help_menu_hotkey"`
	HelpMenuTitle             string   `toml:"help_menu_title"`
	Cursor                    string   `toml:"cursor"`
	Correct                   string   `toml:"correct"`
	Error                     string   `toml:"error"`
	Hint                      string   `toml:"hint"`
	Cancel                    string   `toml:"cancel"`
}

func Load(path string) (Theme, error) {
	var value Theme
	if _, err := toml.DecodeFile(filepath.Clean(path), &value); err != nil {
		return Theme{}, fmt.Errorf("load theme %q: %w", path, err)
	}
	if value.FullScreenFG == "" || value.FullScreenBG == "" {
		return Theme{}, fmt.Errorf("load theme %q: full_screen_fg and full_screen_bg are required", path)
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
