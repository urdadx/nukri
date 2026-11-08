package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/fs/rename"
)

func planAndApply(t *testing.T, dir string, sources, names []string) []rename.Rename {
	t.Helper()
	plan, errs := rename.PlanRename(dir, sources, names)
	if len(errs) != 0 {
		t.Fatalf("plan errors: %v", errs)
	}
	if err := rename.Apply(plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return plan
}

func TestRenameSimple(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "A")
	plan := planAndApply(t, dir, []string{a}, []string{"  b  "})
	if len(plan) != 1 || plan[0].To != filepath.Join(dir, "b") {
		t.Fatalf("plan = %+v", plan)
	}
	if !equalStrings(listDir(t, dir), []string{"b"}) || readFile(t, filepath.Join(dir, "b")) != "A" {
		t.Fatalf("dir = %v", listDir(t, dir))
	}
}

func TestRenameSwap(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "A")
	b := writeFile(t, dir, "b", "B")
	planAndApply(t, dir, []string{a, b}, []string{"b", "a"})
	if readFile(t, a) != "B" || readFile(t, b) != "A" {
		t.Fatal("swap failed")
	}
	if !equalStrings(listDir(t, dir), []string{"a", "b"}) {
		t.Fatalf("leftovers: %v", listDir(t, dir))
	}
}

func TestRenameThreeCycle(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "A")
	b := writeFile(t, dir, "b", "B")
	c := mkdir(t, dir, "c")
	writeFile(t, c, "inner", "C")
	planAndApply(t, dir, []string{a, b, c}, []string{"b", "c", "a"})
	if readFile(t, b) != "A" || readFile(t, c) != "B" || readFile(t, filepath.Join(a, "inner")) != "C" {
		t.Fatal("rotation failed")
	}
	if !equalStrings(listDir(t, dir), []string{"a", "b", "c"}) {
		t.Fatalf("leftovers: %v", listDir(t, dir))
	}
}

func TestRenameChain(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "A")
	b := writeFile(t, dir, "b", "B")
	planAndApply(t, dir, []string{a, b}, []string{"b", "c"})
	if readFile(t, b) != "A" || readFile(t, filepath.Join(dir, "c")) != "B" {
		t.Fatal("chain failed")
	}
	if !equalStrings(listDir(t, dir), []string{"b", "c"}) {
		t.Fatalf("dir = %v", listDir(t, dir))
	}
}

func TestRenameCaseOnly(t *testing.T) {
	dir := t.TempDir()
	foo := writeFile(t, dir, "foo", "x")
	planAndApply(t, dir, []string{foo}, []string{"Foo"})
	if !equalStrings(listDir(t, dir), []string{"Foo"}) {
		t.Fatalf("dir = %v", listDir(t, dir))
	}
}

func TestRenameUnchangedDropped(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "")
	b := writeFile(t, dir, "b", "")
	plan, errs := rename.PlanRename(dir, []string{a, b}, []string{"a", "z"})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(plan) != 1 || plan[0] != (rename.Rename{From: b, To: filepath.Join(dir, "z")}) {
		t.Fatalf("plan = %+v", plan)
	}
	plan, errs = rename.PlanRename(dir, []string{a, b}, []string{"a", "b"})
	if len(errs) != 0 || len(plan) != 0 {
		t.Fatalf("plan = %+v errs = %v", plan, errs)
	}
}

func TestRenameDuplicateTargets(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "")
	b := writeFile(t, dir, "b", "")
	c := writeFile(t, dir, "c", "")
	_, errs := rename.PlanRename(dir, []string{a, b, c}, []string{"x", "x", "a"})
	// Line 2 may take "a" because a is renamed away; only line 1 fails.
	if got := errorLines(errs); !equalInts(got, []int{1}) {
		t.Fatalf("errors = %v", errs)
	}
	// Renaming onto an unchanged source is a duplicate too.
	_, errs = rename.PlanRename(dir, []string{a, b}, []string{"a", "a"})
	if got := errorLines(errs); !equalInts(got, []int{1}) {
		t.Fatalf("errors = %v", errs)
	}
}

func TestRenameTargetExists(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "")
	writeFile(t, dir, "other", "")
	_, errs := rename.PlanRename(dir, []string{a}, []string{"other"})
	if got := errorLines(errs); !equalInts(got, []int{0}) {
		t.Fatalf("errors = %v", errs)
	}
}

func TestRenameInvalidNames(t *testing.T) {
	dir := t.TempDir()
	var sources []string
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		sources = append(sources, writeFile(t, dir, n, ""))
	}
	_, errs := rename.PlanRename(dir, sources, []string{"", "  ", "x/y", ".", ".."})
	if got := errorLines(errs); !equalInts(got, []int{0, 1, 2, 3, 4}) {
		t.Fatalf("errors = %v", errs)
	}
	_, errs = rename.PlanRename(dir, sources, []string{"a"})
	if len(errs) != 1 || errs[0].Line != -1 {
		t.Fatalf("errors = %v", errs)
	}
}

func TestRenameSourceOutsideDir(t *testing.T) {
	dir := t.TempDir()
	sub := mkdir(t, dir, "sub")
	f := writeFile(t, sub, "f", "")
	_, errs := rename.PlanRename(dir, []string{f}, []string{"g"})
	if len(errs) != 1 || errs[0].Line != 0 {
		t.Fatalf("errors = %v", errs)
	}
	if _, err := os.Lstat(f); err != nil {
		t.Fatal(err)
	}
}
