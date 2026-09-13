package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/fs/fileops"
)

func TestApplyDropCopiesRecursivelyAndAddsCollisionSuffix(t *testing.T) {
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

	result := fileops.ApplyDrop(destination, []string{source}, fileops.Copy)
	want := filepath.Join(destination, "folder_1")
	if len(result.Errors) != 0 || len(result.Destinations) != 1 || result.Destinations[0] != want {
		t.Fatalf("unexpected result: %+v", result)
	}
	data, err := os.ReadFile(filepath.Join(want, "nested", "report.txt"))
	if err != nil || string(data) != "content" {
		t.Fatalf("copied data = %q, %v", data, err)
	}
}

func TestApplyDropMovesFile(t *testing.T) {
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

	result := fileops.ApplyDrop(destination, []string{source}, fileops.Move)
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

func TestApplyDropRejectsDirectoryIntoItself(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(source, "nested")
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatal(err)
	}

	result := fileops.ApplyDrop(destination, []string{source}, fileops.Copy)
	if len(result.Destinations) != 0 || len(result.Errors) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestApplyDropPreservesSymlink(t *testing.T) {
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

	result := fileops.ApplyDrop(destination, []string{link}, fileops.Copy)
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if target, err := os.Readlink(filepath.Join(destination, "link")); err != nil || target != "target" {
		t.Fatalf("link target = %q, %v", target, err)
	}
}
