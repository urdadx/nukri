package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/theme"
	"github.com/urdadx/nukri/internal/ui"
	"github.com/urdadx/nukri/internal/ui/browser"
)

func TestResponsiveLayouts(t *testing.T) {
	wide := browser.ResolveLayout(120, 30)
	if wide.SidebarWidth == 0 || wide.FilesWidth == 0 || wide.PreviewWidth == 0 || wide.Stacked {
		t.Fatalf("wide layout should contain three horizontal panes: %#v", wide)
	}

	narrow := browser.ResolveLayout(50, 24)
	if !narrow.Stacked || narrow.SidebarWidth == 0 || narrow.PreviewHeight == 0 {
		t.Fatalf("narrow layout should stack files and preview: %#v", narrow)
	}

	minimal := browser.ResolveLayout(30, 20)
	if minimal.FilesWidth != 30 || minimal.SidebarWidth != 0 || minimal.PreviewWidth != 0 {
		t.Fatalf("minimal layout should only show files: %#v", minimal)
	}
}

func TestShellFitsTerminal(t *testing.T) {
	colors := theme.Theme{
		FullScreenFG: "#ffffff", FullScreenBG: "#000000", FilePanelFG: "#ffffff", FilePanelBG: "#000000",
		FilePanelBorder: "#555555", FilePanelBorderActive: "#00aaff", FilePanelTopPath: "#00aaff",
		FilePanelItemSelectedFG: "#ffffff", FilePanelItemSelectedBG: "#222222", FooterFG: "#ffffff", FooterBG: "#000000",
		FooterBorderActive: "#00aaff", SidebarFG: "#ffffff", SidebarBG: "#000000", SidebarTitle: "#00aaff",
		SidebarBorder: "#555555", SidebarItemSelectedFG: "#ffffff", SidebarItemSelectedBG: "#222222", SidebarDivider: "#777777",
	}
	model := ui.New(colors)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 28})
	view := updated.(ui.Model).View()
	if got := lipgloss.Width(view); got != 100 {
		t.Fatalf("view width = %d, want 100", got)
	}
	if got := lipgloss.Height(view); got != 28 {
		t.Fatalf("view height = %d, want 28", got)
	}
}

func TestBrowserFillsAllocatedWidth(t *testing.T) {
	styles := browser.Styles{
		PanelFG: "#ffffff", PanelBG: "#000000", PanelBorder: "#555555", ActiveBorder: "#00aaff",
		Path: "#00aaff", SelectedFG: "#ffffff", SelectedBG: "#222222", Muted: "#777777",
		SidebarFG: "#ffffff", SidebarBG: "#000000", SidebarTitle: "#00aaff", SidebarBorder: "#555555",
		SidebarSelectedFG: "#ffffff", SidebarSelectedBG: "#222222",
	}
	view := browser.Render(118, 27, styles, browser.Data{Selected: -1})
	if got := lipgloss.Width(view); got != 118 {
		t.Fatalf("browser width = %d, want 118", got)
	}
}

func TestPanelPreservesMultilineContent(t *testing.T) {
	styles := browser.Styles{
		PanelFG: "#ffffff", PanelBG: "#000000", PanelBorder: "#555555", ActiveBorder: "#00aaff",
		Path: "#00aaff", SelectedFG: "#ffffff", SelectedBG: "#222222", Muted: "#777777",
		SidebarFG: "#ffffff", SidebarBG: "#000000", SidebarTitle: "#00aaff", SidebarBorder: "#555555",
		SidebarSelectedFG: "#ffffff", SidebarSelectedBG: "#222222",
	}
	data := browser.Data{Selected: 0, Entries: []browser.Entry{
		{Entry: core.Entry{Name: "first", Kind: core.File}, Icon: "f"},
		{Entry: core.Entry{Name: "second", Kind: core.File}, Icon: "f"},
	}}
	view := browser.Render(100, 20, styles, data)
	if !strings.Contains(view, "first") || !strings.Contains(view, "second") {
		t.Fatal("browser panel discarded multiline entry content")
	}
}

func TestInitialLoadRendersFilesystem(t *testing.T) {
	colors := theme.Theme{
		FullScreenFG: "#ffffff", FullScreenBG: "#000000", FilePanelFG: "#ffffff", FilePanelBG: "#000000",
		FilePanelBorder: "#555555", FilePanelBorderActive: "#00aaff", FilePanelTopPath: "#00aaff",
		FilePanelItemSelectedFG: "#ffffff", FilePanelItemSelectedBG: "#222222", FooterFG: "#ffffff", FooterBG: "#000000",
		SidebarFG: "#ffffff", SidebarBG: "#000000", SidebarItemSelectedFG: "#ffffff", SidebarItemSelectedBG: "#222222", SidebarDivider: "#777777",
	}
	model := ui.New(colors)
	message := model.Init()()
	updated, previewCommand := model.Update(message)
	if previewCommand != nil {
		updated, _ = updated.(ui.Model).Update(previewCommand())
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	view := updated.(ui.Model).View()
	if !strings.Contains(view, filepath.Base(cwd)) || !strings.Contains(view, "layout_test.go") {
		t.Fatalf("loaded view does not contain current filesystem content")
	}
}
