package test

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/ui"
)

// While the preview is hidden nothing is loaded for it; showing it again
// loads the focused entry's preview.
func TestTogglePreviewStopsAndResumesLoading(t *testing.T) {
	model, _ := actionModel(t, "a.txt", "b.txt")
	resized, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	model = resized.(ui.Model)

	if _, cmd := press(t, model, "down"); cmd == nil {
		t.Fatal("moving with the preview shown should load a preview")
	}
	model, cmd := press(t, model, "P")
	if cmd != nil {
		t.Fatal("hiding the preview should load nothing")
	}
	model, cmd = press(t, model, "down")
	if cmd != nil {
		t.Fatal("moving with the preview hidden should load nothing")
	}
	if _, cmd = press(t, model, "P"); cmd == nil {
		t.Fatal("showing the preview should load the focused entry's preview")
	}
}

// A preview hidden last time starts hidden, and every toggle is remembered.
func TestPreviewToggleIsRemembered(t *testing.T) {
	model, _ := actionModel(t, "a.txt", "b.txt")
	var saved []bool
	model = model.WithPreview(false, func(shown bool) error {
		saved = append(saved, shown)
		return nil
	})
	if _, cmd := press(t, model, "down"); cmd != nil {
		t.Fatal("a preview hidden at startup should load nothing")
	}
	model, _ = press(t, model, "P")
	press(t, model, "P")
	if len(saved) != 2 || !saved[0] || saved[1] {
		t.Fatalf("saved = %v, want shown then hidden", saved)
	}
}

func TestPreviewToggleReportsSaveFailure(t *testing.T) {
	model, _ := actionModel(t, "a.txt")
	model = model.WithPreview(true, func(bool) error { return errors.New("read-only file system") })
	model, _ = press(t, model, "P")
	if !strings.Contains(model.View(), "read-only file system") {
		t.Fatal("a failed save should be reported")
	}
}
