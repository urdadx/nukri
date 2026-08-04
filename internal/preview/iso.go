package preview

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func (s *Service) renderISO(ctx context.Context, path string) (*ISOPreview, error) {
	if s.tools.ISOInfo == "" {
		return nil, fmt.Errorf("ISO preview: %w", ErrToolUnavailable)
	}
	descriptor, err := runCommand(ctx, 1<<20, s.tools.ISOInfo, "-d", "-i", path)
	if err != nil {
		return nil, fmt.Errorf("inspect ISO: %w", err)
	}
	info, metadata := parseISODescriptor(string(descriptor))
	if fileInfo, statErr := os.Stat(path); statErr == nil {
		info.FileSize = fileInfo.Size()
		metadata = append(metadata, Field{Name: "File size", Value: formatByteSize(info.FileSize)})
	}

	listing, err := s.listISO(ctx, path)
	if err != nil {
		return nil, err
	}
	entries, truncated := parseISOListing(listing, s.maxArchiveEntries)
	return &ISOPreview{ISO: info, Entries: entries, Metadata: metadata, Truncated: truncated}, nil
}

func (s *Service) listISO(ctx context.Context, path string) (string, error) {
	var lastErr error
	for _, extension := range []string{"-R", "-J", ""} {
		arguments := []string{"-f", "-i", path}
		if extension != "" {
			arguments = append([]string{extension}, arguments...)
		}
		output, err := runCommand(ctx, s.maxToolOutput, s.tools.ISOInfo, arguments...)
		if err == nil {
			return string(output), nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("list ISO: %w", lastErr)
}

func parseISODescriptor(output string) (ISO, []Field) {
	info := ISO{}
	fields := make([]Field, 0, 12)
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		lower := strings.ToLower(line)
		info.RockRidge = info.RockRidge || strings.Contains(lower, "rock ridge")
		info.Joliet = info.Joliet || strings.Contains(lower, "joliet")
		info.Bootable = info.Bootable || strings.Contains(lower, "el torito")
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = safeText(strings.TrimSpace(value))
		switch strings.ToLower(name) {
		case "system id":
			info.SystemID = value
		case "volume id":
			info.VolumeID = value
		case "volume set id":
			info.VolumeSetID = value
		case "publisher id":
			info.PublisherID = value
		case "data preparer id":
			info.DataPreparer = value
		case "application id":
			info.ApplicationID = value
		case "logical block size is":
			info.BlockSize, _ = strconv.ParseInt(value, 10, 64)
		case "volume size is":
			info.VolumeBlocks, _ = strconv.ParseInt(value, 10, 64)
		}
		if value != "" {
			fields = append(fields, Field{Name: name, Value: value})
		}
	}
	if info.RockRidge {
		fields = append(fields, Field{Name: "Rock Ridge", Value: "yes"})
	}
	if info.Joliet {
		fields = append(fields, Field{Name: "Joliet", Value: "yes"})
	}
	if info.Bootable {
		fields = append(fields, Field{Name: "Bootable", Value: "yes"})
	}
	return info, fields
}

func parseISOListing(output string, maximum int) ([]ISOEntry, bool) {
	entries := make([]ISOEntry, 0, min(128, maximum))
	truncated := false
	for line := range strings.SplitSeq(output, "\n") {
		line = safeText(strings.TrimSpace(strings.TrimSuffix(line, "\r")))
		if line == "" || line == "/" {
			continue
		}
		if len(entries) >= maximum {
			truncated = true
			break
		}
		entries = append(entries, ISOEntry{Path: line})
	}
	return entries, truncated
}
