package rename

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Rename moves From to To. Both are absolute paths.
type Rename struct {
	From, To string
}

// candidate is one validated line before unchanged entries are dropped.
type candidate struct {
	line  int
	label string
	from  string
	to    string
}

// PlanRename builds a rename plan for sources, all located in dir, where
// names[i] is the new base name for sources[i]. Names are trimmed of
// surrounding whitespace. Unchanged names are dropped from the plan. A target
// may exist only when it is itself one of the sources being renamed away, so
// swaps and rotations are allowed.
func PlanRename(dir string, sources []string, names []string) ([]Rename, []LineError) {
	if len(names) != len(sources) {
		return nil, []LineError{{Line: -1, Message: countMessage(len(sources), len(names))}}
	}
	dir = filepath.Clean(dir)
	var errs []LineError
	cands := make([]candidate, 0, len(sources))
	for i, source := range sources {
		source = filepath.Clean(source)
		base := filepath.Base(source)
		name := names[i]
		if name != base {
			name = strings.TrimSpace(name)
		}
		if filepath.Dir(source) != dir {
			errs = append(errs, LineError{Line: i, Message: fmt.Sprintf("%q is not in %s", base, dir)})
			continue
		}
		if msg := validateName(name); msg != "" {
			errs = append(errs, LineError{Line: i, Message: msg})
			continue
		}
		cands = append(cands, candidate{line: i, label: name, from: source, to: filepath.Join(dir, name)})
	}
	return finishPlan(cands, errs)
}

// finishPlan runs the cross-line checks shared by inline and editor renames
// and returns the plan with unchanged entries dropped.
func finishPlan(cands []candidate, errs []LineError) ([]Rename, []LineError) {
	moving := map[string]bool{}
	var targets []string
	for _, c := range cands {
		if c.from != c.to {
			moving[c.from] = true
			targets = append(targets, c.to)
		}
	}
	seen := map[string]bool{}
	for _, c := range cands {
		if msg := checkCandidate(c, moving, targets, seen); msg != "" {
			errs = append(errs, LineError{Line: c.line, Message: msg})
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	var plan []Rename
	for _, c := range cands {
		if c.from != c.to {
			plan = append(plan, Rename{From: c.from, To: c.to})
		}
	}
	return plan, nil
}

func checkCandidate(c candidate, moving map[string]bool, targets []string, seen map[string]bool) string {
	if seen[c.to] {
		return fmt.Sprintf("%q appears more than once", c.label)
	}
	seen[c.to] = true
	if c.from == c.to {
		return ""
	}
	if within(c.to, c.from) {
		return "cannot move a folder inside itself"
	}
	for other := range moving {
		if other == c.from {
			continue
		}
		if within(c.from, other) {
			return "a parent folder is also being renamed"
		}
		if within(c.to, other) {
			return "destination folder is being renamed"
		}
	}
	for _, target := range targets {
		if within(c.to, target) {
			return "destination folder is also a rename target"
		}
	}
	if !moving[c.to] && exists(c.to) && !sameFile(c.from, c.to) {
		return fmt.Sprintf("%q already exists", c.label)
	}
	if msg := checkParent(filepath.Dir(c.to)); msg != "" {
		return msg
	}
	return ""
}

// checkParent rejects a destination whose nearest existing ancestor is not a
// directory. Missing directories are created by Apply.
func checkParent(dir string) string {
	for {
		info, err := os.Stat(dir)
		if err == nil {
			if !info.IsDir() {
				return fmt.Sprintf("%q is not a folder", dir)
			}
			return ""
		}
		if !os.IsNotExist(err) {
			return fmt.Sprintf("cannot access %q: %v", dir, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// within reports whether path is strictly inside dir.
func within(path, dir string) bool {
	if dir == "/" {
		return path != "/"
	}
	return strings.HasPrefix(path, dir+"/")
}

// sameFile reports whether a and b name the same inode, which happens for
// case-only renames on case-insensitive filesystems.
func sameFile(a, b string) bool {
	ia, err := os.Lstat(a)
	if err != nil {
		return false
	}
	ib, err := os.Lstat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ia, ib)
}

func countMessage(want, got int) string {
	plural := "s"
	if want == 1 {
		plural = ""
	}
	return fmt.Sprintf("expected %d name%s, got %d", want, plural, got)
}
