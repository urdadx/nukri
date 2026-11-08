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

	var stream io.Reader = file
	switch {
	case compression == "gzip" && s.tools.Pigz != "" && fileSize(file) > pigzMinimumBytes:
		// pigz decompresses about twice as fast as compress/gzip, which pays
		// for starting it only on large files.
		decompressor, err := startDecompressor(ctx, s.tools.Pigz, path)
		if err != nil {
			return nil, err
		}
		defer decompressor.stop()
		stream = decompressor.output
	case compression == "gzip":
		decompressed, err := gzip.NewReader(bufio.NewReaderSize(file, 256<<10))
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		defer decompressed.Close()
		stream = decompressed
	case compression == "bzip2":
		stream = bzip2.NewReader(bufio.NewReaderSize(file, 256<<10))
	case compression == "xz" || compression == "zstd":
		tool := s.tools.XZ
		if compression == "zstd" {
			tool = s.tools.Zstd
		}
		if tool == "" {
			return nil, fmt.Errorf("archive preview: %w", ToolUnavailable(compression))
		}
		decompressor, err := startDecompressor(ctx, tool, path)
		if err != nil {
			return nil, err
		}
		// Stop the decompressor once enough entries are listed.
		defer decompressor.stop()
		stream = decompressor.output
	}

	archive := Archive{}
	reader := tar.NewReader(stream)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if len(archive.Entries) == 0 {
				return nil, fmt.Errorf("read tar: %w", err)
			}
			// A damaged or cut-off tail: keep what was listed.
			archive.Truncated = true
			break
		}
		if len(archive.Entries) >= s.maxArchiveEntries {
			archive.Truncated = true
			break
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
	}
	return &ArchivePreview{Archive: archive}, nil
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

func startDecompressor(ctx context.Context, tool, path string) (*decompressor, error) {
	command := exec.CommandContext(ctx, tool, "-dc", "--", path)
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
