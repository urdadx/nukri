package test

import (
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
