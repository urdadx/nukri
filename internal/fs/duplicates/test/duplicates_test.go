package test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/urdadx/nukri/internal/fs/duplicates"
)

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func find(t *testing.T, root string, options duplicates.Options) []duplicates.Group {
	t.Helper()
	groups, err := duplicates.Find(context.Background(), root, options, nil)
	if err != nil {
		t.Fatal(err)
	}
	return groups
}

func TestFindsIdenticalFilesInNestedDirs(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.txt"), []byte("hello"))
	write(t, filepath.Join(root, "x", "y", "b.txt"), []byte("hello"))
	write(t, filepath.Join(root, "x", "c.txt"), []byte("other"))

	groups := find(t, root, duplicates.Options{})
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %+v", groups)
	}
	want := []string{filepath.Join(root, "a.txt"), filepath.Join(root, "x", "y", "b.txt")}
	if !slices.Equal(groups[0].Paths, want) || groups[0].Size != 5 {
		t.Fatalf("unexpected group %+v", groups[0])
	}
	if duplicates.Wasted(groups) != 5 {
		t.Fatalf("unexpected wasted %d", duplicates.Wasted(groups))
	}
}

func TestLargeIdenticalFilesGrouped(t *testing.T) {
	root := t.TempDir()
	data := bytes.Repeat([]byte("abcdefg"), 100_000)
	write(t, filepath.Join(root, "one"), data)
	write(t, filepath.Join(root, "two"), data)
	if groups := find(t, root, duplicates.Options{}); len(groups) != 1 {
		t.Fatalf("expected 1 group, got %+v", groups)
	}
}

func TestSameSizeDifferentContentNotGrouped(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a"), []byte("aaaa"))
	write(t, filepath.Join(root, "b"), []byte("bbbb"))

	big := bytes.Repeat([]byte{7}, 512*1024)
	changed := slices.Clone(big)
	changed[len(changed)/2] = 8
	write(t, filepath.Join(root, "big1"), big)
	write(t, filepath.Join(root, "big2"), changed)

	if groups := find(t, root, duplicates.Options{}); len(groups) != 0 {
		t.Fatalf("expected no groups, got %+v", groups)
	}
}

func TestEmptyFilesSkippedByDefault(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a"), nil)
	write(t, filepath.Join(root, "b"), nil)
	if groups := find(t, root, duplicates.Options{}); len(groups) != 0 {
		t.Fatalf("expected no groups, got %+v", groups)
	}
}

func TestHiddenSkippedUnlessIncluded(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a"), []byte("same"))
	write(t, filepath.Join(root, ".b"), []byte("same"))
	write(t, filepath.Join(root, ".dir", "c"), []byte("same"))

	if groups := find(t, root, duplicates.Options{}); len(groups) != 0 {
		t.Fatalf("expected no groups, got %+v", groups)
	}
	groups := find(t, root, duplicates.Options{IncludeHidden: true})
	if len(groups) != 1 || len(groups[0].Paths) != 3 {
		t.Fatalf("expected one group of 3, got %+v", groups)
	}
}

func TestSymlinksIgnored(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "a")
	write(t, target, []byte("data"))
	if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if groups := find(t, root, duplicates.Options{}); len(groups) != 0 {
		t.Fatalf("expected no groups, got %+v", groups)
	}
}

func TestHardlinksNotReported(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "a")
	write(t, target, []byte("data"))
	if err := os.Link(target, filepath.Join(root, "b")); err != nil {
		t.Skipf("hardlinks unsupported: %v", err)
	}
	if groups := find(t, root, duplicates.Options{}); len(groups) != 0 {
		t.Fatalf("expected no groups, got %+v", groups)
	}

	write(t, filepath.Join(root, "c"), []byte("data"))
	groups := find(t, root, duplicates.Options{})
	if len(groups) != 1 || len(groups[0].Paths) != 2 {
		t.Fatalf("expected one group of 2, got %+v", groups)
	}
}

func TestCancellation(t *testing.T) {
	root := t.TempDir()
	for i := range 10 {
		write(t, filepath.Join(root, string(rune('a'+i))), []byte("same"))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := duplicates.Find(ctx, root, duplicates.Options{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// Cancel mid-comparison from the progress callback.
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	_, err := duplicates.Find(ctx, root, duplicates.Options{}, func(p duplicates.Progress) {
		if p.Phase == duplicates.Comparing {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestProgressReachesDone(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a"), []byte("same"))
	write(t, filepath.Join(root, "b"), []byte("same"))
	var phases []duplicates.Phase
	var last duplicates.Progress
	_, err := duplicates.Find(context.Background(), root, duplicates.Options{}, func(p duplicates.Progress) {
		phases = append(phases, p.Phase)
		last = p
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(phases, duplicates.Comparing) || last.Phase != duplicates.Done {
		t.Fatalf("unexpected phases %v", phases)
	}
	if last.Scanned != 2 || last.Hashed != 2 || last.BytesHashed != last.BytesTotal {
		t.Fatalf("unexpected final progress %+v", last)
	}
}

func TestOrderedByWastedSpace(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "s1"), []byte("ab"))
	write(t, filepath.Join(root, "s2"), []byte("ab"))
	write(t, filepath.Join(root, "s3"), []byte("ab"))
	write(t, filepath.Join(root, "l1"), []byte("longer!"))
	write(t, filepath.Join(root, "l2"), []byte("longer!"))

	groups := find(t, root, duplicates.Options{})
	if len(groups) != 2 || groups[0].Size != 7 || groups[1].Size != 2 {
		t.Fatalf("unexpected order %+v", groups)
	}
	if duplicates.Wasted(groups) != 7+4 {
		t.Fatalf("unexpected wasted %d", duplicates.Wasted(groups))
	}
}

func TestMaxFilesReturnsError(t *testing.T) {
	root := t.TempDir()
	for i := range 5 {
		write(t, filepath.Join(root, string(rune('a'+i))), []byte("x"))
	}
	groups, err := duplicates.Find(context.Background(), root, duplicates.Options{MaxFiles: 3}, nil)
	if !errors.Is(err, duplicates.ErrTooManyFiles) || groups != nil {
		t.Fatalf("expected ErrTooManyFiles, got %v %+v", err, groups)
	}
}

func TestUnreadableRootFails(t *testing.T) {
	if _, err := duplicates.Find(context.Background(), filepath.Join(t.TempDir(), "missing"), duplicates.Options{}, nil); err == nil {
		t.Fatal("expected error for missing root")
	}
}

func TestUnreadableEntriesIgnored(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions not enforced for root")
	}
	root := t.TempDir()
	write(t, filepath.Join(root, "a"), []byte("same"))
	write(t, filepath.Join(root, "b"), []byte("same"))
	write(t, filepath.Join(root, "locked", "c"), []byte("same"))
	write(t, filepath.Join(root, "noread"), []byte("same"))
	if err := os.Chmod(filepath.Join(root, "locked"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "noread"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Join(root, "locked"), 0o755) })

	groups := find(t, root, duplicates.Options{})
	if len(groups) != 1 || len(groups[0].Paths) != 2 {
		t.Fatalf("expected one group of 2, got %+v", groups)
	}
}
