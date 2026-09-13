package fileops

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type Operation uint8

const (
	Copy Operation = iota + 1
	Move
)

type Result struct {
	Destinations []string
	Errors       []error
}

func ApplyDrop(destination string, sources []string, operation Operation) Result {
	result := Result{}
	info, err := os.Stat(destination)
	if err != nil || !info.IsDir() {
		result.Errors = append(result.Errors, fmt.Errorf("drop destination is not a directory"))
		return result
	}
	for _, source := range sources {
		info, err := os.Lstat(source)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", filepath.Base(source), err))
			continue
		}
		baseDestination := filepath.Join(destination, filepath.Base(source))
		if operation == Move && samePath(source, baseDestination) {
			result.Errors = append(result.Errors, fmt.Errorf("%s is already here", filepath.Base(source)))
			continue
		}
		if info.IsDir() && pathInside(source, destination) {
			result.Errors = append(result.Errors, fmt.Errorf("cannot drop %s into itself", filepath.Base(source)))
			continue
		}
		target := availablePath(baseDestination)
		if operation == Move {
			err = movePath(source, target)
		} else {
			err = copyPath(source, target)
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", filepath.Base(source), err))
			continue
		}
		result.Destinations = append(result.Destinations, target)
	}
	return result
}

func availablePath(path string) string {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	extension := filepath.Ext(path)
	stem := strings.TrimSuffix(filepath.Base(path), extension)
	directory := filepath.Dir(path)
	for index := 1; ; index++ {
		candidate := filepath.Join(directory, fmt.Sprintf("%s_%d%s", stem, index, extension))
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}

func copyPath(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(target, destination)
	}
	if info.IsDir() {
		if err := os.Mkdir(destination, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			_ = os.RemoveAll(destination)
			return err
		}
		for _, entry := range entries {
			if err := copyPath(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
				_ = os.RemoveAll(destination)
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported file type")
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(destination)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	return nil
}

func movePath(source, destination string) error {
	if err := os.Rename(source, destination); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if err := copyPath(source, destination); err != nil {
		_ = os.RemoveAll(destination)
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.RemoveAll(source)
	}
	return os.Remove(source)
}

func samePath(first, second string) bool {
	first, _ = filepath.Abs(first)
	second, _ = filepath.Abs(second)
	return filepath.Clean(first) == filepath.Clean(second)
}

func pathInside(source, destination string) bool {
	if resolved, err := filepath.EvalSymlinks(source); err == nil {
		source = resolved
	} else {
		source = filepath.Clean(source)
	}
	if resolved, err := filepath.EvalSymlinks(destination); err == nil {
		destination = resolved
	} else {
		destination = filepath.Clean(destination)
	}
	relative, err := filepath.Rel(source, destination)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
