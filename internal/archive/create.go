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

// Create writes sources to destination. Symlinks and filesystem objects other
// than regular files and directories are rejected rather than followed.
func Create(destination string, sources []string) (string, error) {
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
		err = writeZIP(stage, sources)
	case TAR, TARGZ:
		err = writeTAR(stage, sources, format == TARGZ)
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
	return destination, nil
}

func walkSources(sources []string, visit func(string, string, os.FileInfo) error) error {
	seen := make(map[string]bool)
	for _, source := range sources {
		source = filepath.Clean(source)
		rootName := filepath.Base(source)
		err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing to archive symlink: %s", path)
			}
			if !info.Mode().IsRegular() && !info.IsDir() {
				return fmt.Errorf("refusing to archive special file: %s", path)
			}
			relative, err := filepath.Rel(source, path)
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

func writeZIP(output io.Writer, sources []string) error {
	writer := zip.NewWriter(output)
	err := walkSources(sources, func(path, name string, info os.FileInfo) error {
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
		return copyRegular(path, entry)
	})
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	return err
}

func writeTAR(output io.Writer, sources []string, compressed bool) error {
	var gzipWriter *gzip.Writer
	if compressed {
		gzipWriter = gzip.NewWriter(output)
		output = gzipWriter
	}
	writer := tar.NewWriter(output)
	err := walkSources(sources, func(path, name string, info os.FileInfo) error {
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = strings.TrimSuffix(name, "/")
		if err := writer.WriteHeader(header); err != nil || info.IsDir() {
			return err
		}
		return copyRegular(path, writer)
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

func copyRegular(path string, destination io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(destination, file)
	return err
}
