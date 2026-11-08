// Package apps resolves MIME types and the desktop applications that can
// open them, following the XDG Desktop Entry and MIME Applications specs.
package apps

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// XDGMimeCommand is the binary MIMEType queries first. Set it to "" to skip
// it and always use the stdlib fallback.
var XDGMimeCommand = "xdg-mime"

// MIMEType reports the MIME type of path, preferring `xdg-mime query
// filetype` and falling back to FallbackMIMEType.
func MIMEType(ctx context.Context, path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return "inode/directory", nil
	}
	if XDGMimeCommand != "" {
		if bin, err := exec.LookPath(XDGMimeCommand); err == nil {
			out, err := exec.CommandContext(ctx, bin, "query", "filetype", path).Output()
			if t := strings.TrimSpace(string(out)); err == nil && strings.Contains(t, "/") {
				return stripParams(t), nil
			}
		}
	}
	return FallbackMIMEType(path)
}

// FallbackMIMEType guesses a MIME type using only the stdlib: the file
// extension first, then content sniffing of the first 512 bytes.
func FallbackMIMEType(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return "inode/directory", nil
	}
	if t := mime.TypeByExtension(filepath.Ext(path)); t != "" {
		return stripParams(t), nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return stripParams(http.DetectContentType(buf[:n])), nil
}

func stripParams(t string) string {
	if i := strings.IndexByte(t, ';'); i >= 0 {
		t = t[:i]
	}
	return strings.ToLower(strings.TrimSpace(t))
}
