package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/config/icons"
	"github.com/urdadx/nukri/internal/core"
	fileinfo "github.com/urdadx/nukri/internal/file_info"
	"github.com/urdadx/nukri/internal/fs/places"
	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/ui/browser"
)

type loadedMsg struct {
	cwd     string
	places  []core.SidebarRow
	entries []browser.Entry
	err     error
}

type previewMsg struct {
	path string
	view preview.View
	err  error
}

func loadFilesystem() tea.Msg {
	cwd, err := os.Getwd()
	if err != nil {
		return loadedMsg{err: fmt.Errorf("get current directory: %w", err)}
	}
	directoryEntries, err := os.ReadDir(cwd)
	if err != nil {
		return loadedMsg{cwd: cwd, places: places.BuildSidebarRows(), err: fmt.Errorf("read current directory: %w", err)}
	}
	entries := make([]browser.Entry, 0, len(directoryEntries))
	for _, directoryEntry := range directoryEntries {
		if strings.HasPrefix(directoryEntry.Name(), ".") {
			continue
		}
		info, infoErr := directoryEntry.Info()
		if infoErr != nil {
			continue
		}
		kind := core.File
		if directoryEntry.IsDir() {
			kind = core.Directory
		}
		entry := core.Entry{
			Name: directoryEntry.Name(), Path: filepath.Join(cwd, directoryEntry.Name()), Kind: kind,
			Size: info.Size(), Modified: info.ModTime(), NameKey: strings.ToLower(directoryEntry.Name()),
		}
		facts := fileinfo.InspectEntryFast(&entry)
		icon := iconFor(entry)
		entries = append(entries, browser.Entry{Entry: entry, Facts: facts, Icon: icon.Icon, IconColor: icon.Color})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Entry.IsDirectory() != entries[j].Entry.IsDirectory() {
			return entries[i].Entry.IsDirectory()
		}
		return entries[i].Entry.NameKey < entries[j].Entry.NameKey
	})
	return loadedMsg{cwd: cwd, places: places.BuildSidebarRows(), entries: entries}
}

func loadPreview(service *preview.Service, entry browser.Entry, width int) tea.Cmd {
	return func() tea.Msg {
		value, err := service.Render(context.Background(), preview.Request{Path: entry.Entry.Path, Facts: entry.Facts, Width: max(1, width)})
		if err != nil {
			if errors.Is(err, preview.ErrUnsupported) {
				err = fmt.Errorf("preview is not available for this file type")
			} else if errors.Is(err, preview.ErrToolUnavailable) {
				err = fmt.Errorf("the required preview tool is not installed")
			}
			return previewMsg{path: entry.Entry.Path, err: err}
		}
		view, err := preview.BuildView(value, preview.ViewOptions{Width: max(1, width)})
		return previewMsg{path: entry.Entry.Path, view: view, err: err}
	}
}

func iconFor(entry core.Entry) icons.Style {
	if entry.IsDirectory() {
		if icon, ok := icons.Folders[entry.Name]; ok {
			return icon
		}
		return icons.Folders["folder"]
	}
	name := strings.ToLower(entry.Name)
	key := name
	if alias, ok := icons.Aliases[key]; ok {
		key = alias
	} else {
		key = strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		if alias, ok := icons.Aliases[key]; ok {
			key = alias
		}
	}
	if icon, ok := icons.Icons[key]; ok {
		return icon
	}
	return icons.Icons["file"]
}
