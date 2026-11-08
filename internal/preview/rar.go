package preview

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf16"
)

/*
RAR listing. Many 7-Zip builds, including Debian's and Ubuntu's, leave out RAR
for licensing reasons, so RAR files are listed here instead. A RAR file is a
chain of headers, each giving the size of the data after it, so listing reads
only the headers and seeks past the data: nothing is decompressed. Both formats
are read: RAR 1.5-4.x ("Rar!\x1a\x07\x00") and RAR 5 ("Rar!\x1a\x07\x01\x00").
*/

var (
	rar4Signature = []byte("Rar!\x1a\x07\x00")
	rar5Signature = []byte("Rar!\x1a\x07\x01\x00")

	errNotRAR       = errors.New("not a RAR archive")
	errRAREncrypted = errors.New("the archive's file names are encrypted")
)

const (
	rarSignatureSearch = 1 << 20 // self-extracting archives put a stub first
	rarMaxHeaderSize   = 2 << 20
)

func isRARPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".rar") || strings.HasSuffix(lower, ".cbr")
}

func listRAR(ctx context.Context, path string, maximumEntries int) (Archive, error) {
	file, err := os.Open(path)
	if err != nil {
		return Archive{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Archive{}, err
	}
	head := make([]byte, min(info.Size(), rarSignatureSearch))
	if _, err := io.ReadFull(file, head); err != nil {
		return Archive{}, err
	}
	reader := &rarReader{file: file, size: info.Size(), limit: maximumEntries}
	if start := bytes.Index(head, rar5Signature); start >= 0 {
		return reader.listRAR5(ctx, int64(start+len(rar5Signature)))
	}
	if start := bytes.Index(head, rar4Signature); start >= 0 {
		return reader.listRAR4(ctx, int64(start+len(rar4Signature)))
	}
	return Archive{}, errNotRAR
}

type rarReader struct {
	file    *os.File
	size    int64
	limit   int
	archive Archive
}

// add records an entry and reports whether there is room for more.
func (r *rarReader) add(entry ArchiveEntry) bool {
	if len(r.archive.Entries) >= r.limit {
		r.archive.Truncated = true
		return false
	}
	r.archive.Entries = append(r.archive.Entries, entry)
	return true
}

// read returns n bytes at offset, or io.ErrUnexpectedEOF past the end.
func (r *rarReader) read(offset int64, n int) ([]byte, error) {
	if n < 0 || n > rarMaxHeaderSize || offset < 0 || offset+int64(n) > r.size {
		return nil, io.ErrUnexpectedEOF
	}
	buffer := make([]byte, n)
	if _, err := r.file.ReadAt(buffer, offset); err != nil {
		return nil, err
	}
	return buffer, nil
}

// finish returns what was listed. A cut-off chain, such as one volume of a
// split archive, still lists the entries before the cut.
func (r *rarReader) finish(err error) (Archive, error) {
	if err != nil && len(r.archive.Entries) == 0 {
		return Archive{}, err
	}
	if err != nil {
		r.archive.Truncated = true
	}
	return r.archive, nil
}

// RAR 1.5-4.x block types and flags.
const (
	rar4MainHeader   = 0x73
	rar4FileHeader   = 0x74
	rar4EndOfArchive = 0x7b

	rar4MainEncrypted = 0x0080 // block headers are encrypted
	rar4LongBlock     = 0x8000 // ADD_SIZE follows the base header
	rar4LargeFile     = 0x0100 // 64-bit sizes
	rar4UnicodeName   = 0x0200
	rar4DirectoryMask = 0x00e0
)

func (r *rarReader) listRAR4(ctx context.Context, offset int64) (Archive, error) {
	for offset < r.size {
		if err := ctx.Err(); err != nil {
			return Archive{}, err
		}
		base, err := r.read(offset, 7)
		if err != nil {
			return r.finish(err)
		}
		blockType := base[2]
		flags := binary.LittleEndian.Uint16(base[3:])
		headerSize := int(binary.LittleEndian.Uint16(base[5:]))
		if headerSize < 7 {
			return r.finish(fmt.Errorf("RAR block header too small"))
		}
		header, err := r.read(offset, headerSize)
		if err != nil {
			return r.finish(err)
		}
		next := offset + int64(headerSize)
		switch blockType {
		case rar4MainHeader:
			if flags&rar4MainEncrypted != 0 {
				return r.finish(errRAREncrypted)
			}
		case rar4FileHeader:
			entry, packed, err := parseRAR4File(header, flags)
			if err != nil {
				return r.finish(err)
			}
			if !r.add(entry) {
				return r.archive, nil
			}
			next += packed
		case rar4EndOfArchive:
			return r.archive, nil
		default:
			if flags&rar4LongBlock != 0 && headerSize >= 11 {
				next += int64(binary.LittleEndian.Uint32(header[7:]))
			}
		}
		offset = next
	}
	return r.archive, nil
}

// parseRAR4File decodes a file header and returns the size of its data.
func parseRAR4File(header []byte, flags uint16) (ArchiveEntry, int64, error) {
	const fixed = 32 // base header plus the fixed file fields
	if len(header) < fixed {
		return ArchiveEntry{}, 0, fmt.Errorf("RAR file header too small")
	}
	packed := int64(binary.LittleEndian.Uint32(header[7:]))
	unpacked := int64(binary.LittleEndian.Uint32(header[11:]))
	hostOS := header[15]
	dosTime := binary.LittleEndian.Uint32(header[20:])
	nameSize := int(binary.LittleEndian.Uint16(header[26:]))
	attributes := binary.LittleEndian.Uint32(header[28:])
	name := fixed
	if flags&rar4LargeFile != 0 {
		if len(header) < fixed+8 {
			return ArchiveEntry{}, 0, fmt.Errorf("RAR file header too small")
		}
		packed |= int64(binary.LittleEndian.Uint32(header[32:])) << 32
		unpacked |= int64(binary.LittleEndian.Uint32(header[36:])) << 32
		name += 8
	}
	if packed < 0 || unpacked < 0 || name+nameSize > len(header) {
		return ArchiveEntry{}, 0, fmt.Errorf("RAR file header is damaged")
	}
	rawName := header[name : name+nameSize]
	path := string(rawName)
	if flags&rar4UnicodeName != 0 {
		if zero := bytes.IndexByte(rawName, 0); zero >= 0 {
			path = decodeRAR4UnicodeName(rawName[:zero], rawName[zero+1:])
		}
	}
	directory := flags&rar4DirectoryMask == rar4DirectoryMask
	return ArchiveEntry{
		Path:       rarPath(path, directory),
		Size:       unpacked,
		PackedSize: packed,
		Modified:   dosTimeLabel(dosTime),
		Attributes: rarAttributes(hostOS == 3, attributes, directory),
		Directory:  directory,
	}, packed, nil
}

/*
decodeRAR4UnicodeName decodes RAR 3's compact Unicode file names: an encoded
stream follows the plain name, and each pair of flag bits picks how the next
character is formed, often by reusing a byte of the plain name.
*/
func decodeRAR4UnicodeName(plain, encoded []byte) string {
	if len(encoded) == 0 {
		return string(plain)
	}
	high := uint16(encoded[0])
	position := 1
	var flags byte
	flagBits := 0
	var out []uint16
	next := func() (byte, bool) {
		if position >= len(encoded) {
			return 0, false
		}
		position++
		return encoded[position-1], true
	}
	for position < len(encoded) {
		if flagBits == 0 {
			flags, _ = next()
			flagBits = 8
		}
		switch flags >> 6 {
		case 0:
			low, ok := next()
			if !ok {
				return string(utf16.Decode(out))
			}
			out = append(out, uint16(low))
		case 1:
			low, ok := next()
			if !ok {
				return string(utf16.Decode(out))
			}
			out = append(out, uint16(low)|high<<8)
		case 2:
			low, ok1 := next()
			hi, ok2 := next()
			if !ok1 || !ok2 {
				return string(utf16.Decode(out))
			}
			out = append(out, uint16(low)|uint16(hi)<<8)
		case 3:
			length, ok := next()
			if !ok {
				return string(utf16.Decode(out))
			}
			if length&0x80 != 0 {
				correction, ok := next()
				if !ok {
					return string(utf16.Decode(out))
				}
				for count := int(length&0x7f) + 2; count > 0 && len(out) < len(plain); count-- {
					out = append(out, uint16(plain[len(out)]+correction)|high<<8)
				}
			} else {
				for count := int(length) + 2; count > 0 && len(out) < len(plain); count-- {
					out = append(out, uint16(plain[len(out)]))
				}
			}
		}
		flags <<= 2
		flagBits -= 2
	}
	return string(utf16.Decode(out))
}

// RAR 5 header types and flags.
const (
	rar5MainHeader       = 1
	rar5FileHeader       = 2
	rar5EncryptionHeader = 4
	rar5EndOfArchive     = 5

	rar5HasExtra     = 0x0001
	rar5HasData      = 0x0002
	rar5FileIsDir    = 0x0001
	rar5FileHasTime  = 0x0002
	rar5FileHasCRC   = 0x0004
	rar5HostUnix     = 1
	rar5MaxVintBytes = 10
)

func (r *rarReader) listRAR5(ctx context.Context, offset int64) (Archive, error) {
	for offset < r.size {
		if err := ctx.Err(); err != nil {
			return Archive{}, err
		}
		// CRC32, then the header size as a variable-length integer.
		prefix, err := r.read(offset, int(min(r.size-offset, 4+rar5MaxVintBytes)))
		if err != nil || len(prefix) < 5 {
			return r.finish(io.ErrUnexpectedEOF)
		}
		size, sizeBytes, ok := readVint(prefix[4:])
		if !ok || size == 0 || size > rarMaxHeaderSize {
			return r.finish(fmt.Errorf("RAR header is damaged"))
		}
		start := offset + 4 + int64(sizeBytes)
		header, err := r.read(start, int(size))
		if err != nil {
			return r.finish(err)
		}
		fields := &vintFields{data: header}
		headerType := fields.next()
		headerFlags := fields.next()
		if headerFlags&rar5HasExtra != 0 {
			fields.next() // extra area size; its records are not needed
		}
		var dataSize uint64
		if headerFlags&rar5HasData != 0 {
			dataSize = fields.next()
		}
		if fields.failed || dataSize > uint64(r.size) {
			return r.finish(fmt.Errorf("RAR header is damaged"))
		}
		switch headerType {
		case rar5EncryptionHeader:
			return r.finish(errRAREncrypted)
		case rar5FileHeader:
			entry, err := parseRAR5File(fields, int64(dataSize))
			if err != nil {
				return r.finish(err)
			}
			if !r.add(entry) {
				return r.archive, nil
			}
		case rar5EndOfArchive:
			return r.archive, nil
		}
		offset = start + int64(size) + int64(dataSize)
	}
	return r.archive, nil
}

func parseRAR5File(fields *vintFields, packed int64) (ArchiveEntry, error) {
	fileFlags := fields.next()
	unpacked := fields.next()
	attributes := fields.next()
	var modified uint32
	if fileFlags&rar5FileHasTime != 0 {
		modified = fields.uint32()
	}
	if fileFlags&rar5FileHasCRC != 0 {
		fields.uint32()
	}
	fields.next() // compression information
	hostOS := fields.next()
	name := fields.bytes(int(fields.next()))
	if fields.failed {
		return ArchiveEntry{}, fmt.Errorf("RAR file header is damaged")
	}
	directory := fileFlags&rar5FileIsDir != 0
	entry := ArchiveEntry{
		Path:       rarPath(string(name), directory),
		Size:       int64(min(unpacked, 1<<62)),
		PackedSize: packed,
		Attributes: rarAttributes(hostOS == rar5HostUnix, uint32(attributes), directory),
		Directory:  directory,
	}
	if fileFlags&rar5FileHasTime != 0 {
		entry.Modified = zipTimeLabel(time.Unix(int64(modified), 0))
	}
	return entry, nil
}

// vintFields reads RAR 5's little-endian base-128 integers and fixed fields
// from a header, remembering whether any read ran past its end.
type vintFields struct {
	data   []byte
	failed bool
}

func (f *vintFields) next() uint64 {
	value, size, ok := readVint(f.data)
	if !ok {
		f.failed = true
		return 0
	}
	f.data = f.data[size:]
	return value
}

func (f *vintFields) uint32() uint32 {
	if len(f.data) < 4 {
		f.failed = true
		return 0
	}
	value := binary.LittleEndian.Uint32(f.data)
	f.data = f.data[4:]
	return value
}

func (f *vintFields) bytes(n int) []byte {
	if n < 0 || n > len(f.data) {
		f.failed = true
		return nil
	}
	value := f.data[:n]
	f.data = f.data[n:]
	return value
}

func readVint(data []byte) (uint64, int, bool) {
	var value uint64
	for index := 0; index < len(data) && index < rar5MaxVintBytes; index++ {
		value |= uint64(data[index]&0x7f) << (7 * index)
		if data[index]&0x80 == 0 {
			return value, index + 1, true
		}
	}
	return 0, 0, false
}

// rarPath turns a stored name into a listing path; archives made on Windows
// use backslashes.
func rarPath(name string, directory bool) string {
	name = safeText(strings.ReplaceAll(strings.ToValidUTF8(name, "�"), "\\", "/"))
	if directory && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	return name
}

// rarAttributes formats Unix modes like other listings, and DOS attributes as
// the letters 7-Zip shows (D, R, H, S, A).
func rarAttributes(unix bool, attributes uint32, directory bool) string {
	if unix {
		mode := os.FileMode(attributes & 0o777)
		if directory {
			mode |= os.ModeDir
		}
		return mode.String()
	}
	var letters strings.Builder
	for _, attribute := range []struct {
		bit    uint32
		letter byte
	}{{0x10, 'D'}, {0x01, 'R'}, {0x02, 'H'}, {0x04, 'S'}, {0x20, 'A'}} {
		if attributes&attribute.bit != 0 || (attribute.letter == 'D' && directory) {
			letters.WriteByte(attribute.letter)
		}
	}
	return letters.String()
}

// dosTimeLabel formats an MS-DOS date and time, as RAR 4 stores them.
func dosTimeLabel(value uint32) string {
	if value == 0 {
		return ""
	}
	date, clock := value>>16, value&0xffff
	return zipTimeLabel(time.Date(int(date>>9)+1980, time.Month(date>>5&0x0f), int(date&0x1f),
		int(clock>>11), int(clock>>5&0x3f), int(clock&0x1f)*2, 0, time.Local))
}
