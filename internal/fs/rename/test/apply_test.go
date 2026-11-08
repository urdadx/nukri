package test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/fs/rename"
)

func TestApplyRefusesToClobberDirectTarget(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "A")
	plan, errs := rename.PlanRename(dir, []string{a}, []string{"b"})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	writeFile(t, dir, "b", "unrelated")
	err := rename.Apply(plan)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("err = %v", err)
	}
	if readFile(t, a) != "A" || readFile(t, filepath.Join(dir, "b")) != "unrelated" {
		t.Fatal("files changed")
	}
}

func TestApplyRefusesOccupiedTargetInSwapPlan(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "A")
	b := writeFile(t, dir, "b", "B")
	c := writeFile(t, dir, "c", "C")
	plan, errs := rename.PlanEditor(dir, []string{a, b, c}, "b\na\nnew/d\n")
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	// Appears between planning and applying.
	writeFile(t, dir, "new/d", "unrelated")
	if err := rename.Apply(plan); !errors.Is(err, os.ErrExist) {
		t.Fatalf("err = %v", err)
	}
	if readFile(t, a) != "A" || readFile(t, b) != "B" || readFile(t, c) != "C" {
		t.Fatal("files changed")
	}
	if readFile(t, filepath.Join(dir, "new", "d")) != "unrelated" {
		t.Fatal("unrelated file clobbered")
	}
	if !equalStrings(listDir(t, dir), []string{"a", "b", "c", "new"}) {
		t.Fatalf("leftovers: %v", listDir(t, dir))
	}
}

func TestApplyRollsBackMidway(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "A")
	b := writeFile(t, dir, "b", "B")
	c := writeFile(t, dir, "c", "C")
	d := writeFile(t, dir, "d", "D")
	ro := mkdir(t, dir, "ro")
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	// Swap a<->b is staged, d moves into a created folder, then creating
	// ro/sub fails and everything must be undone.
	plan := []rename.Rename{
		{From: a, To: b},
		{From: b, To: a},
		{From: d, To: filepath.Join(dir, "made", "d")},
		{From: c, To: filepath.Join(ro, "sub", "c")},
	}
	if err := rename.Apply(plan); err == nil {
		t.Fatal("expected error")
	}
	if readFile(t, a) != "A" || readFile(t, b) != "B" || readFile(t, c) != "C" || readFile(t, d) != "D" {
		t.Fatal("rollback did not restore files")
	}
	if !equalStrings(listDir(t, dir), []string{"a", "b", "c", "d", "ro"}) {
		t.Fatalf("leftovers: %v", listDir(t, dir))
	}
}

func TestApplyRejectsDuplicateTargets(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "")
	b := writeFile(t, dir, "b", "")
	to := filepath.Join(dir, "z")
	if err := rename.Apply([]rename.Rename{{From: a, To: to}, {From: b, To: to}}); err == nil {
		t.Fatal("expected error")
	}
	if !equalStrings(listDir(t, dir), []string{"a", "b"}) {
		t.Fatalf("dir = %v", listDir(t, dir))
	}
}

func TestApplyMissingSource(t *testing.T) {
	dir := t.TempDir()
	if err := rename.Apply([]rename.Rename{{From: filepath.Join(dir, "nope"), To: filepath.Join(dir, "x")}}); err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyEmptyPlan(t *testing.T) {
	if err := rename.Apply(nil); err != nil {
		t.Fatal(err)
	}
}
