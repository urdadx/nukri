package preview

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// imageCache persists downscaled, PNG-encoded image previews on disk so that
// revisiting a file (or re-rendering at the same display size) does not repeat
// the expensive decode + downscale + encode pass. It mirrors how terminal file
// managers render images to a cached size rather than re-decoding the source.
type imageCache struct {
	dir   string
	limit int
}

const (
	imageCacheDefaultLimit = 256
	imageCacheVersion      = "v1"
)

func newImageCache(dir string) *imageCache {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "nukri-image-preview", imageCacheVersion)
	}
	return &imageCache{dir: dir, limit: imageCacheDefaultLimit}
}

// Key identifies a cached rendering uniquely by the source file identity (path,
// size, modification time) and the target longest-side pixel dimension. Two
// requests share a cache entry only when all of these match.
func (c *imageCache) Key(path string, size int64, modified time.Time, longestSide int) string {
	mod := int64(0)
	if !modified.IsZero() {
		mod = modified.UnixNano()
	}
	h := fnv.New64a()
	_, _ = io.WriteString(h, fmt.Sprintf("%s|%d|%d|%d", path, size, mod, longestSide))
	return fmt.Sprintf("%016x", h.Sum64())
}

// CachedImage is a cached downscaled PNG plus its dimensions, source info, and
// optional opaque metadata (e.g. PDF metadata serialized alongside the page).
type CachedImage struct {
	Data     []byte
	Width    int
	Height   int
	Format   string
	SrcW     int
	SrcH     int
	Size     int64
	Metadata []byte
}

// Get returns the cached PNG for the given key, if present and valid. A corrupt
// cache file is removed and treated as a miss.
func (c *imageCache) Get(key string) (CachedImage, bool) {
	path := c.cachePath(key, 0, 0)
	data, err := os.ReadFile(path)
	if err != nil {
		return CachedImage{}, false
	}
	meta, ok := c.readMeta(key)
	if !ok {
		_ = os.Remove(path)
		return CachedImage{}, false
	}
	return CachedImage{Data: data, Width: meta.Width, Height: meta.Height, Format: meta.Format, SrcW: meta.SrcW, SrcH: meta.SrcH, Size: meta.Size, Metadata: meta.Metadata}, true
}

// Put stores a downscaled PNG plus its dimensions under the given key. Writes
// are atomic (temp file + rename) and, when the cache exceeds its size limit,
// the least-recently-used entries are evicted.
func (c *imageCache) Put(key string, data []byte, width, height int, format string, srcW, srcH int, size int64, metadata []byte) {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return
	}
	_ = writeJSON(c.metaPath(key), cachedMeta{Width: width, Height: height, Format: format, SrcW: srcW, SrcH: srcH, Size: size, Metadata: metadata})
	tmp := c.cachePath(key, 0, 0) + fmt.Sprintf(".tmp-%d", os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		_ = os.Remove(tmp)
		return
	}
	_ = os.Rename(tmp, c.cachePath(key, 0, 0))
	c.evictIfNeeded()
}

type cachedMeta struct {
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Format   string `json:"format"`
	SrcW     int    `json:"src_w"`
	SrcH     int    `json:"src_h"`
	Size     int64  `json:"size"`
	Metadata []byte `json:"metadata,omitempty"`
}

func (c *imageCache) readMeta(key string) (cachedMeta, bool) {
	data, err := os.ReadFile(c.metaPath(key))
	if err != nil {
		return cachedMeta{}, false
	}
	var meta cachedMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return cachedMeta{}, false
	}
	if meta.Width <= 0 || meta.Height <= 0 {
		return cachedMeta{}, false
	}
	return meta, true
}

func (c *imageCache) cachePath(key string, _ int, _ int) string {
	return filepath.Join(c.dir, "img-"+key+".png")
}

func (c *imageCache) metaPath(key string) string {
	return filepath.Join(c.dir, "img-"+key+".json")
}

func writeJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// evictIfNeeded removes the oldest entries when the number of cached images
// exceeds the configured limit. Only PNG files matching our naming scheme are
// counted, so unrelated files in the directory are left alone.
func (c *imageCache) evictIfNeeded() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	var pngs []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".png" {
			pngs = append(pngs, entry.Name())
		}
	}
	if len(pngs) <= c.limit {
		return
	}
	excess := len(pngs) - c.limit
	sort.Slice(pngs, func(i, j int) bool {
		ti, _ := fileInfoTime(c.dir, pngs[i])
		tj, _ := fileInfoTime(c.dir, pngs[j])
		return ti.Before(tj)
	})
	for _, name := range pngs[:excess] {
		key := trimCacheKey(name)
		_ = os.Remove(filepath.Join(c.dir, name))
		if key != "" {
			_ = os.Remove(c.metaPath(key))
		}
	}
}

func fileInfoTime(dir, name string) (time.Time, bool) {
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

func trimCacheKey(name string) string {
	base := filepath.Base(name)
	base = base[:len(base)-len(filepath.Ext(base))]
	const prefix = "img-"
	if len(base) > len(prefix) && base[:len(prefix)] == prefix {
		return base[len(prefix):]
	}
	return ""
}
