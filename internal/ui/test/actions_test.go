package test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/clipboard"
	"github.com/urdadx/nukri/internal/ui"
)

// actionModel loads a model on a temporary directory holding files, with a
// private trash.
func actionModel(t *testing.T, files ...string) (ui.Model, string) {
	t.Helper()
	model, directory, _ := loadActionModel(t, files...)
	return model, directory
}

// loadActionModel is actionModel that also returns the commands the first
// listing started, such as loading the sidebar.
func loadActionModel(t *testing.T, files ...string) (ui.Model, string, tea.Cmd) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	directory := t.TempDir()
	for _, name := range files {
		path := filepath.Join(directory, name)
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte("content of "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(directory)
	model := ui.New(searchTheme())
	loaded, cmd := model.Update(model.Init()())
	return loaded.(ui.Model), directory, cmd
}

func press(t *testing.T, model ui.Model, keys ...string) (ui.Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	var updated tea.Model = model
	for _, key := range keys {
		message := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		switch key {
		case "enter":
			message = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			message = tea.KeyMsg{Type: tea.KeyEsc}
		case "space":
			message = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
		case "alt+enter":
			message = tea.KeyMsg{Type: tea.KeyEnter, Alt: true}
		case "ctrl+u":
			message = tea.KeyMsg{Type: tea.KeyCtrlU}
		case "down":
			message = tea.KeyMsg{Type: tea.KeyDown}
		case "alt+d":
			message = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d"), Alt: true}
		}
		updated, cmd = updated.(ui.Model).Update(message)
	}
	return updated.(ui.Model), cmd
}

// settle runs cmd and every command its messages return, expanding batches,
// until nothing is left. Timers would loop, so tea.Tick results are not followed
// past a few rounds.
func settle(t *testing.T, model ui.Model, cmd tea.Cmd) ui.Model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for rounds := 0; len(queue) > 0 && rounds < 200; rounds++ {
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		message := next()
		if batch, ok := message.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if message == nil {
			continue
		}
		var updated tea.Model
		var follow tea.Cmd
		updated, follow = model.Update(message)
		model = updated.(ui.Model)
		if !isPlacesRefresh(message) {
			queue = append(queue, follow)
		}
	}
	return model
}

// isPlacesRefresh stops the sidebar's refresh timer from looping in settle.
func isPlacesRefresh(message tea.Msg) bool {
	return strings.Contains(strings.ToLower(typeName(message)), "places")
}

