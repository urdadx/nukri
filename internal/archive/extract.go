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

// Extract expands source into a unique sibling directory and returns its path.
// Archive links and special entries are rejected.
func Extract(source string) (string, error) {
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
	if format == ZIP {
		err = extractZIP(source, stage)
	} else {
		err = extractTAR(source, stage, format == TARGZ)
	}
	if err != nil {
		return "", err
	}
	if err := os.Rename(stage, destination); err != nil {
		return "", fmt.Errorf("publish extraction: %w", err)
	}
	ok = true
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

func extractZIP(source, root string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, entry := range reader.File {
		mode := entry.Mode()
		if mode&os.ModeSymlink != 0 || (!mode.IsRegular() && !mode.IsDir()) {
			return fmt.Errorf("unsupported archive entry: %s", entry.Name)
		}
		path, err := safeArchivePath(root, entry.Name)
		if err != nil {
			return err
		}
		if mode.IsDir() {
			if err := os.MkdirAll(path, mode.Perm()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		err = writeExtractedFile(path, mode.Perm(), input)
		input.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTAR(source, root string, compressed bool) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	var input io.Reader = file
	if compressed {
		gzipReader, err := gzip.NewReader(file)
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
		path, err := safeArchivePath(root, header.Name)
		if err != nil {
			return err
		}
		mode := os.FileMode(header.Mode).Perm()
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, mode); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := writeExtractedFile(path, mode, reader); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry: %s", header.Name)
		}
	}
}

func writeExtractedFile(path string, mode os.FileMode, source io.Reader) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, source)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
