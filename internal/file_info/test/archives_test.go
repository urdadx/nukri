package test

import (
	"testing"

	"github.com/urdadx/nukri/internal/core"
	. "github.com/urdadx/nukri/internal/file_info"
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
			facts := InspectPath(name, core.File)
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
	kind, ok := CompressedDiskImageKind("disk.qcow2.zst", ".zst", Zstd)
	if !ok || kind.Kind != CompressedDiskImage || kind.Image != Qcow2 || kind.Compression != Zstd {
		t.Fatalf("CompressedDiskImageKind() = (%+v, %t), want a Zstd-compressed QCOW2 image", kind, ok)
	}

	if _, ok := CompressedDiskImageKind("notes.txt", ".zst", Zstd); ok {
		t.Fatal("plain text file was classified as a disk image")
	}
}
