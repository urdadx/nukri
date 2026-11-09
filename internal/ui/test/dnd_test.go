package test

import (
	"bytes"
	"encoding/base64"
	"fmt"
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
	model = model.WithDragOutput(output, nil)
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
	model = model.WithDragOutput(output, nil)
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

// dropUntilData offers source at cell (x, y) and delivers its data, returning
// the model and the transfer command without running it.
func dropUntilData(t *testing.T, model ui.Model, x, y int, operation kittydnd.Operation, source string) (ui.Model, tea.Cmd) {
	t.Helper()
	updated, _ := model.Update(kittydnd.Event{Kind: kittydnd.DropOffer, X: x, Y: y, MIME: 1, Operation: operation, Final: true})
	updated, cmd := updated.(ui.Model).Update(kittydnd.Event{Kind: kittydnd.DropData, MIME: 1, Paths: []string{source}})
	return updated.(ui.Model), cmd
}

func TestDropIsFinishedOnlyAfterTheTransfer(t *testing.T) {
	model, directory, output := dndModel(t, "a.txt")
	source := droppedFile(t)
	model, cmd := dropUntilData(t, model, listX(t, model), 20, kittydnd.Move, source)
	if strings.Contains(output.String(), "t=r:o=") {
		t.Fatalf("drop finished before its files were moved: %q", output.String())
	}

	// A second drop can't start while the first is unfinished.
	output.Reset()
	model.Update(kittydnd.Event{Kind: kittydnd.DropOffer, X: 1, Y: 20, MIME: 1, Operation: kittydnd.Copy})
	if output.String() != kittydnd.RejectDropSequence() {
		t.Fatalf("offer during a drop = %q, want a rejection", output.String())
	}

	output.Reset()
	settle(t, model, cmd)
	if !strings.HasPrefix(output.String(), kittydnd.FinishDropSequence(kittydnd.Move)) {
		t.Fatalf("finished drop = %q, want the move reported", output.String())
	}
	if !exists(filepath.Join(directory, "dropped.txt")) || exists(source) {
		t.Fatal("dropped file was not moved in")
	}
}

func TestFailedDropIsReportedCanceled(t *testing.T) {
	model, _, output := dndModel(t, "a.txt")
	missing := filepath.Join(t.TempDir(), "missing.txt")
	model, cmd := dropUntilData(t, model, listX(t, model), 20, kittydnd.Move, missing)

	output.Reset()
	settle(t, model, cmd)
	if !strings.HasPrefix(output.String(), kittydnd.FinishDropSequence(0)) {
		t.Fatalf("failed drop = %q, want it reported canceled", output.String())
	}
}

// dragsAfter presses the left button on the first entry, sends between, then
// starts a drag over an empty row of the list. It reports whether that drag
// offered the pressed entry.
func dragsAfter(t *testing.T, between func(ui.Model) ui.Model) bool {
	t.Helper()
	model, _, output := dndModel(t, "a.txt")
	x := listX(t, model)
	pressed, _ := model.Update(tea.MouseMsg{X: x, Y: 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	model = between(pressed.(ui.Model))
	output.Reset()
	model.Update(kittydnd.Event{Kind: kittydnd.DragOffer, X: x, Y: 20})
	return strings.Contains(output.String(), "t=p:x=0")
}

func TestDragForgetsPressedEntryAfterRelease(t *testing.T) {
	// X10 mouse reports a release without saying which button went up.
	released := dragsAfter(t, func(model ui.Model) ui.Model {
		updated, _ := model.Update(tea.MouseMsg{X: 0, Y: 0, Button: tea.MouseButtonNone, Action: tea.MouseActionRelease})
		return updated.(ui.Model)
	})
	if released {
		t.Fatal("a drag after the button was released offered the old entry")
	}
}

func TestDragForgetsPressedEntryReleasedUnderDialog(t *testing.T) {
	releasedUnderDialog := dragsAfter(t, func(model ui.Model) ui.Model {
		model, _ = press(t, model, "a") // open the create dialog
		updated, _ := model.Update(tea.MouseMsg{X: 0, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
		model, _ = press(t, updated.(ui.Model), "esc")
		return model
	})
	if releasedUnderDialog {
		t.Fatal("a release while a dialog was open did not forget the pressed entry")
	}
}

func TestDragStartsFromPressedEntry(t *testing.T) {
	if !dragsAfter(t, func(model ui.Model) ui.Model { return model }) {
		t.Fatal("a drag before the release did not offer the pressed entry")
	}
}

// fakeRemote stands in for kittydnd.Input's transfers with another machine.
type fakeRemote struct {
	drops []kittydnd.RemoteDrop
	sent  []string
	err   error
}

func (f *fakeRemote) FetchRemoteDrop(drop kittydnd.RemoteDrop) error {
	f.drops = append(f.drops, drop)
	return f.err
}

func (f *fakeRemote) SendDragEntry(index int, path string) {
	f.sent = append(f.sent, fmt.Sprintf("%d %s", index, path))
}

// remoteDrop drops a file from another machine at cell (x, y) and returns the
// model once nukri has asked for its contents.
func remoteDrop(t *testing.T, model ui.Model, x, y int) ui.Model {
	t.Helper()
	updated, _ := model.Update(kittydnd.Event{Kind: kittydnd.DropOffer, X: x, Y: y, MIME: 1, Operation: kittydnd.Copy, Final: true})
	remote := "/home/me/report.pdf"
	updated, _ = updated.(ui.Model).Update(kittydnd.Event{Kind: kittydnd.DropData, MIME: 1, Remote: true,
		Paths: []string{remote}, Entries: []kittydnd.DropEntry{{Index: 1, Path: remote}}})
	return updated.(ui.Model)
}

func TestRemoteDropIsFetchedIntoFolderUnderPointer(t *testing.T) {
	model, directory, output := dndModel(t, "sub/", "a.txt")
	remote := &fakeRemote{}
	model = model.WithDragOutput(output, remote)
	model = remoteDrop(t, model, listX(t, model), 3) // over sub/

	if len(remote.drops) != 1 {
		t.Fatalf("fetches = %+v, want one", remote.drops)
	}
	drop := remote.drops[0]
	if drop.Destination != filepath.Join(directory, "sub") || drop.MIME != 1 || len(drop.Entries) != 1 || drop.Entries[0].Index != 1 {
		t.Fatalf("fetch = %+v, want the entry fetched into sub", drop)
	}
	if strings.Contains(output.String(), "t=r:o=") {
		t.Fatal("remote drop finished before its files arrived")
	}

	// The files arrive and the session reports it.
	output.Reset()
	received := filepath.Join(directory, "sub", "report.pdf")
	if err := os.WriteFile(received, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	updated, cmd := model.Update(kittydnd.Event{Kind: kittydnd.RemoteDropDone, Completed: 1, Paths: []string{received}})
	model = settle(t, updated.(ui.Model), cmd)
	if !strings.HasPrefix(output.String(), kittydnd.FinishDropSequence(kittydnd.Copy)) {
		t.Fatalf("finished drop = %q, want the copy reported", output.String())
	}
	if !strings.Contains(model.View(), "Copied 1 item") {
		t.Fatal("status does not report the received file")
	}
}

func TestFailedRemoteDropIsReportedCanceled(t *testing.T) {
	model, _, output := dndModel(t, "a.txt")
	model = model.WithDragOutput(output, &fakeRemote{})
	model = remoteDrop(t, model, listX(t, model), 20)

	output.Reset()
	updated, cmd := model.Update(kittydnd.Event{Kind: kittydnd.RemoteDropDone, Error: "report.pdf: permission denied"})
	model = settle(t, updated.(ui.Model), cmd)
	if !strings.HasPrefix(output.String(), kittydnd.FinishDropSequence(0)) {
		t.Fatalf("failed drop = %q, want it reported canceled", output.String())
	}
	if !strings.Contains(model.View(), "permission denied") {
		t.Fatal("status does not show why the drop failed")
	}
}

func TestRemoteDropWithoutTransferIsRejected(t *testing.T) {
	model, _, output := dndModel(t, "a.txt") // no remote: nukri can't fetch
	model = remoteDrop(t, model, listX(t, model), 20)
	if !strings.HasSuffix(output.String(), kittydnd.FinishDropSequence(0)) {
		t.Fatalf("output = %q, want the drop refused", output.String())
	}
	if !strings.Contains(model.View(), "another machine") {
		t.Fatal("status does not say why the drop failed")
	}
}

// The terminal asks for a dragged entry's contents by its place in the drag.
func TestDragEntryRequestSendsThatPath(t *testing.T) {
	model, directory, output := dndModel(t, "a.txt", "b.txt", "c.txt")
	remote := &fakeRemote{}
	model = model.WithDragOutput(output, remote)
	model, _ = press(t, model, "space", "space") // mark a.txt and b.txt
	updated, _ := model.Update(kittydnd.Event{Kind: kittydnd.DragOffer, X: listX(t, model), Y: 3})
	updated.(ui.Model).Update(kittydnd.Event{Kind: kittydnd.DragEntryRequested, Index: 2})

	want := "2 " + filepath.Join(directory, "b.txt")
	if len(remote.sent) != 1 || remote.sent[0] != want {
		t.Fatalf("sent = %q, want %q", remote.sent, want)
	}
}
