package test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/fileinfo"
	"github.com/urdadx/nukri/internal/preview"
)

// These fixtures are built from the RAR 4 and RAR 5 format notes. CRCs are left
// zero: listing reads headers only and does not verify them.

type rar4File struct {
	name, unicode []byte // unicode is the encoded name part, if any
	data          []byte
	windows       bool
	attributes    uint32
	directory     bool
	large         bool
	modified      time.Time
}

func rar4Archive(encrypted bool, files ...rar4File) []byte {
	var out bytes.Buffer
	out.WriteString("Rar!\x1a\x07\x00")
	mainFlags := uint16(0)
	if encrypted {
		mainFlags = 0x0080
	}
	header := make([]byte, 13)
	header[2] = 0x73
	binary.LittleEndian.PutUint16(header[3:], mainFlags)
	binary.LittleEndian.PutUint16(header[5:], 13)
	out.Write(header)
	for _, file := range files {
		name := file.name
		flags := uint16(0x8000)
		if file.unicode != nil {
			flags |= 0x0200
			name = append(append(append([]byte(nil), file.name...), 0), file.unicode...)
		}
		if file.directory {
			flags |= 0x00e0
		}
		fixed := 32
		if file.large {
			flags |= 0x0100
			fixed += 8
		}
		header := make([]byte, fixed+len(name))
		header[2] = 0x74
		binary.LittleEndian.PutUint16(header[3:], flags)
		binary.LittleEndian.PutUint16(header[5:], uint16(len(header)))
		binary.LittleEndian.PutUint32(header[7:], uint32(len(file.data)))
		binary.LittleEndian.PutUint32(header[11:], uint32(len(file.data)))
		header[15] = 3
		if file.windows {
			header[15] = 2
		}
		m := file.modified
		date := uint32(m.Year()-1980)<<9 | uint32(m.Month())<<5 | uint32(m.Day())
		clock := uint32(m.Hour())<<11 | uint32(m.Minute())<<5 | uint32(m.Second()/2)
		binary.LittleEndian.PutUint32(header[20:], date<<16|clock)
		binary.LittleEndian.PutUint16(header[26:], uint16(len(name)))
		binary.LittleEndian.PutUint32(header[28:], file.attributes)
		copy(header[fixed:], name)
		out.Write(header)
		out.Write(file.data)
	}
	end := make([]byte, 7)
	end[2] = 0x7b
	binary.LittleEndian.PutUint16(end[3:], 0x4000)
	binary.LittleEndian.PutUint16(end[5:], 7)
	out.Write(end)
	return out.Bytes()
}

