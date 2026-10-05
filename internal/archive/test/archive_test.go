package archive_test

import (
	"archive/tar"
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/archive"
)

func TestCreateAndExtractFormats(t *testing.T) {
	for _, suffix := range []string{".zip", ".tar", ".tar.gz", ".tgz"} {
		t.Run(suffix, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "project")
			if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "nested", "file.txt"), []byte("content"), 0o640); err != nil {
				t.Fatal(err)
			}
			archivePath := filepath.Join(root, "bundle"+suffix)
			if _, err := archive.Create(archivePath, []string{source}); err != nil {
				t.Fatalf("create: %v", err)
			}
			output, err := archive.Extract(archivePath)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if filepath.Base(output) != "bundle" {
				t.Fatalf("output = %q, want full suffix removed", output)
			}
			content, err := os.ReadFile(filepath.Join(output, "project", "nested", "file.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != "content" {
				t.Fatalf("content = %q", content)
			}
		})
	}
}

func TestCreateDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	destination := filepath.Join(root, "existing.zip")
	if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Create(destination, []string{source}); err == nil {
		t.Fatal("expected existing destination error")
	}
	content, err := os.ReadFile(destination)
	if err != nil || string(content) != "original" {
		t.Fatalf("destination changed: %q, %v", content, err)
	}
}

func TestCreateRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "link")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Create(filepath.Join(root, "out.zip"), []string{link}); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestExtractUsesUniqueDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "item.txt")
	if err := os.WriteFile(source, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(root, "bundle.tar.gz")
	if _, err := archive.Create(archivePath, []string{source}); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "bundle"), 0o755); err != nil {
		t.Fatal(err)
	}
	output, err := archive.Extract(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(output) != "bundle (1)" {
		t.Fatalf("output = %q", output)
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bad.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../outside.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Extract(archivePath); err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "outside.txt")); !os.IsNotExist(err) {
		t.Fatal("archive escaped extraction directory")
	}
}

func TestExtractRejectsArchiveSymlink(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bad.tar")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := tar.NewWriter(file)
	if err := writer.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "target", Mode: 0o777}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Extract(archivePath); err == nil || !strings.Contains(err.Error(), "unsupported archive entry") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}
