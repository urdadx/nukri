package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Format uint8

const (
	ZIP Format = iota + 1
	TAR
	TARGZ
)

func FormatFromPath(path string) (Format, error) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return TARGZ, nil
	case strings.HasSuffix(lower, ".tar"):
		return TAR, nil
	case strings.HasSuffix(lower, ".zip"):
		return ZIP, nil
	default:
		return 0, fmt.Errorf("unsupported archive format: %s", filepath.Base(path))
	}
}

func extractionBase(path string) string {
	name := filepath.Base(path)
	lower := strings.ToLower(name)
	for _, suffix := range []string{".tar.gz", ".tgz", ".tar", ".zip"} {
		if strings.HasSuffix(lower, suffix) {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
}

func uniquePath(parent, name string) string {
	if name == "" {
		name = "archive"
	}
	path := filepath.Join(parent, name)
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return path
	}
	for n := 1; ; n++ {
		path = filepath.Join(parent, fmt.Sprintf("%s (%d)", name, n))
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			return path
		}
	}
}
