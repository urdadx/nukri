package test

import (
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/fs/rename"
)

func TestEditorTextRoundTrip(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "")
	sub := mkdir(t, dir, "sub")
	hash := writeFile(t, dir, "#hash", "")
	outside := filepath.Join(filepath.Dir(dir), "elsewhere")
	sources := []string{a, sub, hash, outside}
	text := rename.EditorText(dir, sources)
	want := "a.txt\nsub\n./#hash\n" + outside + "\n"
	if text != want {
		t.Fatalf("text = %q, want %q", text, want)
	}
	plan, errs := rename.PlanEditor(dir, sources, text)
	if len(errs) != 0 || len(plan) != 0 {
		t.Fatalf("plan = %+v errs = %v", plan, errs)
	}
}

func TestEditorMoveIntoNewSubdir(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "A")
	b := writeFile(t, dir, "b", "B")
	edited := "# header comment\nnew/deeper/a\r\nb\n"
	plan, errs := rename.PlanEditor(dir, []string{a, b}, edited)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(plan) != 1 || plan[0].To != filepath.Join(dir, "new", "deeper", "a") {
		t.Fatalf("plan = %+v", plan)
	}
	if err := rename.Apply(plan); err != nil {
		t.Fatal(err)
	}
	if readFile(t, filepath.Join(dir, "new", "deeper", "a")) != "A" {
		t.Fatal("move failed")
	}
	if !equalStrings(listDir(t, dir), []string{"b", "new"}) {
		t.Fatalf("dir = %v", listDir(t, dir))
	}
}

func TestEditorAbsoluteAndDotDot(t *testing.T) {
	dir := t.TempDir()
	sub := mkdir(t, dir, "sub")
	f := writeFile(t, sub, "f", "")
	plan, errs := rename.PlanEditor(sub, []string{f}, "../g\n")
	if len(errs) != 0 || len(plan) != 1 || plan[0].To != filepath.Join(dir, "g") {
		t.Fatalf("plan = %+v errs = %v", plan, errs)
	}
	plan, errs = rename.PlanEditor(sub, []string{f}, filepath.Join(dir, "h")+"\n")
	if len(errs) != 0 || len(plan) != 1 || plan[0].To != filepath.Join(dir, "h") {
		t.Fatalf("plan = %+v errs = %v", plan, errs)
	}
}

func TestEditorWrongLineCount(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "")
	b := writeFile(t, dir, "b", "")
	for _, edited := range []string{"a\n", "a\nb\nc\n", "# only comment\n", "a\nb\n\n"} {
		_, errs := rename.PlanEditor(dir, []string{a, b}, edited)
		if len(errs) != 1 || errs[0].Line != -1 {
			t.Fatalf("%q: errors = %v", edited, errs)
		}
	}
}

func TestEditorCommentsIgnoredAndLineNumbers(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "")
	b := writeFile(t, dir, "b", "")
	c := writeFile(t, dir, "c", "")
	writeFile(t, dir, "taken", "")
	edited := "# one\n# two\nx\n\n#three\ntaken\n"
	_, errs := rename.PlanEditor(dir, []string{a, b, c}, edited)
	if got := errorLines(errs); !equalInts(got, []int{3, 5}) {
		t.Fatalf("errors = %v", errs)
	}
}

func TestEditorDuplicates(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "")
	b := writeFile(t, dir, "b", "")
	_, errs := rename.PlanEditor(dir, []string{a, b}, "x\n./x\n")
	if got := errorLines(errs); !equalInts(got, []int{1}) {
		t.Fatalf("errors = %v", errs)
	}
}

func TestEditorDirIntoItself(t *testing.T) {
	dir := t.TempDir()
	d := mkdir(t, dir, "d")
	_, errs := rename.PlanEditor(dir, []string{d}, "d/inner/d\n")
	if got := errorLines(errs); !equalInts(got, []int{0}) {
		t.Fatalf("errors = %v", errs)
	}
}

func TestEditorIntoRenamedFolderRejected(t *testing.T) {
	dir := t.TempDir()
	d := mkdir(t, dir, "d")
	f := writeFile(t, dir, "f", "")
	_, errs := rename.PlanEditor(dir, []string{d, f}, "e\nd/f\n")
	if got := errorLines(errs); !equalInts(got, []int{1}) {
		t.Fatalf("errors = %v", errs)
	}
	// Moving into a folder that another line creates by renaming.
	_, errs = rename.PlanEditor(dir, []string{d, f}, "e\ne/f\n")
	if got := errorLines(errs); !equalInts(got, []int{1}) {
		t.Fatalf("errors = %v", errs)
	}
}

func TestEditorParentIsFile(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "")
	writeFile(t, dir, "file", "")
	_, errs := rename.PlanEditor(dir, []string{a}, "file/a\n")
	if got := errorLines(errs); !equalInts(got, []int{0}) {
		t.Fatalf("errors = %v", errs)
	}
}

func TestEditorEmptyAndRootRejected(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "")
	b := writeFile(t, dir, "b", "")
	_, errs := rename.PlanEditor(dir, []string{a, b}, "\n/\n")
	if got := errorLines(errs); !equalInts(got, []int{0, 1}) {
		t.Fatalf("errors = %v", errs)
	}
}
