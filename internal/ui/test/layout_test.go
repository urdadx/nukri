package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/preview"
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
	view := browser.Render(118, 27, &browser.VisualState{}, nil, styles, browser.Data{Selected: -1})
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
	view := browser.Render(100, 20, &browser.VisualState{}, nil, styles, data)
	if !strings.Contains(view, "first") || !strings.Contains(view, "second") {
		t.Fatal("browser panel discarded multiline entry content")
	}
}

func TestDirectoryRowsAndPreviewUseEntries(t *testing.T) {
	styles := browser.Styles{
		PanelFG: "#ffffff", PanelBG: "#000000", PanelBorder: "#555555", ActiveBorder: "#00aaff",
		Path: "#00aaff", Directory: "#00ff00", SelectedFG: "#ffffff", SelectedBG: "#222222", Muted: "#777777",
		SidebarFG: "#cccccc", SidebarBG: "#000000", SidebarTitle: "#00aaff", SidebarBorder: "#555555",
		SidebarSelectedFG: "#ffffff", SidebarSelectedBG: "#222222",
	}
	count := 3
	data := browser.Data{
		Selected: 0,
		Entries: []browser.Entry{{
			Entry: core.Entry{Name: "folder", Kind: core.Directory}, Icon: "d", ItemCount: &count,
		}},
		PreviewIsDir: true,
		PreviewEntries: []browser.Entry{
			{Entry: core.Entry{Name: "child", Kind: core.Directory}, Icon: "d", ItemCount: &count},
			{Entry: core.Entry{Name: "file.txt", Kind: core.File, Size: 3900}, Icon: "f"},
		},
	}
	view := browser.Render(100, 20, &browser.VisualState{}, nil, styles, data)
	for _, expected := range []string{"3 items", "child", "file.txt", "3.9 kB"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("directory view does not contain %q", expected)
		}
	}
	if strings.Contains(view, "| Mode") || strings.Contains(view, "+---") {
		t.Fatal("directory preview should not render an ASCII table")
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

func TestEntryCursorMovement(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"alpha.txt", "beta.txt"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(directory)
	colors := theme.Theme{
		FullScreenFG: "#ffffff", FullScreenBG: "#000000", FilePanelFG: "#ffffff", FilePanelBG: "#000000",
		FilePanelBorder: "#555555", FilePanelBorderActive: "#00aaff", FilePanelTopPath: "#00aaff",
		FilePanelItemSelectedFG: "#ffffff", FilePanelItemSelectedBG: "#222222", FooterFG: "#ffffff", FooterBG: "#000000",
		SidebarFG: "#ffffff", SidebarBG: "#000000", SidebarItemSelectedFG: "#ffffff", SidebarItemSelectedBG: "#222222", SidebarDivider: "#777777",
	}
	model := ui.New(colors)
	loaded, _ := model.Update(model.Init()())
	moved, command := loaded.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	if command == nil {
		t.Fatal("moving the cursor should request a new preview")
	}
	view := moved.(ui.Model).View()
	if !strings.Contains(view, "2/2  beta.txt") {
		t.Fatal("down key did not move selection to the second entry")
	}
	bounded, command := moved.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	if command != nil || !strings.Contains(bounded.(ui.Model).View(), "2/2  beta.txt") {
		t.Fatal("cursor should remain at the final entry")
	}
	up, _ := bounded.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyUp})
	if !strings.Contains(up.(ui.Model).View(), "1/2  alpha.txt") {
		t.Fatal("up key did not move selection to the first entry")
	}
}

