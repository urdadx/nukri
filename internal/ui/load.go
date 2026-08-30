/*
This file is for loading filesystem entries and generating preview data for the user interface. It includes functions to load the current working directory, read directory entries, and generate metadata for each entry. It also provides functionality to load previews for files and directories, including handling errors and unsupported file types.
The file also includes utility functions to determine the appropriate icon for each entry based on its name and type, as well as counting visible items in directories while excluding hidden files.
*/

package ui

import (
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
	cwd          string
	selectedPath string
	places       []core.SidebarRow
	entries      []browser.Entry
	err          error
}

type previewMsg struct {
	path      string
	view      preview.View
	entries   []browser.Entry
	directory bool
	err       error
}

func loadFilesystem() tea.Msg {
	cwd, err := os.Getwd()
	if err != nil {
		return loadedMsg{err: fmt.Errorf("get current directory: %w", err)}
	}
	return loadDirectoryMessage(cwd)
}

func loadDirectory(path, selectedPath string) tea.Cmd {
	return func() tea.Msg {
		message := loadDirectoryMessage(path).(loadedMsg)
		message.selectedPath = selectedPath
		return message
	}
}

func loadDirectoryMessage(cwd string) tea.Msg {
	directoryEntries, err := os.ReadDir(cwd)
	if err != nil {
		return loadedMsg{cwd: cwd, places: places.BuildSidebarRows(), err: fmt.Errorf("read current directory: %w", err)}
	}
	entries := make([]browser.Entry, 0, len(directoryEntries))
	for _, directoryEntry := range directoryEntries {
		// if it is an hidden file or directory. example: .git, .vscode
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
		loadedEntry := browser.Entry{Entry: entry, Facts: facts, Icon: icon.Icon, IconColor: icon.Color}
		if entry.IsDirectory() {
			loadedEntry.ItemCount = visibleItemCount(entry.Path)
		}
		entries = append(entries, loadedEntry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Entry.IsDirectory() != entries[j].Entry.IsDirectory() {
			return entries[i].Entry.IsDirectory()
		}
		return entries[i].Entry.NameKey < entries[j].Entry.NameKey
	})
	return loadedMsg{cwd: cwd, places: places.BuildSidebarRows(), entries: entries}
}

func loadPreview(pool *PreviewPool, entry browser.Entry, width, codeWindow int, syntaxStyle string) tea.Cmd {
	done := pool.Submit(entry, width, syntaxStyle, codeWindow, true)
	return func() tea.Msg {
		return <-done
	}
}

// extendPreview re-requests a code preview with a larger leading-line window so
// the already-loaded lines are preserved while more are rendered. It runs at
// high priority but carries a distinct cache key (the larger window), so it does
// not collide with — and cheaply reuses the disk cache written by — the earlier
// preview render.
func extendPreview(pool *PreviewPool, entry browser.Entry, width, codeWindow int, syntaxStyle string) tea.Cmd {
	done := pool.Submit(entry, width, syntaxStyle, codeWindow, true)
	return func() tea.Msg {
		return <-done
	}
}

func visibleItemCount(path string) *int {
	directory, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer directory.Close()
	names, err := directory.Readdirnames(-1)
	if err != nil {
		return nil
	}
	count := 0
	for _, name := range names {
		// exlude hidden files and directories from the count. example: .git, .vscode
		if !strings.HasPrefix(name, ".") {
			count++
		}
	}
	return &count
}

func directoryPreviewEntries(parent string, values []preview.DirectoryEntry) []browser.Entry {
	entries := make([]browser.Entry, 0, len(values))
	for _, value := range values {
		if strings.HasPrefix(value.Name, ".") {
			continue
		}
		kind := core.File
		if value.Directory {
			kind = core.Directory
		}
		entry := core.Entry{Name: value.Name, Path: filepath.Join(parent, value.Name), Kind: kind, Size: value.Size}
		icon := iconFor(entry)
		previewEntry := browser.Entry{Entry: entry, Icon: icon.Icon, IconColor: icon.Color}
		if entry.IsDirectory() {
			previewEntry.ItemCount = visibleItemCount(entry.Path)
		}
		entries = append(entries, previewEntry)
	}
	return entries
}

func iconFor(entry core.Entry) icons.Style {
	if entry.IsDirectory() {
		if icon, ok := icons.Folders[entry.Name]; ok {
			return icon
		}
		return icons.Folders["folder"]
	}
	// what is the code below for? it is
	name := strings.ToLower(entry.Name)
	key := name
	if alias, ok := icons.Aliases[key]; ok {
		key = alias
	} else {
		// uses the file extension as the key for icon lookup. example: .txt, .md, .go
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
