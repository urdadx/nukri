package test

import (
	"testing"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/fileinfo"
)

func TestInspectArchiveName(t *testing.T) {
	tests := map[string]string{
		"books.cbz":       "Comic ZIP archive",
		"books.cbr":       "Comic RAR archive",
		"application.apk": "Android package",
		"cards.apkg":      "Anki package",
		"backup.tar.xz":   "TAR.XZ archive",
		"backup.tbz":      "TAR.BZ2 archive",
		"backup.tzst":     "TAR.ZST archive",
		"disk.qcow2.zst":  "Zstandard-compressed QCOW2 disk image",
	}

	for name, wantLabel := range tests {
		t.Run(name, func(t *testing.T) {
			facts := fileinfo.InspectPath(name, core.File)
			if facts.BuiltinClass != core.FileClassArchive {
				t.Fatalf("BuiltinClass = %v, want archive", facts.BuiltinClass)
			}
			if facts.SpecificTypeLabel == nil || *facts.SpecificTypeLabel != wantLabel {
				t.Fatalf("SpecificTypeLabel = %v, want %q", facts.SpecificTypeLabel, wantLabel)
			}
		})
	}
}

func TestCompressedDiskImageKind(t *testing.T) {
	kind, ok := fileinfo.CompressedDiskImageKind("disk.qcow2.zst", ".zst", fileinfo.Zstd)
	if !ok || kind.Kind != fileinfo.CompressedDiskImage || kind.Image != fileinfo.Qcow2 || kind.Compression != fileinfo.Zstd {
		t.Fatalf("CompressedDiskImageKind() = (%+v, %t), want a Zstd-compressed QCOW2 image", kind, ok)
	}

	if _, ok := fileinfo.CompressedDiskImageKind("notes.txt", ".zst", fileinfo.Zstd); ok {
		t.Fatal("plain text file was classified as a disk image")
	}
}
