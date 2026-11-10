package preview

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/binary"

	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

/*
Package previews: Debian packages (.deb) and AppImages. Both carry details
worth showing besides their files: a package's name, version and description,
or an AppImage's architecture and size.
*/

func isDebPath(path string) bool  { return strings.EqualFold(pathExt(path), ".deb") }
func isAppImage(path string) bool { return strings.EqualFold(pathExt(path), ".appimage") }
func pathExt(name string) string  { return path.Ext(name) }

// debCompressions maps the suffix of a .deb member to its compression.
var debCompressions = map[string]string{"": "", ".gz": "gzip", ".xz": "xz", ".zst": "zstd", ".bz2": "bzip2"}

/*
renderDeb reads a Debian package: an ar archive holding control.tar.* (the
package's details) and data.tar.* (the files it installs). Both are read
natively, through the same tar reader as tarballs.
*/
func (s *Service) renderDeb(ctx context.Context, path string) (*PackagePreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open package: %w", err)
	}
	defer file.Close()
	members, err := arMembers(file)
	if err != nil {
		return nil, fmt.Errorf("read package: %w", err)
	}
	result := &PackagePreview{Kind: "Debian package"}
	for _, member := range members {
		compression, ok := debMember(member.name, "control.tar")
		if !ok {
			continue
		}
		control, err := s.debControl(ctx, io.NewSectionReader(file, member.offset, member.size), member.size, compression)
		if err != nil {
			return nil, err
		}
		result.Metadata = control
	}
	for _, member := range members {
		compression, ok := debMember(member.name, "data.tar")
		if !ok {
			continue
		}
		// Listing decompresses everything before each entry: hundreds of MB for
		// a large app. The details are already read, so the list gets a time
		// budget and shows what it reached.
		listing, cancel := context.WithTimeout(ctx, debListingBudget)
		archive, err := s.listTarStream(listing, io.NewSectionReader(file, member.offset, member.size), member.size, "", compression)
		budgetSpent := listing.Err() != nil
		cancel()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err != nil && !budgetSpent {
			return nil, err
		}
		if budgetSpent {
			archive.Truncated = true
			result.Note = "The package is large; only the files listed in time are shown"
		}
		archive.Entries = installedPaths(archive.Entries)
		result.Archive = archive
		return result, nil
	}
	return nil, fmt.Errorf("read package: no data.tar member")
}

// debListingBudget bounds how long a package's file list may take to read.
const debListingBudget = 1500 * time.Millisecond

// installedPaths writes package paths as the absolute paths they install to:
// packages store "./usr/bin/x", "usr/bin/x" or "/usr/bin/x". Directories end
// in "/", and the root itself is left out.
func installedPaths(entries []ArchiveEntry) []ArchiveEntry {
	kept := entries[:0]
	for _, entry := range entries {
		name := "/" + strings.Trim(strings.TrimPrefix(entry.Path, "."), "/")
		if name == "/" {
			continue
		}
		if entry.Directory {
			name += "/"
		}
		entry.Path = name
		kept = append(kept, entry)
	}
	return kept
}

// debMember reports whether name is prefix plus a known compression suffix.
func debMember(name, prefix string) (string, bool) {
	suffix, ok := strings.CutPrefix(name, prefix)
	if !ok {
		return "", false
	}
	compression, known := debCompressions[suffix]
	return compression, known
}

// debControl reads the control file's fields worth showing.
func (s *Service) debControl(ctx context.Context, input io.Reader, size int64, compression string) ([]Field, error) {
	var control []byte
	err := s.walkTar(ctx, input, size, "", compression, func(header *tar.Header, contents io.Reader) bool {
		if path.Clean(header.Name) != "control" {
			return true
		}
		control, _ = io.ReadAll(io.LimitReader(contents, 256<<10))
		return false
	})
	if err != nil {
		return nil, err
	}
	values := parseDebControl(control)
	var fields []Field
	for _, field := range []struct{ key, name string }{
		{"Package", "Package"}, {"Version", "Version"}, {"Architecture", "Architecture"},
		{"Maintainer", "Maintainer"}, {"Section", "Section"}, {"Homepage", "Homepage"},
		{"Description", "Description"},
	} {
		if value := values[field.key]; value != "" {
			fields = append(fields, Field{Name: field.name, Value: value})
		}
	}
	if kib, err := strconv.ParseInt(values["Installed-Size"], 10, 64); err == nil {
		fields = append(fields, Field{Name: "Installed size", Value: formatByteSize(kib * 1024)})
	}
	return fields, nil
}

// parseDebControl reads "Key: value" lines, keeping a field's first line only
// (Description continues on indented lines).
func parseDebControl(data []byte) map[string]string {
	values := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		if key, value, ok := strings.Cut(line, ":"); ok {
			values[key] = safeText(strings.TrimSpace(value))
		}
	}
	return values
}

type arMember struct {
	name         string
	offset, size int64
}

