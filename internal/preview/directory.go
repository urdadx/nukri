package preview

import (
	"context"
	"fmt"
	"os"
)

func (s *Service) renderDirectory(ctx context.Context, path string) (*DirectoryPreview, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	folderCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			folderCount++
		}
	}
	fileCount := len(entries) - folderCount
	shown := min(len(entries), s.maxDirectoryEntries)
	result := &DirectoryPreview{
		Entries: make([]DirectoryEntry, 0, shown), TotalItems: len(entries),
		FolderCount: folderCount, FileCount: fileCount, Truncated: shown < len(entries),
	}
	for index, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if index >= s.maxDirectoryEntries {
			break
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect directory entry %q: %w", entry.Name(), err)
		}
		result.Entries = append(result.Entries, DirectoryEntry{
			Name: safeText(entry.Name()), Size: info.Size(), Mode: info.Mode().String(),
			Modified: info.ModTime().Format("2006-01-02 15:04:05"), Directory: entry.IsDir(),
			Symlink: entry.Type()&os.ModeSymlink != 0,
		})
	}
	return result, nil
}
