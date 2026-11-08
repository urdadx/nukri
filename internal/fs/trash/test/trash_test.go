package test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/urdadx/nukri/internal/fs/trash"
)

// setup isolates the home trash and returns (work dir, home trash root).
func setup(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	data := filepath.Join(root, "data")
	t.Setenv("XDG_DATA_HOME", data)
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	return work, filepath.Join(data, "Trash")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestTrashAndRestoreFile(t *testing.T) {
	work, home := setup(t)
	path := filepath.Join(work, "note.txt")
	writeFile(t, path, "hello")

	before := time.Now().Truncate(time.Second)
	item, err := trash.Trash(path)
	if err != nil {
		t.Fatal(err)
	}
	if exists(path) {
		t.Fatal("original still exists")
	}
	if item.TrashedPath != filepath.Join(home, "files", "note.txt") {
		t.Fatalf("trashed path = %s", item.TrashedPath)
	}
	if item.InfoPath != filepath.Join(home, "info", "note.txt.trashinfo") {
		t.Fatalf("info path = %s", item.InfoPath)
	}
	if item.OriginalPath != path {
		t.Fatalf("original path = %s", item.OriginalPath)
	}

	info := readFile(t, item.InfoPath)
	if !strings.HasPrefix(info, "[Trash Info]\nPath="+path+"\nDeletionDate=") {
		t.Fatalf("unexpected trashinfo:\n%s", info)
	}

	meta, err := trash.Info(item.TrashedPath)
	if err != nil {
		t.Fatal(err)
	}
	if meta.OriginalPath != path || meta.DeletedAt.Before(before) || meta.DeletedAt.After(time.Now()) {
		t.Fatalf("unexpected info: %+v", meta)
	}

	restored, err := trash.Restore(item.TrashedPath)
	if err != nil {
		t.Fatal(err)
	}
	if restored != path || readFile(t, path) != "hello" {
		t.Fatalf("restored to %s", restored)
	}
	if exists(item.TrashedPath) || exists(item.InfoPath) {
		t.Fatal("trash entry not cleaned up")
	}
}

func TestTrashAndRestoreDirectory(t *testing.T) {
	work, home := setup(t)
	dir := filepath.Join(work, "project")
	writeFile(t, filepath.Join(dir, "nested", "a.txt"), "a")

	item, err := trash.Trash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, filepath.Join(home, "files", "project", "nested", "a.txt")) != "a" {
		t.Fatal("directory contents not in trash")
	}
	if _, err := trash.Restore(item.TrashedPath); err != nil {
		t.Fatal(err)
	}
	if readFile(t, filepath.Join(dir, "nested", "a.txt")) != "a" {
		t.Fatal("directory contents not restored")
	}
}

