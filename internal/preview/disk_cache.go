package preview

import (
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/adrg/xdg"
)

// Disk cache budgets. Both count and size are bounded: highlighted code can be
// several times larger than its source, and /tmp-style locations may be RAM.
const (
	codeCacheMaxBytes  = 256 << 20
	imageCacheMaxBytes = 256 << 20
)

// cacheDir is where a preview cache lives: the user's cache directory
// ($XDG_CACHE_HOME, usually ~/.cache), not /tmp, which is often RAM-backed.
func cacheDir(name, version string) string {
	return filepath.Join(xdg.CacheHome, "nukri", name, version)
}

// writeFileAtomic writes data to path through a uniquely named temporary file,
// so readers never see a partial file and concurrent writers don't collide.
func writeFileAtomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr == nil {
		writeErr = os.Rename(file.Name(), path)
	}
	if writeErr != nil {
		_ = os.Remove(file.Name())
	}
	return writeErr
}

/*
evictOldest deletes the oldest files with extension ext in dir until at most
maxFiles remain and they total at most maxBytes. Each file is stat'ed once.
removed, when set, is called with each deleted file's name.
*/
func evictOldest(dir, ext string, maxFiles int, maxBytes int64, removed func(name string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type cached struct {
		name     string
		size     int64
		modified time.Time
	}
	var files []cached
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ext {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, cached{name: entry.Name(), size: info.Size(), modified: info.ModTime()})
		total += info.Size()
	}
	if len(files) <= maxFiles && total <= maxBytes {
		return
	}
	slices.SortFunc(files, func(a, b cached) int { return a.modified.Compare(b.modified) })
	for _, file := range files {
		if len(files) <= maxFiles && total <= maxBytes {
			return
		}
		if os.Remove(filepath.Join(dir, file.name)) == nil && removed != nil {
			removed(file.name)
		}
		files = files[1:]
		total -= file.size
	}
}
