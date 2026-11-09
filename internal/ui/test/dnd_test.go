package test

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/kittydnd"
	"github.com/urdadx/nukri/internal/ui"
)

// dndModel is an action model sized so entries can be hit, writing its
// drag-and-drop replies to the returned buffer.
func dndModel(t *testing.T, files ...string) (ui.Model, string, *bytes.Buffer) {
	t.Helper()
	model, directory := actionModel(t, files...)
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
	model, directory, _ := dndModel(t)
	source := filepath.Join(t.TempDir(), "dropped.txt")
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	updated, _ := model.Update(kittydnd.Event{Kind: kittydnd.DropOffer, MIME: 1, Operation: kittydnd.Either, Final: true})
	updated, cmd := updated.(ui.Model).Update(kittydnd.Event{Kind: kittydnd.DropData, MIME: 1, Paths: []string{source}})
	settle(t, updated.(ui.Model), cmd)

	if !exists(filepath.Join(directory, "dropped.txt")) {
		t.Fatal("dropped file was not copied in")
	}
	if !exists(source) {
		t.Fatal("drop removed the file from where it was dragged from")
	}
}
