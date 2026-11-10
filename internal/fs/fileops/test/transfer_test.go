package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/urdadx/nukri/internal/fs/fileops"
	"golang.org/x/sys/unix"
)

func TestApplyTransferCopiesRecursivelyAndAddsCollisionSuffix(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "folder")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "report.txt"), []byte("content"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(destination, "folder"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := fileops.ApplyTransfer(destination, []string{source}, fileops.Copy)
	want := filepath.Join(destination, "folder_1")
	if len(result.Errors) != 0 || len(result.Destinations) != 1 || result.Destinations[0] != want {
		t.Fatalf("unexpected result: %+v", result)
	}
	data, err := os.ReadFile(filepath.Join(want, "nested", "report.txt"))
	if err != nil || string(data) != "content" {
		t.Fatalf("copied data = %q, %v", data, err)
	}
}

func TestApplyTransferMovesFile(t *testing.T) {
	root := t.TempDir()
	sourceDirectory := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(sourceDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDirectory, "file.txt")
	if err := os.WriteFile(source, []byte("move"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := fileops.ApplyTransfer(destination, []string{source}, fileops.Move)
	if len(result.Errors) != 0 || len(result.Destinations) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(destination, "file.txt")); err != nil || string(data) != "move" {
		t.Fatalf("moved data = %q, %v", data, err)
	}
}

func TestApplyTransferRejectsDirectoryIntoItself(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(source, "nested")
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatal(err)
	}

	result := fileops.ApplyTransfer(destination, []string{source}, fileops.Copy)
	if len(result.Destinations) != 0 || len(result.Errors) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestApplyTransferPreservesSymlink(t *testing.T) {
	root := t.TempDir()
	sourceDirectory := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(sourceDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(sourceDirectory, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	result := fileops.ApplyTransfer(destination, []string{link}, fileops.Copy)
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if target, err := os.Readlink(filepath.Join(destination, "link")); err != nil || target != "target" {
		t.Fatalf("link target = %q, %v", target, err)
	}
}

func TestApplyTransferCopiesFileBesideItself(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "report.txt")
	if err := os.WriteFile(source, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := fileops.ApplyTransfer(directory, []string{source}, fileops.Copy)
	want := filepath.Join(directory, "report_1.txt")
	if len(result.Errors) != 0 || len(result.Destinations) != 1 || result.Destinations[0] != want {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestApplyTransferRejectsMovingFileBesideItself(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "report.txt")
	if err := os.WriteFile(source, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := fileops.ApplyTransfer(directory, []string{source}, fileops.Move)
	if len(result.Destinations) != 0 || len(result.Errors) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// A read-only folder is copied with its contents, and keeps its permissions.
func TestApplyTransferCopiesReadOnlyFolder(t *testing.T) {
	source := filepath.Join(t.TempDir(), "locked")
	if err := os.MkdirAll(filepath.Join(source, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "inner", "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, folder := range []string{filepath.Join(source, "inner"), source} {
		if err := os.Chmod(folder, 0o555); err != nil {
			t.Fatal(err)
		}
	}
	destination := t.TempDir()
	t.Cleanup(func() {
		for _, folder := range []string{source, filepath.Join(source, "inner"), filepath.Join(destination, "locked"), filepath.Join(destination, "locked", "inner")} {
			_ = os.Chmod(folder, 0o755)
		}
	})

	result := fileops.ApplyTransfer(destination, []string{source}, fileops.Copy)
	if len(result.Errors) != 0 {
		t.Fatalf("copy failed: %v", result.Errors)
	}
	copied := filepath.Join(destination, "locked")
	if data, err := os.ReadFile(filepath.Join(copied, "inner", "a.txt")); err != nil || string(data) != "a" {
		t.Fatalf("copied file = %q, %v", data, err)
	}
	for _, folder := range []string{copied, filepath.Join(copied, "inner")} {
		if info, err := os.Stat(folder); err != nil || info.Mode().Perm() != 0o555 {
			t.Fatalf("%s permissions = %v, %v; want 0555", folder, info.Mode().Perm(), err)
		}
	}
}

func TestApplyTransferReportsFailedSources(t *testing.T) {
	directory := t.TempDir()
	present := filepath.Join(directory, "present.txt")
	if err := os.WriteFile(present, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(directory, "missing.txt")
	result := fileops.ApplyTransfer(t.TempDir(), []string{present, missing}, fileops.Move)
	if len(result.Destinations) != 1 || len(result.Failed) != 1 || result.Failed[0] != missing {
		t.Fatalf("result = %+v, want present moved and missing failed", result)
	}
}

// Copies keep their source's modification time, as cp -p does, for files,
// folders and symlinks.
func TestApplyTransferKeepsModificationTimes(t *testing.T) {
	source := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(filepath.Join(source, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "docs", "notes.txt"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("docs/notes.txt", filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	// Deepest first: setting a child's time doesn't change its folder's.
	for _, path := range []string{filepath.Join(source, "docs", "notes.txt"), filepath.Join(source, "docs"), source} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	linkTime := time.Date(2021, 6, 7, 8, 9, 10, 0, time.UTC)
	if err := unix.UtimesNanoAt(unix.AT_FDCWD, filepath.Join(source, "link"), []unix.Timespec{unix.NsecToTimespec(linkTime.UnixNano()), unix.NsecToTimespec(linkTime.UnixNano())}, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		t.Fatal(err)
	}

	destination := t.TempDir()
	if result := fileops.ApplyTransfer(destination, []string{source}, fileops.Copy); len(result.Errors) != 0 {
		t.Fatalf("copy failed: %v", result.Errors)
	}
	copied := filepath.Join(destination, "project")
	for _, path := range []string{copied, filepath.Join(copied, "docs"), filepath.Join(copied, "docs", "notes.txt")} {
		if info, err := os.Stat(path); err != nil || !info.ModTime().Equal(old) {
			t.Fatalf("%s modified %v, want %v", path, info.ModTime(), old)
		}
	}
	if info, err := os.Lstat(filepath.Join(copied, "link")); err != nil || !info.ModTime().Equal(linkTime) {
		t.Fatalf("symlink modified %v, want %v", info.ModTime(), linkTime)
	}
}
