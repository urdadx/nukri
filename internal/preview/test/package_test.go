package test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/fileinfo"
	"github.com/urdadx/nukri/internal/preview"
)

func gzipTar(t *testing.T, files map[string]string, order []string) []byte {
	t.Helper()
	var data bytes.Buffer
	compressor := gzip.NewWriter(&data)
	writer := tar.NewWriter(compressor)
	for _, name := range order {
		header := &tar.Header{Name: name, Mode: 0o644, Typeflag: tar.TypeReg, Size: int64(len(files[name]))}
		if strings.HasSuffix(name, "/") {
			header.Typeflag, header.Mode, header.Size = tar.TypeDir, 0o755, 0
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		writer.Write([]byte(files[name]))
	}
	writer.Close()
	compressor.Close()
	return data.Bytes()
}

// arArchive builds an ar archive, as dpkg-deb writes packages.
func arArchive(members ...[2]string) []byte {
	var out bytes.Buffer
	out.WriteString("!<arch>\n")
	for _, member := range members {
		fmt.Fprintf(&out, "%-16s%-12s%-6s%-6s%-8s%-10d`\n", member[0], "0", "0", "0", "100644", len(member[1]))
		out.WriteString(member[1])
		if len(member[1])%2 == 1 {
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

func TestRenderDebianPackage(t *testing.T) {
	control := "Package: hello\nVersion: 2.10-3\nArchitecture: amd64\nMaintainer: Someone <a@b.c>\nInstalled-Size: 280\n" +
		"Description: example package\n more description that is not shown\n"
	controlTar := gzipTar(t, map[string]string{"./control": control}, []string{"./", "./control"})
	// Packages write paths as "./usr/..." or "usr/..."; both list as installed.
	dataTar := gzipTar(t, map[string]string{"./usr/bin/hello": "binary", "usr/share/doc/hello/README": "read me"},
		[]string{"./", "./usr/", "./usr/bin/", "./usr/bin/hello", "usr/share/doc/hello/README"})
	path := filepath.Join(t.TempDir(), "hello_2.10-3_amd64.deb")
	package_ := arArchive([2]string{"debian-binary", "2.0\n"}, [2]string{"control.tar.gz", string(controlTar)}, [2]string{"data.tar.gz", string(dataTar)})
	if err := os.WriteFile(path, package_, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := preview.NewServiceWithTools(preview.Tools{}).Render(context.Background(), preview.Request{Path: path, Facts: fileinfo.InspectPath(path, core.File)})
	if err != nil {
		t.Fatal(err)
	}
	deb := result.(*preview.PackagePreview)
	fields := map[string]string{}
	for _, field := range deb.Metadata {
		fields[field.Name] = field.Value
	}
	if fields["Package"] != "hello" || fields["Version"] != "2.10-3" || fields["Description"] != "example package" || fields["Installed size"] != "280.0 KiB" {
		t.Fatalf("details = %v", fields)
	}
	var paths []string
	for _, entry := range deb.Archive.Entries {
		paths = append(paths, entry.Path)
	}
	want := []string{"/usr/", "/usr/bin/", "/usr/bin/hello", "/usr/share/doc/hello/README"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

// An AppImage's details come from its ELF and squashfs headers alone.
func TestRenderAppImageDetails(t *testing.T) {
	const imageOffset = 4096
	elf := make([]byte, 64)
	copy(elf, "\x7fELF")
	elf[4], elf[5] = 2, 1 // 64-bit, little-endian
	binary.LittleEndian.PutUint16(elf[18:], 0x3e)
	// Section headers end at shoff + shentsize*shnum, where the image starts.
	binary.LittleEndian.PutUint64(elf[0x28:], imageOffset-64*2)
	binary.LittleEndian.PutUint16(elf[0x3a:], 64)
	binary.LittleEndian.PutUint16(elf[0x3c:], 2)
	file := make([]byte, imageOffset+96)
	copy(file, elf)
	super := file[imageOffset:]
	copy(super, "hsqs")
	binary.LittleEndian.PutUint32(super[4:], 42) // inodes
	binary.LittleEndian.PutUint32(super[8:], 0)  // a reproducible build: no date
	binary.LittleEndian.PutUint16(super[20:], 6) // zstd
	path := filepath.Join(t.TempDir(), "App-x86_64.AppImage")
	if err := os.WriteFile(path, file, 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := preview.NewServiceWithTools(preview.Tools{}).Render(context.Background(), preview.Request{Path: path, Facts: fileinfo.InspectPath(path, core.File)})
	if err != nil {
		t.Fatal(err)
	}
	app := result.(*preview.PackagePreview)
	fields := map[string]string{}
	for _, field := range app.Metadata {
		fields[field.Name] = field.Value
	}
	if fields["Architecture"] != "x86-64" || fields["Files"] != "42" || fields["Compression"] != "zstd" {
		t.Fatalf("details = %v", fields)
	}
	if _, ok := fields["Built"]; ok {
		t.Fatalf("a zero build time should not show: %v", fields)
	}
	if !strings.Contains(app.Note, "7-Zip") {
		t.Fatalf("note = %q, want a hint to install 7-Zip", app.Note)
	}
}
