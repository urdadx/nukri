package archive_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/archive"
)

// writeTar builds a .tar from entries in order; a nil body makes a directory.
func writeTar(t *testing.T, path string, entries []tarEntry) {
	t.Helper()
	var data bytes.Buffer
	writer := tar.NewWriter(&data)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: entry.mode, Typeflag: tar.TypeReg, Size: int64(len(entry.body))}
		if entry.body == nil {
			header.Typeflag, header.Size = tar.TypeDir, 0
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

type tarEntry struct {
	name string
	mode int64
	body []byte
}

func TestExtractStopsAtByteLimit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bomb.tar")
	writeTar(t, path, []tarEntry{
		{name: "a.bin", mode: 0o644, body: make([]byte, 600)},
		{name: "b.bin", mode: 0o644, body: make([]byte, 600)},
	})
	_, err := archive.ExtractWithLimits(path, nil, archive.ExtractLimits{MaxBytes: 1000})
	if !errors.Is(err, archive.ErrTooLarge) {
		t.Fatalf("error = %v, want ErrTooLarge", err)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 1 {
		t.Fatalf("a failed extraction must leave nothing behind, found %d entries", len(entries))
	}
}

func TestExtractStopsAtEntryLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "many.tar")
	writeTar(t, path, []tarEntry{
		{name: "a", mode: 0o644, body: []byte("a")},
		{name: "b", mode: 0o644, body: []byte("b")},
		{name: "c", mode: 0o644, body: []byte("c")},
	})
	if _, err := archive.ExtractWithLimits(path, nil, archive.ExtractLimits{MaxEntries: 2}); !errors.Is(err, archive.ErrTooLarge) {
		t.Fatalf("error = %v, want ErrTooLarge", err)
	}
	if _, err := archive.ExtractWithLimits(path, nil, archive.ExtractLimits{MaxEntries: 3, MaxBytes: 3}); err != nil {
		t.Fatalf("an archive exactly at its limits should extract: %v", err)
	}
}

// A read-only directory listed before its contents must not block them, and
// keeps its mode once extraction finishes.
func TestExtractHandlesReadOnlyDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked.tar")
	writeTar(t, path, []tarEntry{
		{name: "locked/", mode: 0o555},
		{name: "locked/inside.txt", mode: 0o644, body: []byte("inside")},
	})
	output, err := archive.Extract(path)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(output, "locked"), 0o755) })
	if content, err := os.ReadFile(filepath.Join(output, "locked", "inside.txt")); err != nil || string(content) != "inside" {
		t.Fatalf("content = %q, %v", content, err)
	}
	if info, err := os.Stat(filepath.Join(output, "locked")); err != nil || info.Mode().Perm() != 0o555 {
		t.Fatalf("directory mode = %v, %v; want 0555", info.Mode().Perm(), err)
	}
	if info, err := os.Stat(output); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("extracted folder mode = %v, %v; want 0755", info.Mode().Perm(), err)
	}
}

func TestCreateStoresCompressedFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "files")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"photo.jpg", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(source, name), bytes.Repeat([]byte("x"), 1000), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	output, err := archive.Create(filepath.Join(root, "files.zip"), []string{source})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	methods := map[string]uint16{}
	for _, file := range reader.File {
		methods[filepath.Base(file.Name)] = file.Method
	}
	if methods["photo.jpg"] != zip.Store || methods["notes.txt"] != zip.Deflate {
		t.Fatalf("methods = %v, want jpg stored and txt deflated", methods)
	}
}
