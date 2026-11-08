package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/fs/rename"
)

// showLineErrors puts validation errors on d's lines and keeps it open. Errors
// about the whole input go to the status line.
func (m Model) showLineErrors(d *dialog, problems []rename.LineError) (Model, tea.Cmd) {
	d.lineErrors = map[int]string{}
	m.status = ""
	for _, problem := range problems {
		if problem.Line < 0 {
			m.status = problem.Message
			continue
		}
		d.lineErrors[problem.Line] = problem.Message
	}
	m.dialog = d
	return m, nil
}

// createEntries asks for names to create, one per line; a leading or trailing
// "/" makes a directory.
func (m Model) createEntries() (tea.Model, tea.Cmd) {
	if m.data.CWD == "" || m.inTrash() {
		return m, nil
	}
	cwd := m.data.CWD
	m.dialog = newTextDialog("Create", "one name per line, end with / for a folder", "", true,
		func(m Model, d *dialog) (Model, tea.Cmd) {
			items, problems := rename.PlanCreate(cwd, d.input.value())
			if len(problems) > 0 {
				return m.showLineErrors(d, problems)
			}
			if len(items) == 0 {
				return m, nil
			}
			created, err := rename.Create(cwd, items)
			status := fmt.Sprintf("Created %d", len(created))
			if len(created) == 1 {
				status = "Created " + filepath.Base(created[0])
			}
			if err != nil {
				status = fmt.Sprintf("Created %d of %d: %v", len(created), len(items), err)
			}
			selected := ""
			if len(created) > 0 {
				selected = created[0]
			}
			m.status = status
			return m, loadDirectory(cwd, selected)
		})
	return m, nil
}

// renameTargets edits the names of the focused or marked entries, one line each.
func (m Model) renameTargets() (tea.Model, tea.Cmd) {
	targets := m.targets()
	if len(targets) == 0 {
		return m, nil
	}
	cwd := m.data.CWD
	focused := ""
	if m.data.Selected >= 0 && m.data.Selected < len(m.data.Entries) {
		focused = m.data.Entries[m.data.Selected].Entry.Path
	}
	names := make([]string, len(targets))
	for index, path := range targets {
		names[index] = filepath.Base(path)
	}
	m.dialog = newTextDialog("Rename "+countLabel(targets), "", strings.Join(names, "\n"), false,
		func(m Model, d *dialog) (Model, tea.Cmd) {
			plan, problems := rename.PlanRename(cwd, targets, d.lines())
			if len(problems) > 0 {
				return m.showLineErrors(d, problems)
			}
			return m.applyRenames(cwd, plan, focused)
		})
	return m, nil
}

// applyRenames runs plan in the background, then reloads with focused (or the
// path it was renamed to) selected.
func (m Model) applyRenames(cwd string, plan []rename.Rename, focused string) (Model, tea.Cmd) {
	if len(plan) == 0 {
		m.status = "Nothing renamed"
		return m, nil
	}
	selected := focused
	for _, step := range plan {
		if step.From == focused {
			selected = step.To
		}
	}
	m.clearMarks()
	return m, func() tea.Msg {
		if err := rename.Apply(plan); err != nil {
			return fileActionMsg{status: "Rename failed: " + err.Error(), reload: true, selectedPath: focused}
		}
		status := fmt.Sprintf("Renamed %d items", len(plan))
		if len(plan) == 1 {
			status = "Renamed to " + filepath.Base(plan[0].To)
		}
		return fileActionMsg{status: status, reload: true, selectedPath: selected}
	}
}

// editorRenameMsg arrives when the editor started by editorRename exits.
type editorRenameMsg struct {
	cwd     string
	targets []string
	file    string
	err     error
}

const editorRenameHeader = `# Edit the names below, one line per item, then save and quit.
# A path containing / moves the item; missing folders are created.
# Lines starting with # are ignored. Leave a file unchanged to cancel.
`

// editorRename opens the focused or marked entries' names in $VISUAL or
// $EDITOR; saving applies the changes.
func (m Model) editorRename() (tea.Model, tea.Cmd) {
	targets := m.targets()
	if len(targets) == 0 {
		return m, nil
	}
	editor := strings.Fields(firstNonEmpty(os.Getenv("VISUAL"), os.Getenv("EDITOR"), "vi"))
	file, err := os.CreateTemp("", "nukri-rename-*.txt")
	if err != nil {
		m.status = "Editor rename: " + err.Error()
		return m, nil
	}
	_, err = file.WriteString(editorRenameHeader + rename.EditorText(m.data.CWD, targets))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(file.Name())
		m.status = "Editor rename: " + err.Error()
		return m, nil
	}
	cwd := m.data.CWD
	cmd := exec.Command(editor[0], append(editor[1:], file.Name())...)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorRenameMsg{cwd: cwd, targets: targets, file: file.Name(), err: err}
	})
}

func (m Model) handleEditorRename(message editorRenameMsg) (tea.Model, tea.Cmd) {
	defer os.Remove(message.file)
	if message.err != nil {
		m.status = "Editor exited with an error: " + message.err.Error()
		return m, nil
	}
	edited, err := os.ReadFile(message.file)
	if err != nil {
		m.status = "Editor rename: " + err.Error()
		return m, nil
	}
	plan, problems := rename.PlanEditor(message.cwd, message.targets, string(edited))
	if len(problems) > 0 {
		m.status = "Nothing renamed: " + problems[0].Error()
		return m, nil
	}
	if len(plan) == 0 {
		m.status = "Nothing renamed"
		return m, nil
	}
	focused := ""
	if m.data.Selected >= 0 && m.data.Selected < len(m.data.Entries) {
		focused = m.data.Entries[m.data.Selected].Entry.Path
	}
	lines := make([]string, 0, min(3, len(plan)))
	for _, step := range plan[:min(3, len(plan))] {
		from, _ := filepath.Rel(message.cwd, step.From)
		to, _ := filepath.Rel(message.cwd, step.To)
		lines = append(lines, from+" → "+to)
	}
	detail := strings.Join(lines, ", ")
	if len(plan) > len(lines) {
		detail += fmt.Sprintf(" and %d more", len(plan)-len(lines))
	}
	m.dialog = newConfirmDialog(fmt.Sprintf("Apply %d renames?", len(plan)), detail,
		func(m Model, _ *dialog) (Model, tea.Cmd) {
			return m.applyRenames(message.cwd, plan, focused)
		})
	return m, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
