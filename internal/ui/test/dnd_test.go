package test

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/fs/places"
	"github.com/urdadx/nukri/internal/kittydnd"
	"github.com/urdadx/nukri/internal/ui"
	"github.com/urdadx/nukri/internal/ui/browser"
)

// dndModel is an action model with its sidebar places loaded, sized so entries
// can be hit, writing its drag-and-drop replies to the returned buffer. HOME is
// a temporary directory.
func dndModel(t *testing.T, files ...string) (ui.Model, string, *bytes.Buffer) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	model, directory, cmd := loadActionModel(t, files...)
	model = settle(t, model, cmd)
	output := &bytes.Buffer{}
	model = model.WithDragOutput(output)
	resized, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 28})
	return resized.(ui.Model), directory, output
}

// dragFromFirstEntry starts a drag on the first listed entry and returns what
// was written to the terminal.
func dragFromFirstEntry(t *testing.T, model ui.Model, output *bytes.Buffer) string {
	t.Helper()
	for x := 0; x < 100; x++ {
		output.Reset()
		model.Update(kittydnd.Event{Kind: kittydnd.DragOffer, X: x, Y: 3})
		if strings.Contains(output.String(), "t=p:x=0") {
			return output.String()
		}
	}
	t.Fatal("no column over the file list started a drag")
	return ""
}

// listX returns a column over the file list.
func listX(t *testing.T, model ui.Model) int {
	t.Helper()
	output := &bytes.Buffer{}
	model = model.WithDragOutput(output)
	for x := 0; x < 100; x++ {
		output.Reset()
		model.Update(kittydnd.Event{Kind: kittydnd.DragOffer, X: x, Y: 3})
		if strings.Contains(output.String(), "t=p:x=0") {
			return x
		}
	}
	t.Fatal("no column is over the file list")
	return 0
}

// sidebarY returns the screen row of the first sidebar place of kind.
func sidebarY(t *testing.T, kind core.SidebarItemKind) int {
	t.Helper()
	row := 0
	for _, place := range places.BuildSidebarRows() {
		if place.Item == nil && place.Section == "" {
			continue
		}
		if place.Item != nil && place.Item.Kind == kind {
			return browser.PanelContentTop + row
		}
		row++
	}
	t.Fatalf("no sidebar place of kind %d", kind)
	return 0
}

// drop offers source at cell (x, y), drops it there, and runs the transfer.
func drop(t *testing.T, model ui.Model, x, y int, operation kittydnd.Operation, source string) ui.Model {
	t.Helper()
	updated, _ := model.Update(kittydnd.Event{Kind: kittydnd.DropOffer, X: x, Y: y, MIME: 1, Operation: operation})
	updated, _ = updated.(ui.Model).Update(kittydnd.Event{Kind: kittydnd.DropOffer, X: x, Y: y, MIME: 1, Operation: operation, Final: true})
	updated, cmd := updated.(ui.Model).Update(kittydnd.Event{Kind: kittydnd.DropData, MIME: 1, Paths: []string{source}})
	return settle(t, updated.(ui.Model), cmd)
}

func droppedFile(t *testing.T) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "dropped.txt")
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return source
}

func TestDraggingMarkedEntryDragsAllMarked(t *testing.T) {
	model, directory, output := dndModel(t, "a.txt", "b.txt", "c.txt")
	model, _ = press(t, model, "space", "space") // mark a.txt and b.txt
	sequence := dragFromFirstEntry(t, model, output)

	want := string(kittydnd.URIListPayload([]string{filepath.Join(directory, "a.txt"), filepath.Join(directory, "b.txt")}))
	if !strings.Contains(sequence, base64.RawStdEncoding.EncodeToString([]byte(want))) {
		t.Fatalf("drag did not offer both marked files: %q", sequence)
	}
}

func TestDraggingUnmarkedEntryDragsOnlyIt(t *testing.T) {
	model, directory, output := dndModel(t, "a.txt", "b.txt", "c.txt")
	model, _ = press(t, model, "down", "space") // mark b.txt only
	sequence := dragFromFirstEntry(t, model, output)

	want := string(kittydnd.URIListPayload([]string{filepath.Join(directory, "a.txt")}))
	if !strings.Contains(sequence, "t=p:x=0:m=0;"+base64.RawStdEncoding.EncodeToString([]byte(want))+"\x1b\\") {
		t.Fatalf("drag should offer only the entry under the pointer: %q", sequence)
	}
}

func TestDropAllowingEitherOperationCopies(t *testing.T) {
	model, directory, _ := dndModel(t, "a.txt")
	source := droppedFile(t)
	drop(t, model, listX(t, model), 20, kittydnd.Either, source)

	if !exists(filepath.Join(directory, "dropped.txt")) {
		t.Fatal("dropped file was not copied in")
	}
	if !exists(source) {
		t.Fatal("drop removed the file from where it was dragged from")
	}
}

func TestDropOnFolderEntryLandsInIt(t *testing.T) {
	model, directory, _ := dndModel(t, "sub/", "a.txt")
	x := listX(t, model)
	drop(t, model, x, 3, kittydnd.Copy, droppedFile(t)) // sub/ is listed first
	if !exists(filepath.Join(directory, "sub", "dropped.txt")) {
		t.Fatal("drop on a folder did not land in it")
	}
	drop(t, model, x, 4, kittydnd.Copy, droppedFile(t)) // a.txt
	if !exists(filepath.Join(directory, "dropped.txt")) {
		t.Fatal("drop on a file did not land in the folder being viewed")
	}
}

func TestDropOnSidebarPlaceLandsInIt(t *testing.T) {
	model, directory, _ := dndModel(t, "a.txt")
	drop(t, model, 1, sidebarY(t, core.Home), kittydnd.Copy, droppedFile(t))
	if !exists(filepath.Join(os.Getenv("HOME"), "dropped.txt")) {
		t.Fatal("drop on the Home place did not land in it")
	}
	if exists(filepath.Join(directory, "dropped.txt")) {
		t.Fatal("drop on the Home place landed in the folder being viewed")
	}
}

func TestDropOnTrashPlaceIsRejected(t *testing.T) {
	model, directory, output := dndModel(t, "a.txt")
	source := droppedFile(t)
	y := sidebarY(t, core.Trash)

	model.Update(kittydnd.Event{Kind: kittydnd.DropOffer, X: 1, Y: y, MIME: 1, Operation: kittydnd.Copy})
	if output.String() != kittydnd.RejectDropSequence() {
		t.Fatalf("hover over Trash = %q, want a rejection", output.String())
	}
	drop(t, model, 1, y, kittydnd.Copy, source)
	if exists(filepath.Join(directory, "dropped.txt")) {
		t.Fatal("drop on Trash landed in the folder being viewed")
	}
	if entries, _ := os.ReadDir(filepath.Join(os.Getenv("XDG_DATA_HOME"), "Trash", "files")); len(entries) > 0 {
		t.Fatal("drop on Trash copied files into the trash")
	}
}