func TestTrashCollisionsGetUniqueNames(t *testing.T) {
	work, home := setup(t)
	want := []string{"report.txt", "report.2.txt", "report.3.txt"}
	for i, name := range want {
		path := filepath.Join(work, "report.txt")
		writeFile(t, path, fmt.Sprint(i))
		item, err := trash.Trash(path)
		if err != nil {
			t.Fatal(err)
		}
		if item.TrashedPath != filepath.Join(home, "files", name) {
			t.Fatalf("got %s, want %s", item.TrashedPath, name)
		}
		if !exists(filepath.Join(home, "info", name+".trashinfo")) {
			t.Fatalf("missing info for %s", name)
		}
	}
	if readFile(t, filepath.Join(home, "files", "report.2.txt")) != "1" {
		t.Fatal("collision overwrote content")
	}

	// Extensionless and dotfile names.
	for _, base := range []string{"Makefile", ".bashrc"} {
		for i := 0; i < 2; i++ {
			writeFile(t, filepath.Join(work, base), "x")
			if _, err := trash.Trash(filepath.Join(work, base)); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, name := range []string{"Makefile.2", ".bashrc.2"} {
		if !exists(filepath.Join(home, "files", name)) {
			t.Fatalf("missing %s", name)
		}
	}
}

func TestTrashSkipsOrphanedFilesEntry(t *testing.T) {
	work, home := setup(t)
	writeFile(t, filepath.Join(home, "files", "a.txt"), "orphan")
	writeFile(t, filepath.Join(work, "a.txt"), "new")
	item, err := trash.Trash(filepath.Join(work, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(item.TrashedPath) != "a.2.txt" || readFile(t, filepath.Join(home, "files", "a.txt")) != "orphan" {
		t.Fatalf("orphan handling wrong: %s", item.TrashedPath)
	}
	if exists(filepath.Join(home, "info", "a.txt.trashinfo")) {
		t.Fatal("stale reservation left behind")
	}
}

func TestPercentEncodingRoundTrip(t *testing.T) {
	work, _ := setup(t)
	path := filepath.Join(work, "my dir", "100% café ☕.txt")
	writeFile(t, path, "x")

	item, err := trash.Trash(path)
	if err != nil {
		t.Fatal(err)
	}
	info := readFile(t, item.InfoPath)
	if !strings.Contains(info, "my%20dir/100%25%20caf%C3%A9%20%E2%98%95.txt") {
		t.Fatalf("path not percent-encoded:\n%s", info)
	}
	meta, err := trash.Info(item.TrashedPath)
	if err != nil {
		t.Fatal(err)
	}
	if meta.OriginalPath != path {
		t.Fatalf("decoded %q, want %q", meta.OriginalPath, path)
	}
	restored, err := trash.Restore(item.TrashedPath)
	if err != nil || restored != path || !exists(path) {
		t.Fatalf("restore: %s %v", restored, err)
	}
}

func TestRestoreConflict(t *testing.T) {
	work, _ := setup(t)
	path := filepath.Join(work, "a.txt")
	writeFile(t, path, "old")
	item, err := trash.Trash(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "new")

	_, err = trash.Restore(item.TrashedPath)
	if !errors.Is(err, trash.ErrRestoreConflict) {
		t.Fatalf("err = %v, want ErrRestoreConflict", err)
	}
	if readFile(t, path) != "new" || readFile(t, item.TrashedPath) != "old" || !exists(item.InfoPath) {
		t.Fatal("conflict modified files")
	}
}

func TestRestoreRecreatesMissingParent(t *testing.T) {
	work, _ := setup(t)
	parent := filepath.Join(work, "gone", "deeper")
	path := filepath.Join(parent, "a.txt")
	writeFile(t, path, "x")
	item, err := trash.Trash(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(work, "gone")); err != nil {
		t.Fatal(err)
	}
	if _, err := trash.Restore(item.TrashedPath); err != nil {
		t.Fatal(err)
	}
	if readFile(t, path) != "x" {
		t.Fatal("not restored")
	}
}

func TestDeleteRemovesTrashInfo(t *testing.T) {
	work, _ := setup(t)
	path := filepath.Join(work, "dir")
	writeFile(t, filepath.Join(path, "f"), "x")
	item, err := trash.Trash(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := trash.Delete(item.TrashedPath); err != nil {
		t.Fatal(err)
	}
	if exists(item.TrashedPath) || exists(item.InfoPath) {
		t.Fatal("trash entry not fully deleted")
	}

	plain := filepath.Join(work, "plain.txt")
	writeFile(t, plain, "x")
	if err := trash.Delete(plain); err != nil || exists(plain) {
		t.Fatalf("plain delete failed: %v", err)
	}
}

func TestDeleteNestedTrashEntryKeepsInfo(t *testing.T) {
	work, _ := setup(t)
	dir := filepath.Join(work, "dir")
	writeFile(t, filepath.Join(dir, "inner.txt"), "x")
	item, err := trash.Trash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := trash.Delete(filepath.Join(item.TrashedPath, "inner.txt")); err != nil {
		t.Fatal(err)
	}
	if !exists(item.InfoPath) || !exists(item.TrashedPath) {
		t.Fatal("deleting nested entry removed parent metadata")
	}
}

func TestTrashSymlinkTrashesLink(t *testing.T) {
	work, _ := setup(t)
	target := filepath.Join(work, "target.txt")
	link := filepath.Join(work, "link")
	writeFile(t, target, "x")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	item, err := trash.Trash(link)
	if err != nil {
		t.Fatal(err)
	}
	if !exists(target) || exists(link) {
		t.Fatal("symlink target was affected")
	}
	fi, err := os.Lstat(item.TrashedPath)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("trashed entry is not a symlink: %v", err)
	}
	if _, err := trash.Restore(item.TrashedPath); err != nil {
		t.Fatal(err)
	}
	if dest, err := os.Readlink(link); err != nil || dest != target {
		t.Fatalf("link not restored: %s %v", dest, err)
	}
}

func TestTrashDanglingSymlink(t *testing.T) {
	work, _ := setup(t)
	link := filepath.Join(work, "dangling")
	if err := os.Symlink(filepath.Join(work, "missing"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := trash.Trash(link); err != nil {
		t.Fatal(err)
	}
	if exists(link) {
		t.Fatal("dangling link not trashed")
	}
}

func TestTrashRejectsTrashedItemsAndMissingPaths(t *testing.T) {
	work, _ := setup(t)
	path := filepath.Join(work, "a")
	writeFile(t, path, "x")
	item, err := trash.Trash(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := trash.Trash(item.TrashedPath); err == nil {
		t.Fatal("expected error trashing an item already in trash")
	}
	if _, err := trash.Trash(filepath.Join(work, "nope")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want not exist", err)
	}
}

func TestIsInTrashAndTrashFilesDir(t *testing.T) {
	work, home := setup(t)
	files := filepath.Join(home, "files")
	if trash.TrashFilesDir() != files {
		t.Fatalf("TrashFilesDir = %s", trash.TrashFilesDir())
	}
	cases := map[string]bool{
		files:                                    true,
		filepath.Join(files, "a", "b"):           true,
		filepath.Join(home, "info"):              false,
		work:                                     false,
		"/media/usb/.Trash-1000/files":           true,
		"/media/usb/.Trash-1000/files/x":         true,
		"/media/usb/.Trash/1000/files/x":         true,
		"/media/usb/.Trash-abc/files/x":          false,
		"/media/usb/Trash/1000/files/x":          false,
		filepath.Join(work, "files", "whatever"): false,
	}
	for path, want := range cases {
		if got := trash.IsInTrash(path); got != want {
			t.Errorf("IsInTrash(%s) = %v, want %v", path, got, want)
		}
	}
}

func TestInfoResolvesTopDirRelativePath(t *testing.T) {
	_, _ = setup(t)
	topdir := t.TempDir()
	root := filepath.Join(topdir, ".Trash-1000")
	writeFile(t, filepath.Join(root, "files", "a b.txt"), "x")
	writeFile(t, filepath.Join(root, "info", "a b.txt.trashinfo"),
		"[Trash Info]\nPath=docs/a%20b.txt\nDeletionDate=2024-05-06T07:08:09\n")

	item, err := trash.Info(filepath.Join(root, "files", "a b.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if item.OriginalPath != filepath.Join(topdir, "docs", "a b.txt") {
		t.Fatalf("original = %s", item.OriginalPath)
	}
	wantDate := time.Date(2024, 5, 6, 7, 8, 9, 0, time.Local)
	if !item.DeletedAt.Equal(wantDate) {
		t.Fatalf("date = %v", item.DeletedAt)
	}
	restored, err := trash.Restore(item.TrashedPath)
	if err != nil || readFile(t, restored) != "x" {
		t.Fatalf("restore: %v", err)
	}
}

func TestTopDirStopsAtDeviceBoundary(t *testing.T) {
	devices := map[string]uint64{
		"/":                 1,
		"/mnt":              1,
		"/mnt/usb":          2,
		"/mnt/usb/docs":     2,
		"/mnt/usb/docs/sub": 2,
	}
	deviceOf := func(p string) (uint64, error) {
		d, ok := devices[p]
		if !ok {
			return 0, os.ErrNotExist
		}
		return d, nil
	}
	got, err := trash.TopDir("/mnt/usb/docs/sub", deviceOf)
	if err != nil || got != "/mnt/usb" {
		t.Fatalf("TopDir = %s, %v", got, err)
	}
	got, err = trash.TopDir("/mnt", deviceOf)
	if err != nil || got != "/" {
		t.Fatalf("TopDir = %s, %v", got, err)
	}
}

func TestTopDirTrashPrefersStickySharedTrash(t *testing.T) {
	topdir := t.TempDir()
	shared := filepath.Join(topdir, ".Trash")
	if err := os.Mkdir(shared, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shared, 0o777|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	root, err := trash.TopDirTrash(topdir, 1000)
	if err != nil || root != filepath.Join(shared, "1000") {
		t.Fatalf("root = %s, %v", root, err)
	}
	if !exists(filepath.Join(root, "files")) || !exists(filepath.Join(root, "info")) {
		t.Fatal("trash subdirs not created")
	}
}

func TestTopDirTrashFallsBack(t *testing.T) {
	// Non-sticky .Trash directory.
	topdir := t.TempDir()
	if err := os.Mkdir(filepath.Join(topdir, ".Trash"), 0o777); err != nil {
		t.Fatal(err)
	}
	root, err := trash.TopDirTrash(topdir, 42)
	if err != nil || root != filepath.Join(topdir, ".Trash-42") {
		t.Fatalf("root = %s, %v", root, err)
	}
	fi, err := os.Stat(root)
	if err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("bad trash dir mode: %v %v", fi.Mode(), err)
	}

	// Symlinked .Trash pointing to a sticky dir must be ignored.
	topdir = t.TempDir()
	real := filepath.Join(topdir, "real")
	if err := os.Mkdir(real, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(real, 0o777|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(topdir, ".Trash")); err != nil {
		t.Fatal(err)
	}
	root, err = trash.TopDirTrash(topdir, 42)
	if err != nil || root != filepath.Join(topdir, ".Trash-42") {
		t.Fatalf("root = %s, %v", root, err)
	}
}

func TestEncodePath(t *testing.T) {
	got := trash.EncodePath("/a b/100%/é")
	if got != "/a%20b/100%25/%C3%A9" {
		t.Fatalf("EncodePath = %s", got)
	}
}
