package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/apps"
	"github.com/urdadx/nukri/internal/clipboard"
	"github.com/urdadx/nukri/internal/fs/trash"
)

// fileActionMsg reports a finished file action: a status line, and the
// directory to reload with the entry to select afterwards.
type fileActionMsg struct {
	status       string
	reload       bool
	selectedPath string
}

// WithTerminal sets the terminal writer used for the OSC 52 clipboard fallback.
func (m Model) WithTerminal(terminal io.Writer) Model {
	m.terminal = terminal
	return m
}

func (m Model) handleFileActionMsg(message fileActionMsg) (tea.Model, tea.Cmd) {
	m.status = message.status
	if message.reload && m.data.CWD != "" {
		return m, loadDirectory(m.data.CWD, message.selectedPath)
	}
	return m, nil
}

// inTrash reports whether the browser is inside a trash's files directory,
// where trash means delete and rename means restore.
func (m Model) inTrash() bool {
	return m.data.CWD != "" && trash.IsInTrash(m.data.CWD)
}

// neighbourAfter picks the entry to select once paths are gone: the first
// remaining entry after the focused one, else the last remaining before it.
func (m Model) neighbourAfter(paths []string) string {
	gone := map[string]bool{}
	for _, path := range paths {
		gone[path] = true
	}
	for index := m.data.Selected; index >= 0 && index < len(m.data.Entries); index++ {
		if path := m.data.Entries[index].Entry.Path; !gone[path] {
			return path
		}
	}
	for index := min(m.data.Selected, len(m.data.Entries)) - 1; index >= 0; index-- {
		if path := m.data.Entries[index].Entry.Path; !gone[path] {
			return path
		}
	}
	return ""
}

// countLabel is "name" for one path and "N items" for several.
func countLabel(paths []string) string {
	if len(paths) == 1 {
		return fmt.Sprintf("%q", filepath.Base(paths[0]))
	}
	return fmt.Sprintf("%d items", len(paths))
}

func (m Model) trashTargets() (tea.Model, tea.Cmd) {
	targets := m.targets()
	if len(targets) == 0 {
		return m, nil
	}
	if m.inTrash() {
		return m.confirmDelete(targets)
	}
	selected := m.neighbourAfter(targets)
	m.clearMarks()
	m.status = "Moving to trash…"
	return m, func() tea.Msg {
		moved := 0
		for _, path := range targets {
			if _, err := trash.Trash(path); err != nil {
				return fileActionMsg{status: fmt.Sprintf("Trashed %d of %d: %v", moved, len(targets), err), reload: true, selectedPath: selected}
			}
			moved++
		}
		return fileActionMsg{status: "Moved " + countLabel(targets) + " to trash", reload: true, selectedPath: selected}
	}
}

func (m Model) deleteTargets() (tea.Model, tea.Cmd) {
	targets := m.targets()
	if len(targets) == 0 {
		return m, nil
	}
	return m.confirmDelete(targets)
}

// confirmDelete asks before permanently deleting targets, then removes them in
// the background.
func (m Model) confirmDelete(targets []string) (tea.Model, tea.Cmd) {
	names := make([]string, 0, min(3, len(targets)))
	for _, path := range targets[:min(3, len(targets))] {
		names = append(names, filepath.Base(path))
	}
	detail := strings.Join(names, ", ")
	if len(targets) > len(names) {
		detail += fmt.Sprintf(" and %d more", len(targets)-len(names))
	}
	m.dialog = newConfirmDialog("Permanently delete "+countLabel(targets)+"? This can't be undone.", detail,
		func(m Model, _ *dialog) (Model, tea.Cmd) {
			selected := m.neighbourAfter(targets)
			m.clearMarks()
			m.status = "Deleting…"
			return m, func() tea.Msg {
				for index, path := range targets {
					if err := trash.Delete(path); err != nil {
						return fileActionMsg{status: fmt.Sprintf("Deleted %d of %d: %v", index, len(targets), err), reload: true, selectedPath: selected}
					}
				}
				return fileActionMsg{status: "Deleted " + countLabel(targets), reload: true, selectedPath: selected}
			}
		})
	return m, nil
}

func (m Model) restoreTargets() (tea.Model, tea.Cmd) {
	targets := m.targets()
	if len(targets) == 0 {
		return m, nil
	}
	selected := m.neighbourAfter(targets)
	m.clearMarks()
	return m, func() tea.Msg {
		restored, conflicts := 0, 0
		var failure error
		for _, path := range targets {
			_, err := trash.Restore(path)
			switch {
			case err == nil:
				restored++
			case errors.Is(err, trash.ErrRestoreConflict):
				conflicts++
			default:
				failure = err
			}
		}
		status := "Restored " + countLabel(targets)
		if restored != len(targets) {
			status = fmt.Sprintf("Restored %d of %d", restored, len(targets))
			if conflicts > 0 {
				status += fmt.Sprintf("; %d already exist at their original location", conflicts)
			}
			if failure != nil {
				status += fmt.Sprintf("; %v", failure)
			}
		}
		return fileActionMsg{status: status, reload: true, selectedPath: selected}
	}
}

