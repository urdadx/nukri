package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/fs/rename"
)

func TestPlanCreateParsesFilesAndDirs(t *testing.T) {
	dir := t.TempDir()
	text := "a.txt\n\n/docs\nsrc/\n  //both//  \n\t\n"
	items, errs := rename.PlanCreate(dir, text)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	want := []rename.CreateItem{
		{Line: 0, Name: "a.txt"},
		{Line: 2, Name: "docs", Dir: true},
		{Line: 3, Name: "src", Dir: true},
		{Line: 4, Name: "both", Dir: true},
	}
	if len(items) != len(want) {
		t.Fatalf("items = %+v, want %+v", items, want)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Fatalf("item %d = %+v, want %+v", i, items[i], want[i])
		}
	}
}

func TestPlanCreateErrorsHaveLineNumbers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "existing", "")
	text := "ok\n\nexisting\n/\n.\n..\na/b\nok\nnul\x00\ndup/\ndup\n"
	items, errs := rename.PlanCreate(dir, text)
	if got, want := errorLines(errs), []int{2, 3, 4, 5, 6, 7, 8, 10}; !equalInts(got, want) {
		t.Fatalf("error lines = %v, want %v (%v)", got, want, errs)
	}
	if len(items) != 2 || items[0].Name != "ok" || items[1].Name != "dup" || !items[1].Dir {
		t.Fatalf("items = %+v", items)
	}
}

func TestCreateMakesFilesAndDirs(t *testing.T) {
	dir := t.TempDir()
	items, errs := rename.PlanCreate(dir, "f.txt\nd/\n")
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	created, err := rename.Create(dir, items)
	if err != nil {
		t.Fatal(err)
	}
	if !equalStrings(created, []string{filepath.Join(dir, "f.txt"), filepath.Join(dir, "d")}) {
		t.Fatalf("created = %v", created)
	}
	fi, err := os.Stat(filepath.Join(dir, "f.txt"))
	if err != nil || fi.IsDir() || fi.Mode().Perm()&0o600 != 0o600 {
		t.Fatalf("file: %v %v", fi, err)
	}
	di, err := os.Stat(filepath.Join(dir, "d"))
	if err != nil || !di.IsDir() {
		t.Fatalf("dir: %v %v", di, err)
	}
}

func TestCreateStopsAtFirstError(t *testing.T) {
	dir := t.TempDir()
	items, errs := rename.PlanCreate(dir, "one\ntwo\nthree\n")
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	// Appears between planning and creating; must not be truncated.
	writeFile(t, dir, "two", "keep")
	created, err := rename.Create(dir, items)
	if err == nil {
		t.Fatal("expected error")
	}
	if !equalStrings(created, []string{filepath.Join(dir, "one")}) {
		t.Fatalf("created = %v", created)
	}
	if readFile(t, filepath.Join(dir, "two")) != "keep" {
		t.Fatal("existing file was modified")
	}
	if _, err := os.Lstat(filepath.Join(dir, "three")); !os.IsNotExist(err) {
		t.Fatal("three should not exist")
	}
}
