package preview

import (
	"archive/tar"
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// tarCompressions maps tarball suffixes to their compression; "" is plain tar.
var tarCompressions = []struct{ suffix, compression string }{
	{".tar", ""},
	{".tar.gz", "gzip"}, {".tgz", "gzip"},
	{".tar.bz2", "bzip2"}, {".tbz2", "bzip2"}, {".tbz", "bzip2"},
	{".tar.xz", "xz"}, {".txz", "xz"},
	{".tar.zst", "zstd"}, {".tzst", "zstd"},
}

func tarCompression(path string) (string, bool) {
	lower := strings.ToLower(path)
	for _, candidate := range tarCompressions {
		if strings.HasSuffix(lower, candidate.suffix) {
			return candidate.compression, true
		}
	}
	return "", false
}

/*
listTar lists a tarball's entries by reading tar headers directly. 7-Zip lists a
compressed tarball as the single .tar inside it, or not at all for xz and zstd,
so it never showed their files. A plain tar is read from the file, which lets
the tar reader seek past file data; a compressed one is decompressed as a
stream: by the standard library for gzip (or pigz, for large files, when
installed) and bzip2, and by the xz or zstd tool otherwise, and reading stops once maxArchiveEntries are
listed.
*/
func (s *Service) listTar(ctx context.Context, path, compression string) (*ArchivePreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open tar: %w", err)
	}
	defer file.Close()
	archive, err := s.listTarStream(ctx, file, fileSize(file), path, compression)
	if err != nil {
		return nil, err
	}
	return &ArchivePreview{Archive: archive}, nil
}

// listTarStream lists the tar in input, compressed as compression says. path
// names input on disk when it is a whole file, letting external tools read it
// directly; otherwise they read input on standard input.
func (s *Service) listTarStream(ctx context.Context, input io.Reader, size int64, path, compression string) (Archive, error) {
	archive := Archive{}
	err := s.walkTar(ctx, input, size, path, compression, func(header *tar.Header, _ io.Reader) bool {
		if len(archive.Entries) >= s.maxArchiveEntries {
			archive.Truncated = true
			return false
		}
		mode := header.FileInfo().Mode()
		archive.Entries = append(archive.Entries, ArchiveEntry{
			Path: safeText(header.Name),
			Size: header.Size,
			// Tar stores files uncompressed; any compression covers the whole
			// stream, so no per-entry packed size exists.
			PackedSize: header.Size,
			Modified:   zipTimeLabel(header.ModTime),
			Attributes: mode.String(),
			Directory:  mode.IsDir(),
		})
		return true
	})
	if err != nil && len(archive.Entries) == 0 {
		return Archive{}, err
	}
	if err != nil {
		// A damaged or cut-off tail: keep what was listed.
		archive.Truncated = true
	}
	return archive, nil
}

// walkTar calls visit with each header of the tar in input, and a reader for
// that entry's contents, until visit returns false or the tar ends.
func (s *Service) walkTar(ctx context.Context, input io.Reader, size int64, path, compression string, visit func(*tar.Header, io.Reader) bool) error {
	stream, done, err := s.decompress(ctx, input, size, path, compression)
	if err != nil {
		return err
	}
	// Stops an external decompressor, which may still be writing.
	defer done()
	reader := tar.NewReader(stream)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if !visit(header, reader) {
			return nil
		}
	}
}

// decompress returns input decompressed: in-process for gzip (or by pigz for
// large files, when installed) and bzip2, and by the xz or zstd tool otherwise.
// A plain tar is input itself, so a file can be seeked through.
func (s *Service) decompress(ctx context.Context, input io.Reader, size int64, path, compression string) (io.Reader, func(), error) {
	none := func() {}
	tool := ""
	switch {
	case compression == "":
		return input, none, nil
	case compression == "gzip" && s.tools.Pigz != "" && size > pigzMinimumBytes:
		// pigz decompresses about twice as fast as compress/gzip, which pays
		// for starting it only on large files.
		tool = s.tools.Pigz
	case compression == "gzip":
		decompressed, err := gzip.NewReader(bufio.NewReaderSize(input, 256<<10))
		if err != nil {
			return nil, nil, fmt.Errorf("read tar: %w", err)
		}
		return decompressed, func() { decompressed.Close() }, nil
	case compression == "bzip2":
		return bzip2.NewReader(bufio.NewReaderSize(input, 256<<10)), none, nil
	case compression == "xz":
		tool = s.tools.XZ
	case compression == "zstd":
		tool = s.tools.Zstd
	default:
		return nil, nil, fmt.Errorf("unsupported compression %q", compression)
	}
	if tool == "" {
		return nil, nil, fmt.Errorf("archive preview: %w", ToolUnavailable(compression))
	}
	var stdin io.Reader
	if path == "" {
		stdin = input
	}
	decompressor, err := startDecompressor(ctx, tool, path, stdin)
	if err != nil {
		return nil, nil, err
	}
	return decompressor.output, decompressor.stop, nil
}

// pigzMinimumBytes is the gzip tarball size from which pigz beats starting
// nothing and decompressing in-process.
const pigzMinimumBytes = 32 << 20

func fileSize(file *os.File) int64 {
	info, err := file.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}

// decompressor streams a file through an external decompression tool.
type decompressor struct {
	command *exec.Cmd
	output  io.ReadCloser
}

// startDecompressor runs tool on path, or on stdin when it is set.
func startDecompressor(ctx context.Context, tool, path string, stdin io.Reader) (*decompressor, error) {
	command := exec.CommandContext(ctx, tool, "-dc", "--", path)
	if stdin != nil {
		command = exec.CommandContext(ctx, tool, "-dc")
		command.Stdin = stdin
	}
	output, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", tool, err)
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", tool, err)
	}
	return &decompressor{command: command, output: output}, nil
}

// stop ends the tool, which may still be writing when listing stops early.
func (d *decompressor) stop() {
	_ = d.command.Process.Kill()
	_ = d.command.Wait()
}
