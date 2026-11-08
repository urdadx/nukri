package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/config/icons"
	"github.com/urdadx/nukri/internal/theme"
)

// ThemeCatalog connects the theme picker to where themes live and where the
// choice is saved.
type ThemeCatalog struct {
	Current string          // name of the theme in use
	Names   func() []string // themes to offer
	Load    func(name string) (theme.Theme, error)
	Save    func(name string) error // remember the choice
}

// WithThemes enables the theme picker.
func (m Model) WithThemes(catalog ThemeCatalog) Model {
	m.themes = catalog
	return m
}

// applyTheme restyles the UI. Icon colors are cached per entry, so callers
// reload the listing when the change is kept.
func (m Model) applyTheme(t theme.Theme) Model {
	m.styles = NewStyles(t)
	icons.InitIcon(true, t.DirectoryIconColor)
	return m
}

/*
openThemePicker lists the available themes. Moving through the list previews
each one live; Enter keeps it and saves it to the config, Esc puts the original
back.
*/
func (m Model) openThemePicker() (Model, tea.Cmd) {
	if m.themes.Names == nil || m.themes.Load == nil {
		return m, nil
	}
	names := m.themes.Names()
	if len(names) == 0 {
		return m, nil
	}
	original, originalName := m.styles.Theme, m.themes.Current
	choices := make([]dialogChoice, len(names))
	selected := 0
	for index, name := range names {
		choices[index] = dialogChoice{label: name, value: name}
		if name == originalName {
			choices[index].detail = "current"
			selected = index
		}
	}
	d := newChoiceDialog("Theme", "↑↓ to preview", choices, func(m Model, d *dialog) (Model, tea.Cmd) {
		name, _ := d.chosen().(string)
		chosen, err := m.themes.Load(name)
		if err != nil {
			m.status = "Theme: " + err.Error()
			return m.applyTheme(original), nil
		}
		m = m.applyTheme(chosen)
		m.themes.Current = name
		m.status = "Theme: " + name
		if m.themes.Save != nil {
			if err := m.themes.Save(name); err != nil {
				m.status = "Theme applied but not saved: " + err.Error()
			}
		}
		// Reload so folder icon colors and code highlighting pick it up.
		return m, m.reloadKeepingSelection()
	})
	d.selected = selected
	d.moved = func(m Model, d *dialog) Model {
		name, _ := d.chosen().(string)
		if preview, err := m.themes.Load(name); err == nil {
			return m.applyTheme(preview)
		}
		return m
	}
	d.cancel = func(m Model) Model { return m.applyTheme(original) }
	m.dialog = d
	return m, nil
}

// reloadKeepingSelection reloads the current folder with the focused entry
// still selected.
func (m Model) reloadKeepingSelection() tea.Cmd {
	if m.data.CWD == "" {
		return nil
	}
	selected := ""
	if m.data.Selected >= 0 && m.data.Selected < len(m.data.Entries) {
		selected = m.data.Entries[m.data.Selected].Entry.Path
	}
	return loadDirectory(m.data.CWD, selected)
}
