package preview

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
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
		dir = cacheDir("code-preview", codeCacheVersion)
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
	if writeFileAtomic(c.path(key), []byte(content)) != nil {
		return
	}
	evictOldest(c.dir, ".hl", c.limit, codeCacheMaxBytes, nil)
}

func (c *codeCache) path(key string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return filepath.Join(c.dir, fmt.Sprintf("code-%016x.hl", h.Sum64()))
}
