// Package trash implements the FreeDesktop.org Trash specification.
package trash

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	infoSuffix = ".trashinfo"
	dateLayout = "2006-01-02T15:04:05"
)

// ErrRestoreConflict is returned when the original location is occupied.
var ErrRestoreConflict = errors.New("restore target already exists")

// Item describes a trashed entry and its metadata.
type Item struct {
	TrashedPath  string
	InfoPath     string
	OriginalPath string
	DeletedAt    time.Time
}

// Trash moves path (not following a final symlink) into the appropriate trash.
func Trash(path string) (Item, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Item{}, fmt.Errorf("resolve %s: %w", path, err)
	}
	abs = filepath.Clean(abs)
	if _, _, ok := trashOf(abs); ok {
		return Item{}, fmt.Errorf("%s is already in the trash", filepath.Base(abs))
	}
	itemDev, err := lstatDev(abs)
	if err != nil {
		return Item{}, fmt.Errorf("trash %s: %w", filepath.Base(abs), err)
	}

	home := homeTrashDir()
	if isWithin(home, abs) || isWithin(abs, home) {
		return Item{}, fmt.Errorf("cannot trash the trash itself")
	}
	if err := ensureTrashDirs(home); err != nil {
		return Item{}, fmt.Errorf("create home trash: %w", err)
	}
	homeDev, err := statDev(home)
	if err != nil {
		return Item{}, fmt.Errorf("stat home trash: %w", err)
	}

	root := home
	recorded := abs
	if itemDev != homeDev {
		parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
		if err != nil {
			return Item{}, fmt.Errorf("resolve parent of %s: %w", filepath.Base(abs), err)
		}
		real := filepath.Join(parent, filepath.Base(abs))
		topdir, err := TopDir(parent, statDev)
		if err != nil {
			return Item{}, fmt.Errorf("find mount point for %s: %w", filepath.Base(abs), err)
		}
		root, err = TopDirTrash(topdir, os.Getuid())
		if err != nil {
			return Item{}, err
		}
		recorded, err = filepath.Rel(topdir, real)
		if err != nil {
			return Item{}, fmt.Errorf("relative path for %s: %w", filepath.Base(abs), err)
		}
		abs = real
	}

	now := time.Now()
	name, infoPath, err := reserve(root, filepath.Base(abs), recorded, now)
	if err != nil {
		return Item{}, err
	}
	trashed := filepath.Join(root, "files", name)
	if err := os.Rename(abs, trashed); err != nil {
		_ = os.Remove(infoPath)
		return Item{}, fmt.Errorf("move %s to trash: %w", filepath.Base(abs), err)
	}
	original := recorded
	if !filepath.IsAbs(original) {
		original = abs
	}
	return Item{
		TrashedPath:  trashed,
		InfoPath:     infoPath,
		OriginalPath: original,
		DeletedAt:    now.Truncate(time.Second),
	}, nil
}

// Restore moves a trashed entry back to its original location and returns it.
func Restore(trashedFile string) (string, error) {
	item, err := Info(trashedFile)
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(item.OriginalPath); err == nil {
		return "", fmt.Errorf("restore %s: %w", item.OriginalPath, ErrRestoreConflict)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("restore %s: %w", item.OriginalPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(item.OriginalPath), 0o755); err != nil {
		return "", fmt.Errorf("recreate parent of %s: %w", item.OriginalPath, err)
	}
	if err := os.Rename(item.TrashedPath, item.OriginalPath); err != nil {
		return "", fmt.Errorf("restore %s: %w", item.OriginalPath, err)
	}
	if err := os.Remove(item.InfoPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return item.OriginalPath, fmt.Errorf("remove trash info: %w", err)
	}
	return item.OriginalPath, nil
}

// Info reads the metadata of an entry directly inside a trash files directory.
func Info(trashedFile string) (Item, error) {
	abs, err := filepath.Abs(trashedFile)
	if err != nil {
		return Item{}, fmt.Errorf("resolve %s: %w", trashedFile, err)
	}
	abs = filepath.Clean(abs)
	root, filesDir, ok := trashOf(abs)
	if !ok || filepath.Dir(abs) != filesDir {
		return Item{}, fmt.Errorf("%s is not a trashed item", abs)
	}
	name := filepath.Base(abs)
	infoPath := filepath.Join(root, "info", name+infoSuffix)
	rawPath, deleted, err := readInfo(infoPath)
	if err != nil {
		return Item{}, err
	}
	original := rawPath
	if !filepath.IsAbs(original) {
		topdir, ok := trashTopDir(root)
		if !ok {
			return Item{}, fmt.Errorf("relative path in home trash info %s", infoPath)
		}
		original = filepath.Join(topdir, original)
	}
	return Item{
		TrashedPath:  abs,
		InfoPath:     infoPath,
		OriginalPath: filepath.Clean(original),
		DeletedAt:    deleted,
	}, nil
}

// Delete permanently removes path, and its trash info when it is a trashed item.
func Delete(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", path, err)
	}
	abs = filepath.Clean(abs)
	if err := os.RemoveAll(abs); err != nil {
		return fmt.Errorf("delete %s: %w", filepath.Base(abs), err)
	}
	root, filesDir, ok := trashOf(abs)
	if ok && filepath.Dir(abs) == filesDir {
		infoPath := filepath.Join(root, "info", filepath.Base(abs)+infoSuffix)
		if err := os.Remove(infoPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove trash info: %w", err)
		}
	}
	return nil
}

