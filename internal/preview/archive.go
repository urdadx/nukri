package preview

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (s *Service) listArchive(ctx context.Context, path string) (*ArchivePreview, error) {
	if s.tools.SevenZip != "" {
		output, err := runCommand(ctx, s.maxToolOutput, s.tools.SevenZip, "l", "-slt", "-ba", "--", path)
		if err == nil {
			archive := parseSevenZipListing(string(output), s.maxArchiveEntries)
			return &ArchivePreview{Archive: archive}, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
	}
	// Fall back to the standard library for ZIP-based containers (.zip, .jar,
	// .cbz, .apk, .aab, .apkg) so those preview without an external tool.
	if archive, ok := parseZipListing(ctx, path, s.maxArchiveEntries); ok {
		return &ArchivePreview{Archive: archive}, nil
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
		entry := ArchiveEntry{
			Path:       safeText(f.Name),
			Size:       int64(f.UncompressedSize64),
			PackedSize: int64(f.CompressedSize64),
			Modified:   zipTimeLabel(f.Modified),
			Directory:  f.FileInfo().IsDir(),
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

func parseSevenZipListing(output string, maximumEntries int) Archive {
	archive := Archive{}
	entry := ArchiveEntry{}
	hasEntry := false
	// why do we need to flush? because the 7z output is a series of key-value pairs, and an empty line indicates the end of an entry. So we need to flush the current entry when we encounter an empty line, or when we reach the maximum number of entries.
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

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			if !flush() {
				break
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
	flush()
	return archive
}
