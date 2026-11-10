package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/ui"
)

func wheel(model tea.Model, x, y int, button tea.MouseButton) (tea.Model, tea.Cmd) {
	return model.Update(tea.MouseMsg{X: x, Y: y, Button: button, Action: tea.MouseActionPress})
}

// The wheel moves the file list one entry a notch, and the preview waits
// until the wheel rests instead of loading for every entry passed.
func TestWheelOverFilesWaitsToPreview(t *testing.T) {
	directory := t.TempDir()
	for i := range 10 {
		name := filepath.Join(directory, fmt.Sprintf("file-%02d.txt", i))
		if err := os.WriteFile(name, []byte(fmt.Sprintf("content-%02d", i)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(directory)
	model := ui.New(searchTheme())
	resized, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 14})
	loaded, previewCommand := resized.(ui.Model).Update(model.Init()())
	previewed := runCommand(loaded, previewCommand)
	if !strings.Contains(previewed.(ui.Model).View(), "content-00") {
		t.Fatal("the first file's preview did not load")
	}

	// Over the file list: x 30 is past the 23 cell sidebar, y 5 is an entry row.
	first, firstSettle := wheel(previewed, 30, 5, tea.MouseButtonWheelDown)
	view := first.(ui.Model).View()
	if !strings.Contains(view, "2/10") {
		t.Fatal("one wheel notch should move the selection one entry")
	}
	if !strings.Contains(view, "Loading preview") || firstSettle == nil {
		t.Fatal("the preview should wait for the wheel to rest")
	}
	second, secondSettle := wheel(first, 30, 5, tea.MouseButtonWheelDown)
	if !strings.Contains(second.(ui.Model).View(), "3/10") {
		t.Fatal("a second notch should move one more entry")
	}

	// The first notch's timer is stale once the wheel has moved again.
	if _, command := second.Update(firstSettle()); command != nil {
		t.Fatal("a superseded wheel move should not load a preview")
	}
	settled, load := second.Update(secondSettle())
	if load == nil {
		t.Fatal("the preview should load once the wheel rests")
	}
	if !strings.Contains(runCommand(settled, load).(ui.Model).View(), "content-02") {
		t.Fatal("the preview that loaded is not the selected file's")
	}

	// A key press loads its own preview at once, so a waiting wheel timer is dropped.
	wheeled, settle := wheel(previewed, 30, 5, tea.MouseButtonWheelDown)
	keyed, _ := wheeled.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if _, command := keyed.Update(settle()); command != nil {
		t.Fatal("a wheel timer should not load a preview after a key moved the selection")
	}
}

func TestWheelMovesThroughSearchResults(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"a1.txt", "a2.txt", "a3.txt"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(directory)
	model := ui.New(searchTheme())
	resized, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	loaded, _ := resized.(ui.Model).Update(model.Init()())
	searching := openAndLoadSearch(t, loaded.(ui.Model))

	selectedRow := func(model tea.Model) string {
		for line := range strings.SplitSeq(model.(ui.Model).View(), "\n") {
			if strings.Contains(line, "› ") {
				return line
			}
		}
		return ""
	}
	down, _ := wheel(searching, 50, 12, tea.MouseButtonWheelDown)
	if !strings.Contains(selectedRow(down), "a2.txt") {
		t.Fatalf("wheel down should select the second result, got %q", selectedRow(down))
	}
	up, _ := wheel(down, 50, 12, tea.MouseButtonWheelUp)
	up, _ = wheel(up, 50, 12, tea.MouseButtonWheelUp)
	if !strings.Contains(selectedRow(up), "a1.txt") {
		t.Fatalf("wheel up should stop at the first result, got %q", selectedRow(up))
	}
	if !strings.Contains(up.(ui.Model).View(), "Find in current tree") {
		t.Fatal("the wheel should not close the search")
	}
}
