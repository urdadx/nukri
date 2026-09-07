package preview

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"time"
)

/*
codeCache persists fully-highlighted source on disk so revisiting a file (or
re-rendering at a different pane size or scroll window) does not re-run the
syntax highlighter. It is keyed by a hash of the source text plus the language
and style, so any two identical highlight requests share a single entry
regardless of where the file lives.
*/

type codeCache struct {
	dir   string
	limit int
}

const (
	codeCacheDefaultLimit = 64
	codeCacheVersion      = "v1"
)

/*
codeCacheService is the shared, process-wide code highlight cache. Code
highlighting is a pure function of (source, language, style), so one shared
disk cache serves every preview service instance.
*/
var codeCacheService = newCodeCache("")

func newCodeCache(dir string) *codeCache {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "nukri-code-preview", codeCacheVersion)
	}
	return &codeCache{dir: dir, limit: codeCacheDefaultLimit}
}

/*
key returns the disk identity for a highlight request. The source is
content-addressed so the same rendered file is reused across pane sizes and
sessions.
*/
func (c *codeCache) key(source, language, style string) string {
	sum := sha256.Sum256([]byte(source))
	return fmt.Sprintf("%s|%s|%s|%s", hex.EncodeToString(sum[:]), language, style, codeCacheVersion)
}

func (c *codeCache) get(key string) (string, bool) {
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		return "", false
	}
	return string(data), true
}

func (c *codeCache) put(key, content string) {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return
	}
	tmp := c.path(key) + fmt.Sprintf(".tmp-%d", os.Getpid())
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		_ = os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, c.path(key)); err != nil {
		_ = os.Remove(tmp)
		return
	}
	c.evictIfNeeded()
}

func (c *codeCache) path(key string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return filepath.Join(c.dir, fmt.Sprintf("code-%016x.hl", h.Sum64()))
}

/*
evictIfNeeded removes the oldest entries from the cache until the total
number of entries is below the limit. It is called after a new entry is added.
*/
func (c *codeCache) evictIfNeeded() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".hl" {
			files = append(files, entry.Name())
		}
	}
	if len(files) <= c.limit {
		return
	}
	excess := len(files) - c.limit
	sort.Slice(files, func(i, j int) bool {
		ti, _ := fileTime(c.dir, files[i])
		tj, _ := fileTime(c.dir, files[j])
		return ti.Before(tj)
	})
	for _, name := range files[:excess] {
		_ = os.Remove(filepath.Join(c.dir, name))
	}
}

func fileTime(dir, name string) (time.Time, bool) {
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}