// arMembers lists an ar archive's members: a "!<arch>" magic, then 60-byte
// headers each followed by its data, padded to an even length.
func arMembers(file *os.File) ([]arMember, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	magic := make([]byte, 8)
	if _, err := file.ReadAt(magic, 0); err != nil || string(magic) != "!<arch>\n" {
		return nil, fmt.Errorf("not an ar archive")
	}
	var members []arMember
	header := make([]byte, 60)
	for offset := int64(8); offset+60 <= info.Size(); {
		if _, err := file.ReadAt(header, offset); err != nil {
			return nil, err
		}
		size, err := strconv.ParseInt(strings.TrimSpace(string(header[48:58])), 10, 64)
		if err != nil || size < 0 || string(header[58:60]) != "`\n" {
			return nil, fmt.Errorf("damaged ar header")
		}
		name := strings.TrimSuffix(strings.TrimSpace(string(header[:16])), "/")
		members = append(members, arMember{name: name, offset: offset + 60, size: size})
		offset += 60 + size + size%2
	}
	return members, nil
}

/*
renderAppImage describes an AppImage: an ELF launcher with a squashfs image of
the app appended. The details come from the ELF and squashfs headers, without
decompressing anything. The file list comes from 7-Zip, which some builds can't
produce for zstd-compressed images; the details still show then.
*/
func (s *Service) renderAppImage(ctx context.Context, path string) (*PackagePreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open AppImage: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	elf := make([]byte, 64)
	if _, err := file.ReadAt(elf, 0); err != nil || string(elf[:4]) != "\x7fELF" {
		return nil, fmt.Errorf("read AppImage: not an ELF executable")
	}
	architecture, imageOffset := elfDetails(elf)
	result := &PackagePreview{Kind: "AppImage"}
	if architecture != "" {
		result.Metadata = append(result.Metadata, Field{Name: "Architecture", Value: architecture})
	}
	compression := ""
	if super, ok := squashfsSuperblock(file, imageOffset); ok {
		compression = super.compression
		result.Metadata = append(result.Metadata,
			Field{Name: "Files", Value: strconv.FormatUint(uint64(super.inodes), 10)},
			Field{Name: "Compression", Value: super.compression},
		)
		// Reproducible builds store a zero time, which isn't a real date.
		if super.built != 0 {
			result.Metadata = append(result.Metadata, Field{Name: "Built", Value: time.Unix(int64(super.built), 0).Format("2006-01-02 15:04")})
		}
	}
	result.Metadata = append(result.Metadata, Field{Name: "Size", Value: formatByteSize(info.Size())})

	if s.tools.SevenZip == "" {
		result.Note = "Install 7-Zip (7zz) to list its files"
		return result, nil
	}
	archive, err := s.listSevenZip(ctx, path)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		result.Note = "File list unavailable: the installed 7-Zip can't read this AppImage"
		if compression == "zstd" {
			result.Note = "File list unavailable: the installed 7-Zip can't read zstd-compressed AppImages"
		}
		return result, nil
	}
	result.Archive = archive
	return result, nil
}

// elfDetails returns the executable's architecture and where its section
// headers end, which is where an AppImage's squashfs image starts.
func elfDetails(header []byte) (string, int64) {
	order := binary.ByteOrder(binary.LittleEndian)
	if header[5] == 2 {
		order = binary.BigEndian
	}
	architectures := map[uint16]string{0x03: "x86", 0x28: "ARM", 0x3e: "x86-64", 0xb7: "ARM64", 0xf3: "RISC-V"}
	architecture := architectures[order.Uint16(header[18:])]
	if header[4] == 2 { // 64-bit
		return architecture, int64(order.Uint64(header[0x28:])) + int64(order.Uint16(header[0x3a:]))*int64(order.Uint16(header[0x3c:]))
	}
	return architecture, int64(order.Uint32(header[0x20:])) + int64(order.Uint16(header[0x2e:]))*int64(order.Uint16(header[0x30:]))
}

type squashfsInfo struct {
	inodes, built uint32
	compression   string
}

// squashfsSuperblock reads the squashfs superblock at offset, if one is there.
func squashfsSuperblock(file *os.File, offset int64) (squashfsInfo, bool) {
	block := make([]byte, 24)
	if offset <= 0 {
		return squashfsInfo{}, false
	}
	if _, err := file.ReadAt(block, offset); err != nil || string(block[:4]) != "hsqs" {
		return squashfsInfo{}, false
	}
	compressions := map[uint16]string{1: "gzip", 2: "lzma", 3: "lzo", 4: "xz", 5: "lz4", 6: "zstd"}
	compression := compressions[binary.LittleEndian.Uint16(block[20:])]
	if compression == "" {
		compression = "unknown"
	}
	return squashfsInfo{
		inodes:      binary.LittleEndian.Uint32(block[4:]),
		built:       binary.LittleEndian.Uint32(block[8:]),
		compression: compression,
	}, true
}
