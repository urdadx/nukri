// Package duplicates finds files with identical content under a directory.
package duplicates

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	sampleSize       = 64 * 1024
	bufferSize       = 256 * 1024
	progressInterval = 100 * time.Millisecond
	defaultMaxFiles  = 1_000_000
	maxWorkers       = 4
)

// ErrTooManyFiles is returned when the scan sees more than Options.MaxFiles regular files.
var ErrTooManyFiles = errors.New("too many files to scan for duplicates")

// Phase is the current stage of a scan.
type Phase uint8

const (
	Collecting Phase = iota
	Comparing
	Done
)

// Group is a set of files with identical content. Paths are sorted shortest first, then lexically.
type Group struct {
	Size  int64
	Paths []string
}

// Progress reports scan state. Hashed counts files hashed across all comparison
// stages; BytesTotal grows as each stage starts.
type Progress struct {
	Phase       Phase
	Scanned     int
	Candidates  int
	Hashed      int
	BytesHashed int64
	BytesTotal  int64
}

// Options controls a scan. Zero values select defaults.
type Options struct {
	IncludeHidden bool
	// MinSize skips smaller files. Default 1 (skip empty files).
	MinSize int64
	// MaxFiles caps the number of regular files collected. Default 1_000_000.
	// Exceeding it aborts the scan with ErrTooManyFiles (no partial results).
	MaxFiles int
}

type entry struct {
	path string
	size int64
}

type fileID struct {
	dev uint64
	ino uint64
}

type digest [sha256.Size]byte

// Find scans root and returns groups of duplicate files, ordered by wasted space
// descending. progress, if non-nil, is called only from the calling goroutine.
func Find(ctx context.Context, root string, options Options, progress func(Progress)) ([]Group, error) {
	if options.MinSize <= 0 {
		options.MinSize = 1
	}
	if options.MaxFiles <= 0 {
		options.MaxFiles = defaultMaxFiles
	}
	r := &reporter{fn: progress}

	sets, err := collect(ctx, root, options, r)
	if err != nil {
		return nil, err
	}

	r.state.Phase = Comparing
	r.state.Candidates = countEntries(sets)
	r.emit(true)

	var sampleBytes int64
	for _, set := range sets {
		for _, e := range set {
			sampleBytes += sampleLength(e.size)
		}
	}
	sets, err = refine(ctx, sets, sampleBytes, hashSample, r)
	if err != nil {
		return nil, err
	}

	// Files no larger than one sample were fully read by the sample stage.
	var full [][]entry
	var fullBytes int64
	var groups []Group
	for _, set := range sets {
		if set[0].size <= sampleSize {
			groups = append(groups, newGroup(set))
			continue
		}
		full = append(full, set)
		fullBytes += set[0].size * int64(len(set))
	}
	r.state.Candidates = countEntries(sets)
	full, err = refine(ctx, full, fullBytes, hashFull, r)
	if err != nil {
		return nil, err
	}
	for _, set := range full {
		groups = append(groups, newGroup(set))
	}

	slices.SortFunc(groups, func(a, b Group) int {
		if c := cmp.Compare(wasted(b), wasted(a)); c != 0 {
			return c
		}
		return cmp.Compare(a.Paths[0], b.Paths[0])
	})

	r.state.Phase = Done
	r.state.Candidates = countGroups(groups)
	r.emit(true)
	return groups, nil
}

// Wasted returns the total bytes reclaimable by keeping one file per group.
func Wasted(groups []Group) int64 {
	var total int64
	for _, g := range groups {
		total += wasted(g)
	}
	return total
}

func wasted(g Group) int64 {
	if len(g.Paths) < 2 {
		return 0
	}
	return g.Size * int64(len(g.Paths)-1)
}

func newGroup(set []entry) Group {
	paths := make([]string, len(set))
	for i, e := range set {
		paths[i] = e.path
	}
	slices.SortFunc(paths, func(a, b string) int {
		if c := cmp.Compare(len(a), len(b)); c != 0 {
			return c
		}
		return cmp.Compare(a, b)
	})
	return Group{Size: set[0].size, Paths: paths}
}

func countEntries(sets [][]entry) int {
	n := 0
	for _, set := range sets {
		n += len(set)
	}
	return n
}

func countGroups(groups []Group) int {
	n := 0
	for _, g := range groups {
		n += len(g.Paths)
	}
	return n
}

// collect walks root and returns same-size sets with at least two files.
func collect(ctx context.Context, root string, options Options, r *reporter) ([][]entry, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("scan %s: %w", root, err)
	}
	r.emit(true)

	seen := make(map[fileID]struct{})
	bySize := make(map[int64][]entry)
	files := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			if path == root {
				return fmt.Errorf("scan %s: %w", root, walkErr)
			}
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path != root && !options.IncludeHidden && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files++
		if files > options.MaxFiles {
			return fmt.Errorf("scan %s: %w (limit %d)", root, ErrTooManyFiles, options.MaxFiles)
		}
		r.state.Scanned = files
		r.emit(false)
		if info.Size() < options.MinSize {
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			id := fileID{dev: uint64(st.Dev), ino: uint64(st.Ino)}
			if _, dup := seen[id]; dup {
				return nil
			}
			seen[id] = struct{}{}
		}
		bySize[info.Size()] = append(bySize[info.Size()], entry{path: path, size: info.Size()})
		return nil
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}

	var sets [][]entry
	for _, set := range bySize {
		if len(set) > 1 {
			sets = append(sets, set)
		}
	}
	return sets, nil
}