func vint(value uint64) []byte {
	var out []byte
	for {
		b := byte(value & 0x7f)
		value >>= 7
		if value == 0 {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

// rar5Block frames header fields (starting at the header type) as a block.
func rar5Block(fields ...[]byte) []byte {
	body := bytes.Join(fields, nil)
	return append(append(make([]byte, 4), vint(uint64(len(body)))...), body...)
}

type rar5File struct {
	name      string
	data      []byte
	directory bool
	unix      bool
	mode      uint64
	modified  uint32
}

func rar5Archive(files ...rar5File) []byte {
	var out bytes.Buffer
	out.WriteString("Rar!\x1a\x07\x01\x00")
	out.Write(rar5Block(vint(1), vint(0), vint(0)))
	for _, file := range files {
		fileFlags := uint64(0x0002 | 0x0004) // time and CRC present
		if file.directory {
			fileFlags |= 0x0001
		}
		host := uint64(0)
		if file.unix {
			host = 1
		}
		times := make([]byte, 8)
		binary.LittleEndian.PutUint32(times, file.modified)
		fields := [][]byte{vint(2)}
		if len(file.data) > 0 {
			fields = append(fields, vint(0x0002), vint(uint64(len(file.data))))
		} else {
			fields = append(fields, vint(0))
		}
		fields = append(fields, vint(fileFlags), vint(uint64(len(file.data))), vint(file.mode), times,
			vint(0), vint(host), vint(uint64(len(file.name))), []byte(file.name))
		out.Write(rar5Block(fields...))
		out.Write(file.data)
	}
	out.Write(rar5Block(vint(5), vint(0), vint(0)))
	return out.Bytes()
}

func renderRAR(t *testing.T, data []byte) (preview.Archive, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.rar")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	// No 7-Zip: RAR must list without any tool.
	result, err := preview.NewServiceWithTools(preview.Tools{}).Render(context.Background(), preview.Request{Path: path, Facts: fileinfo.InspectPath(path, core.File)})
	if err != nil {
		return preview.Archive{}, err
	}
	return result.(*preview.ArchivePreview).Archive, nil
}

func TestRenderRAR4(t *testing.T) {
	modified := time.Date(2012, 12, 13, 18, 45, 8, 0, time.Local)
	data := rar4Archive(false,
		rar4File{name: []byte(`docs\readme.txt`), data: []byte("hello"), windows: true, attributes: 0x20, modified: modified},
		// Mode 3 copies the plain name; mode 2 spells out full 16-bit characters.
		rar4File{name: []byte("readme.txt"), unicode: []byte{0, 0xc0, 8}, data: []byte("x"), modified: modified, attributes: 0o644},
		// Each flag byte (0xaa: four mode-2 characters) precedes its characters.
		rar4File{name: []byte("e?e.txt"), unicode: []byte{0, 0xaa, 0xe9, 0, 't', 0, 0xe9, 0, '.', 0, 0xaa, 't', 0, 'x', 0, 't', 0}, modified: modified, attributes: 0o644},
		rar4File{name: []byte("big.bin"), data: []byte("123"), large: true, modified: modified, attributes: 0o600},
		rar4File{name: []byte("folder"), directory: true, modified: modified, attributes: 0o755},
	)
	archive, err := renderRAR(t, data)
	if err != nil {
		t.Fatal(err)
	}
	want := []preview.ArchiveEntry{
		{Path: "docs/readme.txt", Size: 5, PackedSize: 5, Modified: "2012-12-13 18:45:08", Attributes: "A"},
		{Path: "readme.txt", Size: 1, PackedSize: 1, Modified: "2012-12-13 18:45:08", Attributes: "-rw-r--r--"},
		{Path: "été.txt", Modified: "2012-12-13 18:45:08", Attributes: "-rw-r--r--"},
		{Path: "big.bin", Size: 3, PackedSize: 3, Modified: "2012-12-13 18:45:08", Attributes: "-rw-------"},
		{Path: "folder/", Modified: "2012-12-13 18:45:08", Attributes: "drwxr-xr-x", Directory: true},
	}
	if len(archive.Entries) != len(want) {
		t.Fatalf("entries = %#v", archive.Entries)
	}
	for index := range want {
		if archive.Entries[index] != want[index] {
			t.Fatalf("entry %d = %#v, want %#v", index, archive.Entries[index], want[index])
		}
	}
}

func TestRenderRAR5(t *testing.T) {
	data := rar5Archive(
		rar5File{name: "src/main.go", data: []byte("package main"), unix: true, mode: 0o644, modified: 1700000000},
		rar5File{name: "src", directory: true, unix: true, mode: 0o755, modified: 1700000000},
		rar5File{name: "Ünïcode name.txt", data: []byte("x"), mode: 0x20, modified: 1700000000},
	)
	archive, err := renderRAR(t, data)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1700000000, 0).Format("2006-01-02 15:04:05")
	want := []preview.ArchiveEntry{
		{Path: "src/main.go", Size: 12, PackedSize: 12, Modified: stamp, Attributes: "-rw-r--r--"},
		{Path: "src/", Modified: stamp, Attributes: "drwxr-xr-x", Directory: true},
		{Path: "Ünïcode name.txt", Size: 1, PackedSize: 1, Modified: stamp, Attributes: "A"},
	}
	if len(archive.Entries) != len(want) {
		t.Fatalf("entries = %#v", archive.Entries)
	}
	for index := range want {
		if archive.Entries[index] != want[index] {
			t.Fatalf("entry %d = %#v, want %#v", index, archive.Entries[index], want[index])
		}
	}
}

func TestRenderRARWithEncryptedNames(t *testing.T) {
	rar5 := append([]byte("Rar!\x1a\x07\x01\x00"), rar5Block(vint(4), vint(0), vint(0), vint(0))...)
	for name, data := range map[string][]byte{"rar4": rar4Archive(true), "rar5": rar5} {
		if _, err := renderRAR(t, data); err == nil || !strings.Contains(err.Error(), "encrypted") {
			t.Fatalf("%s: error = %v, want encrypted names", name, err)
		}
	}
}

// Every prefix cut after the signature must list what it can or fail; never
// panic. Shorter than the signature, a file isn't RAR and other readers try it.
func TestRenderRARSurvivesTruncation(t *testing.T) {
	modified := time.Date(2020, 1, 2, 3, 4, 6, 0, time.Local)
	for name, data := range map[string][]byte{
		"rar4": rar4Archive(false, rar4File{name: []byte("a.txt"), data: []byte("abc"), modified: modified}, rar4File{name: []byte("b.txt"), data: []byte("de"), modified: modified}),
		"rar5": rar5Archive(rar5File{name: "a.txt", data: []byte("abc")}, rar5File{name: "b.txt", data: []byte("de")}),
	} {
		for length := 8; length < len(data); length++ {
			archive, err := renderRAR(t, data[:length])
			if err == nil && len(archive.Entries) > 2 {
				t.Fatalf("%s cut to %d bytes listed %d entries", name, length, len(archive.Entries))
			}
			var tool *preview.ToolUnavailableError
			if errors.As(err, &tool) {
				t.Fatalf("%s cut to %d bytes fell through to 7-Zip: %v", name, length, err)
			}
		}
	}
}