func TestEnterSelectedEntry(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "inside.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	colors := theme.Theme{
		FullScreenFG: "#ffffff", FullScreenBG: "#000000", FilePanelFG: "#ffffff", FilePanelBG: "#000000",
		FilePanelBorder: "#555555", FilePanelBorderActive: "#00aaff", FilePanelTopPath: "#00aaff",
		FilePanelItemSelectedFG: "#ffffff", FilePanelItemSelectedBG: "#222222", FooterFG: "#ffffff", FooterBG: "#000000",
		SidebarFG: "#ffffff", SidebarBG: "#000000", SidebarDivider: "#777777",
	}
	model := ui.New(colors)
	loaded, _ := model.Update(model.Init()())
	navigating, directoryCommand := loaded.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if directoryCommand == nil {
		t.Fatal("entering a selected directory should start a directory load")
	}
	navigated, previewCommand := navigating.(ui.Model).Update(directoryCommand())
	view := navigated.(ui.Model).View()
	if !strings.Contains(view, "inside.txt") || !strings.Contains(view, "1/1  inside.txt") {
		t.Fatal("navigated view does not show child directory content")
	}
	if previewCommand == nil {
		t.Fatal("new directory selection should request a preview")
	}
	_, command := navigated.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("enter on a regular file should request opening it")
	}
	parenting, parentCommand := navigated.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if parentCommand == nil {
		t.Fatal("backspace should navigate to the parent directory")
	}
	parented, _ := parenting.(ui.Model).Update(parentCommand())
	if !strings.Contains(parented.(ui.Model).View(), "1/1  child") {
		t.Fatal("parent navigation did not reselect the directory that was exited")
	}
	historyBack, historyCommand := parented.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyLeft, Alt: true})
	if historyCommand == nil {
		t.Fatal("alt+left should navigate backward through directory history")
	}
	historyResult, _ := historyBack.(ui.Model).Update(historyCommand())
	if !strings.Contains(historyResult.(ui.Model).View(), "inside.txt") {
		t.Fatal("history back did not return to the child directory")
	}
	_, placeCommand := loaded.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	if placeCommand == nil {
		t.Fatal("tab should cycle to and activate a sidebar place")
	}
	_, clickCommand := loaded.(ui.Model).Update(tea.MouseMsg{X: 2, Y: 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if clickCommand == nil {
		t.Fatal("clicking a sidebar place should activate it")
	}
}

func TestSelectedEntryRemainsVisible(t *testing.T) {
	styles := browser.Styles{
		PanelFG: "#ffffff", PanelBG: "#000000", PanelBorder: "#555555", ActiveBorder: "#00aaff",
		Path: "#00aaff", SelectedFG: "#ffffff", SelectedBG: "#222222", Muted: "#777777", SidebarFG: "#cccccc",
	}
	entries := make([]browser.Entry, 8)
	for i := range entries {
		entries[i] = browser.Entry{Entry: core.Entry{Name: fmt.Sprintf("entry-%d", i), Kind: core.File}, Icon: "f"}
	}
	view := browser.RenderEntries(60, 10, styles, browser.Data{Entries: entries, Selected: 7})
	if !strings.Contains(view, "entry-7") || strings.Contains(view, "entry-0") {
		t.Fatal("entry viewport did not follow the selected row")
	}
}

func TestPreviewIsClippedAndOffset(t *testing.T) {
	styles := browser.Styles{
		PanelFG: "#ffffff", PanelBG: "#000000", PanelBorder: "#555555", Path: "#00aaff", Muted: "#777777",
	}
	previewLines := make([]string, 12)
	for i := range previewLines {
		previewLines[i] = fmt.Sprintf("line-%02d", i)
	}
	data := browser.Data{
		Selected: 0,
		Entries:  []browser.Entry{{Entry: core.Entry{Name: "sample.txt", Kind: core.File}, Icon: "f"}},
		Preview:  preview.View{Title: "Text", Lines: previewLines}, PreviewOffset: 5,
	}
	view := browser.RenderPreview(50, 8, 24, 3, &browser.VisualState{}, nil, styles, data)
	if got := lipgloss.Height(view); got != 8 {
		t.Fatalf("preview height = %d, want 8", got)
	}
	if strings.Contains(view, "line-00") || !strings.Contains(view, "line-03") {
		t.Fatal("preview did not render the requested scroll window")
	}
}

func TestPreviewPageDown(t *testing.T) {
	directory := t.TempDir()
	content := make([]string, 40)
	for i := range content {
		content[i] = fmt.Sprintf("line-%02d", i)
	}
	if err := os.WriteFile(filepath.Join(directory, "sample.txt"), []byte(strings.Join(content, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(directory)
	colors := theme.Theme{
		FullScreenFG: "#ffffff", FullScreenBG: "#000000", FilePanelFG: "#ffffff", FilePanelBG: "#000000",
		FilePanelBorder: "#555555", FilePanelBorderActive: "#00aaff", FilePanelTopPath: "#00aaff",
		FilePanelItemSelectedFG: "#ffffff", FilePanelItemSelectedBG: "#222222", FooterFG: "#ffffff", FooterBG: "#000000",
		SidebarFG: "#ffffff", SidebarBG: "#000000", SidebarDivider: "#777777",
	}
	model := ui.New(colors)
	resized, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	loaded, previewCommand := resized.(ui.Model).Update(model.Init()())
	previewed, _ := loaded.(ui.Model).Update(previewCommand())
	if !strings.Contains(previewed.(ui.Model).View(), "line-00") {
		t.Fatal("initial preview does not show its first line")
	}
	scrolled, _ := previewed.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyPgDown})
	view := scrolled.(ui.Model).View()
	if strings.Contains(view, "line-00") || !strings.Contains(view, "line-01") {
		t.Fatal("page down did not advance the preview")
	}
	mouseScrolled, mouseCommand := previewed.(ui.Model).Update(tea.MouseMsg{
		X: 90, Y: 5, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress,
	})
	if mouseCommand != nil {
		t.Fatal("mouse wheel should scroll immediately without scheduling frames")
	}
	mouseView := mouseScrolled.(ui.Model).View()
	if strings.Contains(mouseView, "Text") || !strings.Contains(mouseView, "line-06") {
		t.Fatal("mouse wheel did not immediately advance by the adaptive preview step")
	}
	tall, _ := previewed.(ui.Model).Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	tallScrolled, _ := tall.(ui.Model).Update(tea.MouseMsg{
		X: 90, Y: 5, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress,
	})
	tallView := tallScrolled.(ui.Model).View()
	if strings.Contains(tallView, "line-01") || !strings.Contains(tallView, "line-26") {
		t.Fatal("mouse wheel should use a larger step for a tall preview")
	}
	outside, _ := previewed.(ui.Model).Update(tea.MouseMsg{
		X: 5, Y: 5, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress,
	})
	if !strings.Contains(outside.(ui.Model).View(), "line-00") {
		t.Fatal("mouse wheel outside preview should not scroll it")
	}
}