// copyPathChoices are what "copy path" can copy, for one or several targets.
func copyPathChoices(cwd string, targets []string) []dialogChoice {
	join := func(value func(string) string) string {
		values := make([]string, len(targets))
		for index, path := range targets {
			values[index] = value(path)
		}
		return strings.Join(values, "\n")
	}
	stem := func(path string) string {
		name := filepath.Base(path)
		if extension := filepath.Ext(name); extension != name {
			return strings.TrimSuffix(name, extension)
		}
		return name
	}
	choices := []dialogChoice{
		{label: "Name", key: "n", value: join(filepath.Base)},
		{label: "Name without extension", key: "s", value: join(stem)},
		{label: "Absolute path", key: "p", value: join(func(path string) string { return path })},
		{label: "Directory path", key: "d", value: cwd},
	}
	for index := range choices {
		preview, _, _ := strings.Cut(choices[index].value.(string), "\n")
		if len(targets) > 1 {
			preview += fmt.Sprintf(" (+%d)", len(targets)-1)
		}
		choices[index].detail = preview
	}
	return choices
}

func (m Model) copyPath() (tea.Model, tea.Cmd) {
	targets := m.targets()
	if len(targets) == 0 {
		return m, nil
	}
	m.dialog = newChoiceDialog("Copy to clipboard", countLabel(targets), copyPathChoices(m.data.CWD, targets),
		func(m Model, d *dialog) (Model, tea.Cmd) {
			text, _ := d.chosen().(string)
			if _, err := clipboard.Copy(text, m.terminal); err != nil {
				m.status = "Copy failed: " + err.Error()
				return m, nil
			}
			m.status = "Copied " + strings.ToLower(d.choices[d.selected].label)
			return m, nil
		})
	return m, nil
}

// openDefault opens each target with its default application.
func (m Model) openDefault() (tea.Model, tea.Cmd) {
	targets := m.targets()
	if len(targets) == 0 {
		return m, nil
	}
	m.clearMarks()
	var commands []tea.Cmd
	for _, path := range targets {
		commands = append(commands, openWithDefault(path))
	}
	return m, tea.Sequence(commands...)
}

// openWithDefault opens path with its MIME type's default application. Apps
// that run in a terminal take over this one until they exit; others start
// detached. Without a known default it falls back to xdg-open.
func openWithDefault(path string) tea.Cmd {
	mimeType, err := apps.MIMEType(context.Background(), path)
	if err == nil {
		if app, ok, _ := apps.DefaultFor(mimeType); ok {
			return launch(app, []string{path})
		}
	}
	return func() tea.Msg {
		if err := startDetached("xdg-open", path); err != nil {
			return fileActionMsg{status: "Open failed: " + err.Error()}
		}
		return nil
	}
}

// launch runs app on paths.
func launch(app apps.App, paths []string) tea.Cmd {
	if !app.Terminal {
		return func() tea.Msg {
			if err := app.Start(paths); err != nil {
				return fileActionMsg{status: "Open failed: " + err.Error()}
			}
			return fileActionMsg{status: "Opened with " + app.Name}
		}
	}
	var commands []tea.Cmd
	for _, argv := range app.Commands(paths) {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = app.Dir
		commands = append(commands, tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return fileActionMsg{status: app.Name + ": " + err.Error()}
			}
			return nil
		}))
	}
	if len(commands) == 0 {
		return func() tea.Msg { return fileActionMsg{status: "Can't run " + app.Name + ": invalid Exec line"} }
	}
	return tea.Sequence(commands...)
}

// startDetached starts a program that outlives nukri and doesn't touch the
// terminal.
func startDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer null.Close()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, null, null
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// openWith lists the applications that can open the focused or marked items.
func (m Model) openWith() (tea.Model, tea.Cmd) {
	targets := m.targets()
	if len(targets) == 0 {
		return m, nil
	}
	mimeType, err := apps.MIMEType(context.Background(), targets[0])
	if err != nil {
		m.status = "Open with: " + err.Error()
		return m, nil
	}
	candidates, err := apps.AppsFor(mimeType)
	if err != nil || len(candidates) == 0 {
		m.status = "No applications found for " + mimeType
		return m, nil
	}
	choices := make([]dialogChoice, len(candidates))
	for index, app := range candidates {
		choices[index] = dialogChoice{label: app.Name, value: app}
		if index == 0 {
			choices[index].detail = "default"
		}
		if app.Terminal {
			choices[index].detail = strings.TrimSpace(choices[index].detail + " terminal")
		}
	}
	m.dialog = newChoiceDialog("Open "+countLabel(targets)+" with", mimeType, choices,
		func(m Model, d *dialog) (Model, tea.Cmd) {
			app, ok := d.chosen().(apps.App)
			if !ok {
				return m, nil
			}
			m.clearMarks()
			return m, launch(app, targets)
		})
	return m, nil
}
