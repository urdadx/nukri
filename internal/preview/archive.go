package preview

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func (s *Service) listArchive(ctx context.Context, path string) (*ArchivePreview, error) {
	if s.tools.SevenZip == "" {
		return nil, fmt.Errorf("archive preview: %w", ErrToolUnavailable)
	}
	output, err := runCommand(ctx, s.maxToolOutput, s.tools.SevenZip, "l", "-slt", "-ba", "--", path)
	if err != nil {
		return nil, fmt.Errorf("list archive: %w", err)
	}
	archive := parseSevenZipListing(string(output), s.maxArchiveEntries)
	return &ArchivePreview{Archive: archive}, nil
}

func parseSevenZipListing(output string, maximumEntries int) Archive {
	archive := Archive{}
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