// IsInTrash reports whether path is a trash files directory or inside one.
func IsInTrash(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	_, _, ok := trashOf(filepath.Clean(abs))
	return ok
}

// TrashFilesDir returns the files directory of the home trash.
func TrashFilesDir() string {
	return filepath.Join(homeTrashDir(), "files")
}

// TopDir walks up from dir while the device id stays the same and returns the
// last directory on that device (the mount point).
func TopDir(dir string, deviceOf func(string) (uint64, error)) (string, error) {
	cur := filepath.Clean(dir)
	dev, err := deviceOf(cur)
	if err != nil {
		return "", err
	}
	for {
		parent := filepath.Dir(cur)
		if parent == cur {
			return cur, nil
		}
		pdev, err := deviceOf(parent)
		if err != nil || pdev != dev {
			return cur, nil
		}
		cur = parent
	}
}

// TopDirTrash returns (creating as needed) the trash root for topdir:
// $topdir/.Trash/$uid when $topdir/.Trash is a sticky, non-symlink directory,
// otherwise $topdir/.Trash-$uid.
func TopDirTrash(topdir string, uid int) (string, error) {
	shared := filepath.Join(topdir, ".Trash")
	if fi, err := os.Lstat(shared); err == nil && fi.IsDir() && fi.Mode()&os.ModeSticky != 0 {
		root := filepath.Join(shared, strconv.Itoa(uid))
		if err := ensureTrashDirs(root); err == nil {
			return root, nil
		}
	}
	root := filepath.Join(topdir, ".Trash-"+strconv.Itoa(uid))
	if err := ensureTrashDirs(root); err != nil {
		return "", fmt.Errorf("create trash in %s: %w", topdir, err)
	}
	return root, nil
}

// EncodePath percent-encodes each segment of p, keeping '/' separators.
func EncodePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func homeTrashDir() string {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" || !filepath.IsAbs(data) {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/"
		}
		data = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(filepath.Clean(data), "Trash")
}

func ensureTrashDirs(root string) error {
	for _, sub := range []string{"files", "info"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o700); err != nil {
			return err
		}
	}
	return nil
}

// reserve atomically claims a unique name in root/info and writes its metadata.
func reserve(root, base, recorded string, when time.Time) (string, string, error) {
	content := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		EncodePath(recorded), when.Format(dateLayout))
	for n := 1; ; n++ {
		name := candidateName(base, n)
		infoPath := filepath.Join(root, "info", name+infoSuffix)
		f, err := os.OpenFile(infoPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", "", fmt.Errorf("write trash info: %w", err)
		}
		if _, err := os.Lstat(filepath.Join(root, "files", name)); err == nil {
			// Orphaned entry without info; keep it and try the next name.
			f.Close()
			_ = os.Remove(infoPath)
			continue
		}
		_, werr := f.WriteString(content)
		cerr := f.Close()
		if werr != nil || cerr != nil {
			_ = os.Remove(infoPath)
			return "", "", fmt.Errorf("write trash info: %w", errors.Join(werr, cerr))
		}
		return name, infoPath, nil
	}
}

// candidateName returns base for n == 1 and "stem.n.ext" otherwise.
func candidateName(base string, n int) string {
	if n == 1 {
		return base
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem, ext = base, ""
	}
	return stem + "." + strconv.Itoa(n) + ext
}

func readInfo(infoPath string) (string, time.Time, error) {
	f, err := os.Open(infoPath)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("read trash info: %w", err)
	}
	defer f.Close()

	var rawPath, rawDate string
	inSection := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inSection = line == "[Trash Info]"
			continue
		}
		if !inSection {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Path":
			if rawPath == "" {
				rawPath = strings.TrimSpace(value)
			}
		case "DeletionDate":
			if rawDate == "" {
				rawDate = strings.TrimSpace(value)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", time.Time{}, fmt.Errorf("read trash info: %w", err)
	}
	if rawPath == "" {
		return "", time.Time{}, fmt.Errorf("trash info %s has no Path", infoPath)
	}
	decoded, err := url.PathUnescape(rawPath)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("decode trash info path: %w", err)
	}
	var deleted time.Time
	if rawDate != "" {
		if t, err := time.ParseInLocation(dateLayout, rawDate, time.Local); err == nil {
			deleted = t
		}
	}
	return decoded, deleted, nil
}

// trashOf finds the trash root and files dir that contain abs, if any.
func trashOf(abs string) (string, string, bool) {
	home := homeTrashDir()
	for cur := abs; ; {
		if filepath.Base(cur) == "files" {
			root := filepath.Dir(cur)
			if root == home || isTopDirTrashRoot(root) {
				return root, cur, true
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", "", false
		}
		cur = parent
	}
}

func isTopDirTrashRoot(root string) bool {
	_, ok := trashTopDir(root)
	return ok
}

// trashTopDir returns the topdir a .Trash-$uid or .Trash/$uid root belongs to.
func trashTopDir(root string) (string, bool) {
	base := filepath.Base(root)
	if uid, ok := strings.CutPrefix(base, ".Trash-"); ok && isDigits(uid) {
		return filepath.Dir(root), true
	}
	if isDigits(base) && filepath.Base(filepath.Dir(root)) == ".Trash" {
		return filepath.Dir(filepath.Dir(root)), true
	}
	return "", false
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

func lstatDev(path string) (uint64, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Dev), nil
}

func statDev(path string) (uint64, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Dev), nil
}
