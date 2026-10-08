package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Progress struct {
	Completed int64
	Total     int64
}

type progressTracker struct {
	progress Progress
	next     int64
	report   func(Progress)
}

// Create writes sources to destination. In-tree symlinks to regular files are
// dereferenced; other symlinks and special filesystem objects are rejected.
func Create(destination string, sources []string) (string, error) {
	return CreateWithProgress(destination, sources, nil)
}

// CreateWithProgress writes sources to destination and reports copied bytes.
func CreateWithProgress(destination string, sources []string, report func(Progress)) (string, error) {
	format, err := FormatFromPath(destination)
	if err != nil {
		return "", err
	}
	if len(sources) == 0 {
		return "", fmt.Errorf("no files selected")
	}
	if _, err := os.Lstat(destination); err == nil {
		return "", fmt.Errorf("destination already exists: %s", filepath.Base(destination))
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("check destination: %w", err)
	}
	tracker := &progressTracker{report: report}
	if report != nil {
		err := walkSources(sources, func(_ string, _ string, info os.FileInfo) error {
			if info.Mode().IsRegular() {
				tracker.progress.Total += info.Size()
			}
			return nil
		})
		if err != nil {
			return "", err
		}
		tracker.next = 1 << 20
		report(tracker.progress)
	}
	parent := filepath.Dir(destination)
	stage, err := os.CreateTemp(parent, ".nukri-archive-*")
	if err != nil {
		return "", fmt.Errorf("create staging file: %w", err)
	}
	stagePath := stage.Name()
	ok := false
	defer func() {
		stage.Close()
		if !ok {
			_ = os.Remove(stagePath)
		}
	}()

	switch format {
	case ZIP:
		err = writeZIP(stage, sources, tracker)
	case TAR, TARGZ:
		err = writeTAR(stage, sources, format == TARGZ, tracker)
	}
	if closeErr := stage.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(destination); err == nil {
		return "", fmt.Errorf("destination already exists: %s", filepath.Base(destination))
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("check destination: %w", err)
	}
	if err := os.Rename(stagePath, destination); err != nil {
		return "", fmt.Errorf("publish archive: %w", err)
	}
	ok = true
	tracker.finish()
	return destination, nil
}

func walkSources(sources []string, visit func(string, string, os.FileInfo) error) error {
	seen := make(map[string]bool)
	for _, source := range sources {
		source = filepath.Clean(source)
		root, err := filepath.Abs(source)
		if err != nil {
			return err
		}
		rootName := filepath.Base(source)
		err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			archivePath := path
			if info.Mode()&os.ModeSymlink != 0 {
				if path == source {
					return fmt.Errorf("refusing to archive source symlink: %s", path)
				}
				resolved, err := filepath.EvalSymlinks(path)
				if err != nil {
					return err
				}
				resolved, err = filepath.Abs(resolved)
				if err != nil {
					return err
				}
				relativeTarget, err := filepath.Rel(root, resolved)
				if err != nil || relativeTarget == ".." || strings.HasPrefix(relativeTarget, ".."+string(filepath.Separator)) {
					return fmt.Errorf("refusing to archive symlink outside source: %s", path)
				}
				info, err = os.Stat(resolved)
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("refusing to archive symlink to non-regular file: %s", path)
				}
				path = resolved
			}
			if !info.Mode().IsRegular() && !info.IsDir() {
				return fmt.Errorf("refusing to archive special file: %s", path)
			}
			relative, err := filepath.Rel(source, archivePath)
			if err != nil {
				return err
			}
			name := rootName
			if relative != "." {
				name = filepath.Join(rootName, relative)
			}
			name = filepath.ToSlash(name)
			if seen[name] {
				return fmt.Errorf("duplicate archive path: %s", name)
			}
			seen[name] = true
			return visit(path, name, info)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func writeZIP(output io.Writer, sources []string, progress *progressTracker) error {
	writer := zip.NewWriter(output)
	err := walkSources(sources, func(path, name string, info os.FileInfo) error {
		var file *os.File
		if !info.IsDir() {
			var err error
			file, info, err = openRegular(path, info)
			if err != nil {
				return err
			}
			defer file.Close()
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}
		entry, err := writer.CreateHeader(header)
		if err != nil || info.IsDir() {
			return err
		}
		err = copyWithProgress(entry, file, progress)
		return err
	})
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	return err
}

func writeTAR(output io.Writer, sources []string, compressed bool, progress *progressTracker) error {
	var gzipWriter *gzip.Writer
	if compressed {
		gzipWriter = gzip.NewWriter(output)
		output = gzipWriter
	}
	writer := tar.NewWriter(output)
	err := walkSources(sources, func(path, name string, info os.FileInfo) error {
		var file *os.File
		if !info.IsDir() {
			var err error
			file, info, err = openRegular(path, info)
			if err != nil {
				return err
			}
			defer file.Close()
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = strings.TrimSuffix(name, "/")
		if err := writer.WriteHeader(header); err != nil || info.IsDir() {
			return err
		}
		err = copyWithProgress(writer, file, progress)
		return err
	})
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if gzipWriter != nil {
		if closeErr := gzipWriter.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

func copyWithProgress(destination io.Writer, source io.Reader, progress *progressTracker) error {
	buffer := make([]byte, 256*1024)
	for {
		read, readErr := source.Read(buffer)
		if read > 0 {
			written, writeErr := destination.Write(buffer[:read])
			progress.add(int64(written))
			if writeErr != nil {
				return writeErr
			}
			if written != read {
				return io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func (p *progressTracker) add(bytes int64) {
	if p == nil || p.report == nil {
		return
	}
	p.progress.Completed += bytes
	if p.progress.Completed >= p.next {
		p.next = p.progress.Completed + 1<<20
		p.report(p.progress)
	}
}

func (p *progressTracker) finish() {
	if p == nil || p.report == nil {
		return
	}
	p.progress.Completed = p.progress.Total
	p.report(p.progress)
}

func openRegular(path string, walkedInfo os.FileInfo) (*os.File, os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || !pathInfo.Mode().IsRegular() ||
		!os.SameFile(walkedInfo, info) || !os.SameFile(info, pathInfo) {
		file.Close()
		return nil, nil, fmt.Errorf("source changed while archiving: %s", path)
	}
	return file, info, nil
}
