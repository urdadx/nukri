//go:build !linux

package archive

// availableBytes is only implemented on Linux; elsewhere extraction size is
// limited only by the entry count.
func availableBytes(string) (int64, bool) { return 0, false }
