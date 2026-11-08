package preview

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

/*
listArchive lists an archive's entries. ZIP-based containers (.zip, .jar, .cbz,
.apk, ...) are read with the standard library first: it reads only the central
directory, in a few milliseconds, where running 7-Zip and parsing its text
listing took over 100ms for a large archive. Anything else, or a ZIP the
standard library rejects, goes to 7-Zip.
*/
func (s *Service) listArchive(ctx context.Context, path string) (*ArchivePreview, error) {
	if compression, ok := tarCompression(path); ok {
		return s.listTar(ctx, path, compression)
	}
	if archive, ok := parseZipListing(ctx, path, s.maxArchiveEntries); ok {
		return &ArchivePreview{Archive: archive}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.tools.SevenZip != "" {
		archive, err := s.listSevenZip(ctx, path)
		if err == nil {
			return &ArchivePreview{Archive: archive}, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
	}
	if s.tools.SevenZip == "" {
		return nil, fmt.Errorf("archive preview: %w", ToolUnavailable("7zz"))
	}
	return nil, fmt.Errorf("list archive: 7z could not read %q", path)
}

// parseZipListing lists the contents of a ZIP container using the standard
// library. It returns ok=false when the file is not a readable ZIP archive.
func parseZipListing(ctx context.Context, path string, maximumEntries int) (Archive, bool) {
	file, err := zip.OpenReader(path)
	if err != nil {
		return Archive{}, false
	}
	defer file.Close()

	archive := Archive{}
	for _, f := range file.File {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Archive{}, false
		}
		if len(archive.Entries) >= maximumEntries {
			archive.Truncated = true
			break
		}
		mode := f.Mode()
		entry := ArchiveEntry{
			Path:       safeText(f.Name),
			Size:       int64(f.UncompressedSize64),
			PackedSize: int64(f.CompressedSize64),
			Modified:   zipTimeLabel(f.Modified),
			Attributes: mode.String(), // as 7-Zip shows them, e.g. -rw-r--r--
			Directory:  mode.IsDir(),
		}
		archive.Entries = append(archive.Entries, entry)
	}
	return archive, true
}

func zipTimeLabel(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

/*
listSevenZip runs 7-Zip's technical listing and parses it as it streams. Large
archives produce megabytes of key = value lines; reading them as they arrive
avoids collecting the whole listing first, and 7-Zip is stopped as soon as
maxArchiveEntries are listed.
*/
func (s *Service) listSevenZip(parent context.Context, path string) (Archive, error) {
	ctx, cancel := context.WithTimeout(parent, toolTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, s.tools.SevenZip, "l", "-slt", "-ba", "--", path)
	stderr := &limitedBuffer{limit: 64 << 10}
	command.Stderr = stderr
	output, err := command.StdoutPipe()
	if err != nil {
		return Archive{}, err
	}
	if err := command.Start(); err != nil {
		return Archive{}, err
	}
	archive, complete, readErr := parseSevenZipListing(output, s.maxArchiveEntries)
	if !complete {
		// Stopped at the entry limit: 7-Zip may still be writing.
		_ = command.Process.Kill()
		_ = command.Wait()
		return archive, readErr
	}
	if err := command.Wait(); err != nil {
		if parentErr := parent.Err(); parentErr != nil {
			return Archive{}, parentErr
		}
		return Archive{}, fmt.Errorf("%s failed: %w: %s", s.tools.SevenZip, err, bytes.TrimSpace(stderr.buffer.Bytes()))
	}
	return archive, readErr
}

// parseSevenZipListing parses 7-Zip's "key = value" listing, where a blank line
// ends each entry. complete is false when it stopped at maximumEntries.
func parseSevenZipListing(listing io.Reader, maximumEntries int) (archive Archive, complete bool, err error) {
	entry := ArchiveEntry{}
	hasEntry := false
	flush := func() bool {
		if !hasEntry || entry.Path == "" {
			entry = ArchiveEntry{}
			hasEntry = false
			return true
		}
		if len(archive.Entries) >= maximumEntries {
			archive.Truncated = true
			return false
		}
		entry.Directory = strings.HasPrefix(entry.Attributes, "D") || strings.HasSuffix(entry.Path, "/")
		archive.Entries = append(archive.Entries, entry)
		entry = ArchiveEntry{}
		hasEntry = false
		return true
	}

	scanner := bufio.NewScanner(listing)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			if !flush() {
				return archive, false, nil
			}
			continue
		}
		name, value, ok := strings.Cut(line, " = ")
		if !ok {
			continue
		}
		hasEntry = true
		switch strings.TrimSpace(name) {
		case "Path":
			entry.Path = safeText(value)
		case "Size":
			entry.Size, _ = strconv.ParseInt(value, 10, 64)
		case "Packed Size":
			entry.PackedSize, _ = strconv.ParseInt(value, 10, 64)
		case "Modified":
			entry.Modified = safeText(value)
		case "Attributes":
			entry.Attributes = safeText(value)
		}
	}
	if !flush() {
		return archive, false, nil
	}
	return archive, true, scanner.Err()
}