func typeName(value any) string {
	return strings.TrimPrefix(strings.TrimPrefix(fmtType(value), "ui."), "*")
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestMarkingSelectsAndEscClears(t *testing.T) {
	model, _ := actionModel(t, "a.txt", "b.txt", "c.txt")
	model, _ = press(t, model, "space", "space")
	if view := model.View(); !strings.Contains(view, "2 selected") {
		t.Fatal("footer should count marked entries")
	}
	model, _ = press(t, model, "esc")
	if strings.Contains(model.View(), "selected") {
		t.Fatal("esc should clear the marks")
	}
}

func TestTrashMovesMarkedEntriesAndRestoreBringsThemBack(t *testing.T) {
	model, directory := actionModel(t, "a.txt", "b.txt", "c.txt")
	model, _ = press(t, model, "space", "space") // mark a and b
	model, cmd := press(t, model, "d")
	settle(t, model, cmd)
	if exists(filepath.Join(directory, "a.txt")) || exists(filepath.Join(directory, "b.txt")) || !exists(filepath.Join(directory, "c.txt")) {
		t.Fatal("d should trash exactly the marked entries")
	}
	trashFiles := filepath.Join(os.Getenv("XDG_DATA_HOME"), "Trash", "files")
	if !exists(filepath.Join(trashFiles, "a.txt")) {
		t.Fatal("trashed entry is not in the trash")
	}

	// Inside the trash, r restores.
	t.Chdir(trashFiles)
	model = ui.New(searchTheme())
	loaded, _ := model.Update(model.Init()())
	model, cmd = press(t, loaded.(ui.Model), "r")
	settle(t, model, cmd)
	if !exists(filepath.Join(directory, "a.txt")) {
		t.Fatal("r in the trash should restore the entry")
	}
}

func TestPermanentDeleteAsksFirst(t *testing.T) {
	model, directory := actionModel(t, "doomed.txt", "kept.txt")
	model, cmd := press(t, model, "D")
	if cmd != nil || !strings.Contains(model.View(), "Permanently delete") {
		t.Fatal("D should ask for confirmation before deleting")
	}
	cancelled, _ := press(t, model, "n")
	if !exists(filepath.Join(directory, "doomed.txt")) || strings.Contains(cancelled.View(), "Permanently delete") {
		t.Fatal("n should cancel without deleting")
	}
	model, cmd = press(t, model, "y")
	settle(t, model, cmd)
	if exists(filepath.Join(directory, "doomed.txt")) || !exists(filepath.Join(directory, "kept.txt")) {
		t.Fatal("y should delete only the focused entry")
	}
}

func TestCreateMakesFilesAndFolders(t *testing.T) {
	model, directory := actionModel(t, "existing.txt")
	model, _ = press(t, model, "a", "notes.md", "alt+enter", "docs/")
	model, cmd := press(t, model, "enter")
	settle(t, model, cmd)
	if info, err := os.Stat(filepath.Join(directory, "docs")); err != nil || !info.IsDir() {
		t.Fatal("a trailing / should create a folder")
	}
	if !exists(filepath.Join(directory, "notes.md")) {
		t.Fatal("create did not make the file")
	}
}

func TestCreateKeepsDialogOpenOnError(t *testing.T) {
	model, _ := actionModel(t, "existing.txt")
	model, _ = press(t, model, "a", "existing.txt")
	model, _ = press(t, model, "enter")
	if !strings.Contains(model.View(), "line 1:") {
		t.Fatal("an existing name should be reported on its line, keeping the dialog open")
	}
}

func TestRenameMarkedEntriesInline(t *testing.T) {
	model, directory := actionModel(t, "one.txt", "two.txt")
	model, _ = press(t, model, "space", "space", "r")
	// Replace both lines.
	model, _ = press(t, model, "ctrl+u", "first.txt", "down", "ctrl+u", "second.txt")
	model, cmd := press(t, model, "enter")
	settle(t, model, cmd)
	if !exists(filepath.Join(directory, "first.txt")) || !exists(filepath.Join(directory, "second.txt")) || exists(filepath.Join(directory, "one.txt")) {
		t.Fatal("inline bulk rename did not rename both entries")
	}
}

func TestCopyPathUsesTerminalClipboard(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")
	model, directory := actionModel(t, "file.txt")
	var terminal bytes.Buffer
	model = model.WithTerminal(&terminal)
	press(t, model, "c", "p")
	if terminal.String() != clipboard.OSC52(filepath.Join(directory, "file.txt")) {
		t.Fatalf("copy path wrote %q", terminal.String())
	}
}

func TestDuplicateFinderNeverTrashesLastCopy(t *testing.T) {
	_, directory := actionModel(t)
	for _, name := range []string{"x.bin", "y.bin"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("same bytes"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Reload so the listing includes the new files.
	fresh := ui.New(searchTheme())
	loaded, _ := fresh.Update(fresh.Init()())
	model, cmd := press(t, loaded.(ui.Model), "alt+d")
	model = settle(t, model, cmd)
	if !strings.Contains(model.View(), "2 copies") {
		t.Fatal("duplicate finder did not report the pair")
	}
	model, _ = press(t, model, "d")
	press(t, model, "d")
	remaining := 0
	for _, name := range []string{"x.bin", "y.bin"} {
		if exists(filepath.Join(directory, name)) {
			remaining++
		}
	}
	if remaining != 1 {
		t.Fatalf("%d copies remain, want exactly 1", remaining)
	}
}

func fmtType(value any) string { return reflect.TypeOf(value).String() }

// Previewing a file with Windows line endings must not send a carriage return
// to the terminal; it used to corrupt the sidebar and file list.
func TestCRLFPreviewSendsNoCarriageReturn(t *testing.T) {
	_, directory := actionModel(t)
	if err := os.WriteFile(filepath.Join(directory, "License.txt"), []byte("\tLicense\r\n\r\nline two\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "bad\rname.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	fresh := ui.New(searchTheme())
	loaded, cmd := fresh.Update(fresh.Init()())
	model := settle(t, loaded.(ui.Model), cmd)
	// Check both entries: License.txt, then the oddly named file.
	if strings.ContainsAny(model.View(), "\r\t") {
		t.Fatal("view contains a carriage return or tab")
	}
	moved, next := press(t, model, "down")
	if strings.ContainsAny(settle(t, moved, next).View(), "\r\t") {
		t.Fatal("view of the second entry contains a carriage return or tab")
	}
}

// pasteMarkedInto marks a.txt and b.txt, presses key (y or x), enters dest and
// pastes.
func pasteMarkedInto(t *testing.T, key string) (ui.Model, string) {
	t.Helper()
	model, directory := actionModel(t, "dest/", "a.txt", "b.txt", "c.txt")
	// Folders come first: dest, a.txt, b.txt, c.txt.
	model, _ = press(t, model, "down", "space", "space", key)
	if !strings.Contains(model.View(), "2 items") {
		t.Fatalf("footer should count the 2 items on the clipboard")
	}
	model, _ = press(t, model, "up", "up", "up")
	model, cmd := press(t, model, "enter")
	model = settle(t, model, cmd)
	model, cmd = press(t, model, "p")
	return settle(t, model, cmd), directory
}

func TestCopyAndPasteMarkedEntries(t *testing.T) {
	_, directory := pasteMarkedInto(t, "y")
	for _, name := range []string{"a.txt", "b.txt"} {
		if !exists(filepath.Join(directory, "dest", name)) || !exists(filepath.Join(directory, name)) {
			t.Fatalf("%s should be copied into dest and stay in place", name)
		}
	}
	if exists(filepath.Join(directory, "dest", "c.txt")) {
		t.Fatal("an unmarked entry was copied")
	}
}

func TestCutAndPasteMovesMarkedEntries(t *testing.T) {
	model, directory := pasteMarkedInto(t, "x")
	for _, name := range []string{"a.txt", "b.txt"} {
		if !exists(filepath.Join(directory, "dest", name)) || exists(filepath.Join(directory, name)) {
			t.Fatalf("%s should be moved into dest", name)
		}
	}
	if strings.Contains(model.View(), "items cut") {
		t.Fatal("the clipboard should be empty after the move")
	}
}

// Pressing paste again while a paste runs must not paste everything twice.
func TestSecondPasteWaitsForTheFirst(t *testing.T) {
	model, directory := actionModel(t, "a.txt", "sub/")
	model, _ = press(t, model, "y") // copy the focused entry
	model, first := press(t, model, "p")
	model, second := press(t, model, "p")
	if first == nil || second != nil {
		t.Fatal("the second paste should wait for the first")
	}
	model = settle(t, model, first)
	entries, _ := os.ReadDir(directory)
	if len(entries) != 3 {
		t.Fatalf("folder has %d entries after one paste, want 3", len(entries))
	}
	if _, cmd := press(t, model, "y", "p"); cmd == nil {
		t.Fatal("a paste after the first finished should run")
	}
}
