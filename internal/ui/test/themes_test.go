package test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/theme"
	"github.com/urdadx/nukri/internal/ui"
)

// themeSpy is a catalog that records what the picker loads and saves.
type themeSpy struct {
	loaded []string
	saved  []string
}

func (s *themeSpy) catalog() ui.ThemeCatalog {
	return ui.ThemeCatalog{
		Current: "alpha",
		Names:   func() []string { return []string{"alpha", "beta", "gamma"} },
		Load: func(name string) (theme.Theme, error) {
			s.loaded = append(s.loaded, name)
			return searchTheme(), nil
		},
		Save: func(name string) error {
			s.saved = append(s.saved, name)
			return nil
		},
	}
}

func TestThemePickerPreviewsAndSavesChoice(t *testing.T) {
	spy := &themeSpy{}
	model, _ := actionModel(t)
	model = model.WithThemes(spy.catalog())
	model, _ = press(t, model, "T", "down")
	if len(spy.loaded) != 1 || spy.loaded[0] != "beta" {
		t.Fatalf("moving should preview the next theme, loaded %v", spy.loaded)
	}
	model, cmd := press(t, model, "enter")
	if len(spy.saved) != 1 || spy.saved[0] != "beta" || cmd == nil {
		t.Fatalf("enter should save the theme and reload, saved %v", spy.saved)
	}
	if model.View() == "" {
		t.Fatal("empty view after choosing a theme")
	}
}

func TestThemePickerEscKeepsOriginal(t *testing.T) {
	spy := &themeSpy{}
	model, _ := actionModel(t)
	model = model.WithThemes(spy.catalog())
	model, _ = press(t, model, "T", "down", "down")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if len(spy.saved) != 0 {
		t.Fatalf("esc must not save, saved %v", spy.saved)
	}
	if updated.(ui.Model).View() == "" {
		t.Fatal("empty view after cancelling")
	}
}
