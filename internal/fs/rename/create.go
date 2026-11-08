// Package rename plans and executes batch create, rename and editor bulk
// rename operations. Planning only validates and never touches the disk, so
// the UI can show per-line errors before anything is applied.
package rename

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// LineError reports a validation problem for one input line. Line is the
// 0-based index of the line in the text or slice given to the planner; it is
// -1 for errors that concern the input as a whole.
type LineError struct {
	Line    int
	Message string
}

func (e LineError) Error() string {
	if e.Line < 0 {
		return e.Message
	}
	return fmt.Sprintf("line %d: %s", e.Line+1, e.Message)
}

// CreateItem is one entry of a batch create plan.
type CreateItem struct {
	Line int
	Name string
	Dir  bool
}

// PlanCreate parses text (one entry per line) into items to create in dir.
// Blank lines are ignored and surrounding whitespace is trimmed. A leading or
// trailing '/' marks a directory.
func PlanCreate(dir string, text string) ([]CreateItem, []LineError) {
	var items []CreateItem
	var errs []LineError
	seen := map[string]bool{}
	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		item := CreateItem{
			Line: i,
			Dir:  strings.HasPrefix(line, "/") || strings.HasSuffix(line, "/"),
			Name: strings.Trim(line, "/"),
		}
		if msg := validateName(item.Name); msg != "" {
			errs = append(errs, LineError{Line: i, Message: msg})
			continue
		}
		if seen[item.Name] {
			errs = append(errs, LineError{Line: i, Message: fmt.Sprintf("%q appears more than once", item.Name)})
			continue
		}
		seen[item.Name] = true
		if exists(filepath.Join(dir, item.Name)) {
			errs = append(errs, LineError{Line: i, Message: fmt.Sprintf("%q already exists", item.Name)})
			continue
		}
		items = append(items, item)
	}
	return items, errs
}

// Create creates the planned items in dir: files with O_EXCL and mode 0644,
// directories with mode 0755. It stops at the first error and returns the
// paths created so far.
func Create(dir string, items []CreateItem) (created []string, err error) {
	for _, item := range items {
		if msg := validateName(item.Name); msg != "" {
			return created, fmt.Errorf("create %q: %s", item.Name, msg)
		}
		path := filepath.Join(dir, item.Name)
		if item.Dir {
			if err := os.Mkdir(path, 0o755); err != nil {
				return created, fmt.Errorf("create folder %q: %w", item.Name, err)
			}
		} else {
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				return created, fmt.Errorf("create file %q: %w", item.Name, err)
			}
			if err := f.Close(); err != nil {
				return append(created, path), fmt.Errorf("create file %q: %w", item.Name, err)
			}
		}
		created = append(created, path)
	}
	return created, nil
}

// validateName checks a single base name and returns a message or "".
func validateName(name string) string {
	switch {
	case name == "":
		return "name cannot be empty"
	case name == "." || name == "..":
		return "name cannot be . or .."
	case strings.ContainsRune(name, '/'):
		return "name cannot contain /"
	case strings.ContainsRune(name, 0):
		return "name cannot contain NUL"
	}
	return ""
}

// exists reports whether path exists without following a final symlink.
func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}
