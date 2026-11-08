package archive

import "golang.org/x/sys/unix"

// availableBytes returns the space an unprivileged user can still write on the
// filesystem holding path.
func availableBytes(path string) (int64, bool) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, false
	}
	return int64(stat.Bavail) * stat.Bsize, true
}