type hashFunc func(ctx context.Context, e entry, buf []byte, read *atomic.Int64) (digest, error)

type job struct {
	set int
	e   entry
}

type result struct {
	job
	sum digest
	err error
}

// refine splits each set by the hash produced by fn and drops unique files.
// Files that fail to hash are dropped.
func refine(ctx context.Context, sets [][]entry, stageBytes int64, fn hashFunc, r *reporter) ([][]entry, error) {
	if len(sets) == 0 {
		return nil, ctx.Err()
	}
	r.state.BytesTotal += stageBytes
	baseBytes := r.state.BytesHashed
	var read atomic.Int64

	workers := min(runtime.NumCPU(), maxWorkers)
	jobs := make(chan job)
	results := make(chan result)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, bufferSize)
			for j := range jobs {
				sum, err := fn(ctx, j.e, buf, &read)
				select {
				case results <- result{job: j, sum: sum, err: err}:
				case <-ctx.Done():
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i, set := range sets {
			for _, e := range set {
				select {
				case jobs <- job{set: i, e: e}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	split := make([]map[digest][]entry, len(sets))
	ticker := time.NewTicker(progressInterval)
	defer ticker.Stop()
	pending := countEntries(sets)
	for pending > 0 {
		select {
		case <-ctx.Done():
			for range results {
			}
			return nil, ctx.Err()
		case <-ticker.C:
			r.state.BytesHashed = baseBytes + read.Load()
			r.emit(false)
		case res, ok := <-results:
			if !ok {
				return nil, ctx.Err()
			}
			pending--
			r.state.Hashed++
			r.state.BytesHashed = baseBytes + read.Load()
			r.emit(false)
			if res.err != nil {
				continue
			}
			if split[res.set] == nil {
				split[res.set] = make(map[digest][]entry)
			}
			split[res.set][res.sum] = append(split[res.set][res.sum], res.e)
		}
	}
	for range results {
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var out [][]entry
	for _, m := range split {
		for _, set := range m {
			if len(set) > 1 {
				out = append(out, set)
			}
		}
	}
	return out, nil
}

func sampleLength(size int64) int64 {
	if size > 2*sampleSize {
		return 2 * sampleSize
	}
	return min(size, sampleSize)
}

// hashSample hashes the first 64 KiB, plus the last 64 KiB for files over 128 KiB.
func hashSample(ctx context.Context, e entry, buf []byte, read *atomic.Int64) (digest, error) {
	f, err := os.Open(e.path)
	if err != nil {
		return digest{}, fmt.Errorf("open %s: %w", e.path, err)
	}
	defer f.Close()
	h := sha256.New()
	if err := copyN(ctx, h, io.NewSectionReader(f, 0, min(e.size, sampleSize)), buf, read); err != nil {
		return digest{}, fmt.Errorf("hash %s: %w", e.path, err)
	}
	if e.size > 2*sampleSize {
		if err := copyN(ctx, h, io.NewSectionReader(f, e.size-sampleSize, sampleSize), buf, read); err != nil {
			return digest{}, fmt.Errorf("hash %s: %w", e.path, err)
		}
	}
	var sum digest
	h.Sum(sum[:0])
	return sum, nil
}

// hashFull hashes the entire file content.
func hashFull(ctx context.Context, e entry, buf []byte, read *atomic.Int64) (digest, error) {
	f, err := os.Open(e.path)
	if err != nil {
		return digest{}, fmt.Errorf("open %s: %w", e.path, err)
	}
	defer f.Close()
	h := sha256.New()
	if err := copyN(ctx, h, f, buf, read); err != nil {
		return digest{}, fmt.Errorf("hash %s: %w", e.path, err)
	}
	var sum digest
	h.Sum(sum[:0])
	return sum, nil
}

// copyN streams src into dst, checking ctx between reads.
func copyN(ctx context.Context, dst io.Writer, src io.Reader, buf []byte, read *atomic.Int64) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := src.Read(buf)
		if n > 0 {
			dst.Write(buf[:n])
			read.Add(int64(n))
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// reporter throttles progress callbacks. It is only used from Find's goroutine.
type reporter struct {
	fn    func(Progress)
	state Progress
	last  time.Time
}

func (r *reporter) emit(force bool) {
	if r.fn == nil {
		return
	}
	now := time.Now()
	if !force && now.Sub(r.last) < progressInterval {
		return
	}
	r.last = now
	r.fn(r.state)
}
