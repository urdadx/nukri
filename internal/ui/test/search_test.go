package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/theme"
	"github.com/urdadx/nukri/internal/ui"
)

func searchTheme() theme.Theme {
	return theme.Theme{
		FullScreenFG: "#ffffff", FullScreenBG: "#000000", FilePanelFG: "#ffffff", FilePanelBG: "#000000",
		FilePanelBorder: "#555555", FilePanelBorderActive: "#00aaff", FilePanelTopPath: "#00aaff",
		FilePanelItemSelectedFG: "#ffffff", FilePanelItemSelectedBG: "#222222", FooterFG: "#ffffff", FooterBG: "#000000",
		SidebarFG: "#ffffff", SidebarBG: "#000000", SidebarDivider: "#777777", ModalFG: "#ffffff", ModalBG: "#111111",
		ModalBorderActive: "#00aaff",
	}
}

func openAndLoadSearch(t testing.TB, model ui.Model) ui.Model {
	t.Helper()
	searching, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if command == nil {
		t.Fatal("opening search should start the search worker")
	}
	loaded, next := searching.(ui.Model).Update(command())
	for next != nil {
		loaded, next = loaded.(ui.Model).Update(next())
	}
	return loaded.(ui.Model)
}

func TestSearchFiltersCurrentDirectoryAndCloses(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"alpha.txt", "Beta.md"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(directory)
	model := ui.New(searchTheme())
	loaded, _ := model.Update(model.Init()())
	searching := openAndLoadSearch(t, loaded.(ui.Model))
	filtered, _ := searching.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bet")})
	view := filtered.(ui.Model).View()
	if !strings.Contains(view, "Find in current tree") || !strings.Contains(view, "Beta.md") {
		t.Fatalf("search dialog did not show the matching entry")
	}
	closed, _ := filtered.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if strings.Contains(closed.(ui.Model).View(), "Find in current tree") {
		t.Fatalf("escape did not close search")
	}
}

func TestSearchEnterNavigatesToDirectory(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "documents")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "inside.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	model := ui.New(searchTheme())
	loaded, _ := model.Update(model.Init()())
	searching := openAndLoadSearch(t, loaded.(ui.Model))
	filtered, _ := searching.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("doc")})
	navigating, command := filtered.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("entering a directory search result should load it")
	}
	navigated, _ := navigating.(ui.Model).Update(command())
	view := navigated.(ui.Model).View()
	if strings.Contains(view, "Find in current tree") || !strings.Contains(view, "inside.txt") {
		t.Fatalf("search result did not navigate to the selected directory")
	}
}

func TestSearchEnterSelectsFile(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "documents")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "needle.txt"), []byte("found"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	model := ui.New(searchTheme())
	loaded, _ := model.Update(model.Init()())
	searching := openAndLoadSearch(t, loaded.(ui.Model))
	filtered, _ := searching.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("needle")})
	selecting, command := filtered.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("selecting a nested file should load its containing directory")
	}
	selected, _ := selecting.(ui.Model).Update(command())
	view := selected.(ui.Model).View()
	if strings.Contains(view, "Find in current tree") || !strings.Contains(view, "1/1  needle.txt") {
		t.Fatal("file search result was not selected in the entries pane")
	}
}

func TestSearchFindsNestedEntries(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "one", "two")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "needle.txt"), []byte("found"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	model := ui.New(searchTheme())
	loaded, _ := model.Update(model.Init()())
	searching := openAndLoadSearch(t, loaded.(ui.Model))
	filtered, _ := searching.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("needle")})
	view := filtered.(ui.Model).View()
	if !strings.Contains(view, filepath.Join("one", "two", "needle.txt")) {
		t.Fatalf("recursive search did not show the nested file")
	}
}

func BenchmarkSearchTypingLargeIndex(b *testing.B) {
	root := b.TempDir()
	for index := 0; index < 10_000; index++ {
		name := filepath.Join(root, fmt.Sprintf("candidate-%05d.txt", index))
		if err := os.WriteFile(name, nil, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	b.Chdir(root)
	model := ui.New(searchTheme())
	loaded, _ := model.Update(model.Init()())
	searching := openAndLoadSearch(b, loaded.(ui.Model))
	b.ResetTimer()
	for range b.N {
		updated, _ := searching.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		backspaced, _ := updated.(ui.Model).Update(tea.KeyMsg{Type: tea.KeyBackspace})
		searching = backspaced.(ui.Model)
	}
}
