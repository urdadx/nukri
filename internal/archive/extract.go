package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extract expands source into a unique sibling directory and returns its path.
// Archive links and special entries are rejected.
func Extract(source string) (string, error) {
	return ExtractWithProgress(source, nil)
}

// ErrTooLarge means an archive expands past its extraction limits.
var ErrTooLarge = errors.New("archive is too large to extract")

// ExtractLimits bound what an extraction may write, guarding against archive
// bombs: small files that expand to fill the disk.
type ExtractLimits struct {
	MaxBytes   int64 // total size of extracted files; 0 means no limit
	MaxEntries int   // number of files and directories; 0 means no limit
}

const defaultMaxExtractEntries = 1_000_000

// defaultExtractLimits allows up to the free space at parent, less a reserve,
// so an extraction can never fill the disk.
func defaultExtractLimits(parent string) ExtractLimits {
	limits := ExtractLimits{MaxEntries: defaultMaxExtractEntries}
	if free, ok := availableBytes(parent); ok {
		limits.MaxBytes = max(1, free-free/20) // keep 5% free
	}
	return limits
}

// ExtractWithProgress expands source and reports bytes read from archive data.
func ExtractWithProgress(source string, report func(Progress)) (string, error) {
	return ExtractWithLimits(source, report, defaultExtractLimits(filepath.Dir(source)))
}

// ExtractWithLimits is ExtractWithProgress with explicit limits.
func ExtractWithLimits(source string, report func(Progress), limits ExtractLimits) (string, error) {
	format, err := FormatFromPath(source)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(source)
	destination := uniquePath(parent, extractionBase(source))
	stage, err := os.MkdirTemp(parent, ".nukri-extract-*")
	if err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(stage)
		}
	}()
	tracker := &progressTracker{report: report}
	target := &extraction{root: stage, limits: limits, buffer: make([]byte, 256<<10)}
	if format == ZIP {
		err = extractZIP(source, target, tracker)
	} else {
		info, statErr := os.Stat(source)
		if statErr != nil {
			return "", statErr
		}
		tracker.progress.Total = info.Size()
		tracker.next = 1 << 20
		if report != nil {
			report(tracker.progress)
		}
		err = extractTAR(source, target, format == TARGZ, tracker)
	}
	if err == nil {
		err = target.finish()
	}
	if err != nil {
		return "", err
	}
	// MkdirTemp creates the staging directory private (0700); the published
	// folder should look like any other.
	if err := os.Chmod(stage, 0o755); err != nil {
		return "", fmt.Errorf("publish extraction: %w", err)
	}
	if err := os.Rename(stage, destination); err != nil {
		return "", fmt.Errorf("publish extraction: %w", err)
	}
	ok = true
	tracker.finish()
	return destination, nil
}

func safeArchivePath(root, name string) (string, error) {
	if name == "" || strings.ContainsRune(name, '\x00') || filepath.IsAbs(name) {
		return "", fmt.Errorf("unsafe archive path: %q", name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive path: %q", name)
	}
	path := filepath.Join(root, clean)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive path: %q", name)
	}
	return path, nil
}

func extractZIP(source string, target *extraction, progress *progressTracker) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()
	if limit := target.limits.MaxEntries; limit > 0 && len(reader.File) > limit {
		return ErrTooLarge
	}
	for _, entry := range reader.File {
		progress.progress.Total += int64(entry.UncompressedSize64)
	}
	// Declared sizes can lie, so this only fails early; writes are still counted.
	if limit := target.limits.MaxBytes; limit > 0 && progress.progress.Total > limit {
		return ErrTooLarge
	}
	progress.next = 1 << 20
	if progress.report != nil {
		progress.report(progress.progress)
	}
	for _, entry := range reader.File {
		mode := entry.Mode()
		if mode&os.ModeSymlink != 0 || (!mode.IsRegular() && !mode.IsDir()) {
			return fmt.Errorf("unsupported archive entry: %s", entry.Name)
		}
		if mode.IsDir() {
			if err := target.directory(entry.Name, mode.Perm()); err != nil {
				return err
			}
			continue
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		err = target.file(entry.Name, mode.Perm(), &progressReader{reader: input, progress: progress})
		input.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTAR(source string, target *extraction, compressed bool, progress *progressTracker) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	var input io.Reader = &progressReader{reader: file, progress: progress}
	if compressed {
		gzipReader, err := gzip.NewReader(input)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		input = gzipReader
	}
	reader := tar.NewReader(input)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		mode := os.FileMode(header.Mode).Perm()
		// The reader reports old-style regular files (TypeRegA) as TypeReg.
		switch header.Typeflag {
		case tar.TypeDir:
			err = target.directory(header.Name, mode)
		case tar.TypeReg:
			err = target.file(header.Name, mode, reader)
		default:
			err = fmt.Errorf("unsupported archive entry: %s", header.Name)
		}
		if err != nil {
			return err
		}
	}
}

// extraction writes archive entries under root, enforcing its limits.
type extraction struct {
	root    string
	limits  ExtractLimits
	buffer  []byte
	entries int
	written int64
	// Directory modes are applied after extraction: a read-only directory
	// created up front would reject its own contents.
	directories []directoryMode
}

type directoryMode struct {
	path string
	mode os.FileMode
}

func (e *extraction) count() error {
	e.entries++
	if e.limits.MaxEntries > 0 && e.entries > e.limits.MaxEntries {
		return ErrTooLarge
	}
	return nil
}

func (e *extraction) directory(name string, mode os.FileMode) error {
	if err := e.count(); err != nil {
		return err
	}
	path, err := safeArchivePath(e.root, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	e.directories = append(e.directories, directoryMode{path: path, mode: mode})
	return nil
}

func (e *extraction) file(name string, mode os.FileMode, source io.Reader) error {
	if err := e.count(); err != nil {
		return err
	}
	path, err := safeArchivePath(e.root, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if e.limits.MaxBytes > 0 {
		// Read one byte past the remaining budget to detect overflow.
		source = io.LimitReader(source, e.limits.MaxBytes-e.written+1)
	}
	written, copyErr := io.CopyBuffer(file, source, e.buffer)
	closeErr := file.Close()
	e.written += written
	if e.limits.MaxBytes > 0 && e.written > e.limits.MaxBytes {
		return ErrTooLarge
	}
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// finish applies directory modes, deepest first so parents stay writable.
func (e *extraction) finish() error {
	for i := len(e.directories) - 1; i >= 0; i-- {
		if err := os.Chmod(e.directories[i].path, e.directories[i].mode); err != nil {
			return err
		}
	}
	return nil
}

type progressReader struct {
	reader   io.Reader
	progress *progressTracker
}

func (r *progressReader) Read(buffer []byte) (int, error) {
	read, err := r.reader.Read(buffer)
	r.progress.add(int64(read))
	return read, err
}
