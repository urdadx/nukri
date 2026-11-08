package rename

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// move records one completed os.Rename so it can be undone.
type move struct {
	from, to string
}

// Apply executes plan. Sources whose target is occupied by another source
// (swaps, cycles, chains) are first parked in a private temporary directory
// next to them, then every item is moved to its final place. Missing parent
// directories of targets are created. Before every rename the destination is
// checked with os.Lstat so a path outside the plan is never overwritten. On
// failure Apply undoes what it did and returns the error, joined with any
// rollback errors.
func Apply(plan []Rename) error {
	if len(plan) == 0 {
		return nil
	}
	froms := map[string]bool{}
	tos := map[string]bool{}
	for _, r := range plan {
		if !filepath.IsAbs(r.From) || !filepath.IsAbs(r.To) {
			return fmt.Errorf("rename %q: paths must be absolute", r.From)
		}
		if froms[r.From] {
			return fmt.Errorf("rename %q: listed more than once", r.From)
		}
		if tos[r.To] {
			return fmt.Errorf("rename to %q: target listed more than once", r.To)
		}
		froms[r.From] = true
		tos[r.To] = true
	}
	for _, r := range plan {
		if _, err := os.Lstat(r.From); err != nil {
			return fmt.Errorf("rename %q: %w", filepath.Base(r.From), err)
		}
		if !froms[r.To] {
			if err := checkFree(r.From, r.To); err != nil {
				return err
			}
		}
	}

	a := &applier{}
	if err := a.run(plan, froms); err != nil {
		if rbErr := a.rollback(); rbErr != nil {
			return errors.Join(err, fmt.Errorf("rollback incomplete: %w", rbErr))
		}
		return err
	}
	_ = a.cleanupTemp()
	return nil
}

type applier struct {
	moves    []move
	tempDirs []string
	madeDirs []string
}

func (a *applier) run(plan []Rename, froms map[string]bool) error {
	var direct, staged []Rename
	for _, r := range plan {
		if froms[r.To] {
			staged = append(staged, r)
		} else {
			direct = append(direct, r)
		}
	}
	// Park every source whose target is still occupied by another source.
	parked := make([]string, len(staged))
	for i, r := range staged {
		tmp, err := os.MkdirTemp(filepath.Dir(r.From), ".nukri-rename-")
		if err != nil {
			return fmt.Errorf("rename %q: %w", filepath.Base(r.From), err)
		}
		a.tempDirs = append(a.tempDirs, tmp)
		parked[i] = filepath.Join(tmp, filepath.Base(r.From))
		if err := a.move(r.From, parked[i]); err != nil {
			return err
		}
	}
	// Targets of direct moves are not sources, so they are free now.
	for _, r := range direct {
		if err := a.place(r.From, r.To, r.From); err != nil {
			return err
		}
	}
	// Every source has been vacated, so staged targets are free too.
	for i, r := range staged {
		if err := a.place(parked[i], r.To, r.From); err != nil {
			return err
		}
	}
	return nil
}

// place moves from to its final destination after creating missing parents
// and re-checking that the destination is free. orig is the source path the
// user sees in errors and the path a case-only rename may alias.
func (a *applier) place(from, to, orig string) error {
	if err := a.mkdirParents(filepath.Dir(to)); err != nil {
		return fmt.Errorf("rename %q: %w", filepath.Base(orig), err)
	}
	if err := checkFree(from, to); err != nil {
		return err
	}
	return a.move(from, to)
}

func (a *applier) move(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return fmt.Errorf("rename %q: %w", filepath.Base(from), err)
	}
	a.moves = append(a.moves, move{from: from, to: to})
	return nil
}

// mkdirParents creates dir and any missing ancestors, recording each one so
// rollback can remove it again.
func (a *applier) mkdirParents(dir string) error {
	var missing []string
	for d := dir; ; d = filepath.Dir(d) {
		info, err := os.Stat(d)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("%q is not a folder", d)
			}
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		missing = append(missing, d)
		if filepath.Dir(d) == d {
			break
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0o755); err != nil {
			return err
		}
		a.madeDirs = append(a.madeDirs, missing[i])
	}
	return nil
}

// rollback undoes completed moves in reverse order, never overwriting an
// existing path, then removes created and temporary directories.
func (a *applier) rollback() error {
	var errs []error
	for i := len(a.moves) - 1; i >= 0; i-- {
		m := a.moves[i]
		if _, err := os.Lstat(m.from); err == nil && !sameFile(m.from, m.to) {
			errs = append(errs, fmt.Errorf("cannot restore %q: path exists", m.from))
			continue
		}
		if err := os.Rename(m.to, m.from); err != nil {
			errs = append(errs, fmt.Errorf("restore %q: %w", m.from, err))
		}
	}
	for i := len(a.madeDirs) - 1; i >= 0; i-- {
		if err := os.Remove(a.madeDirs[i]); err != nil {
			errs = append(errs, fmt.Errorf("remove created folder: %w", err))
		}
	}
	if err := a.cleanupTemp(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// cleanupTemp removes the (now empty) temporary parking directories.
func (a *applier) cleanupTemp() error {
	var errs []error
	for _, dir := range a.tempDirs {
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove temporary folder: %w", err))
		}
	}
	return errors.Join(errs...)
}

// checkFree returns an error unless to is absent or is the same file as from
// (a case-only rename on a case-insensitive filesystem).
func checkFree(from, to string) error {
	_, err := os.Lstat(to)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return fmt.Errorf("rename to %q: %w", filepath.Base(to), err)
	case sameFile(from, to):
		return nil
	}
	return fmt.Errorf("rename to %q: %w", filepath.Base(to), os.ErrExist)
}
