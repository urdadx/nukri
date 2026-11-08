/*
This file is for loading filesystem entries and generating preview data for the user interface. It includes functions to load the current working directory, read directory entries, and generate metadata for each entry. It also provides functionality to load previews for files and directories, including handling errors and unsupported file types.
The file also includes utility functions to determine the appropriate icon for each entry based on its name and type, as well as counting visible items in directories while excluding hidden files.
*/

package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

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
	entries      []browser.Entry
	err          error
}

// placesMsg carries freshly built sidebar rows.
type placesMsg struct {
	rows []core.SidebarRow
}

// placesRefreshInterval is how often the sidebar is rebuilt to pick up mounted
// and unmounted devices.
const placesRefreshInterval = 5 * time.Second

// loadPlaces builds the sidebar off the UI loop. Building reads mount tables and
// stats every place, so it runs at startup and on a timer, not per navigation.
func loadPlaces() tea.Msg {
	return placesMsg{rows: places.BuildSidebarRows()}
}

// refreshPlaces schedules the next sidebar rebuild. It is only re-armed after a
// rebuild finishes, so a slow mount can't stack up rebuilds.
func refreshPlaces() tea.Cmd {
	return tea.Tick(placesRefreshInterval, func(time.Time) tea.Msg { return loadPlaces() })
}

// itemCountsMsg carries the visible child counts for the directories listed in
// cwd. Counting needs one directory read per subdirectory, so it runs after the
// listing is shown instead of delaying it.
type itemCountsMsg struct {
	cwd     string // the directory whose subdirectories were counted
	counts  map[string]int
	preview bool // counts for the preview pane rather than the file list
}

type previewMsg struct {
	path      string
	view      preview.View
	entries   []browser.Entry
	directory bool
	err       error
	// cancelled marks a render that was superseded; it carries no preview.
	cancelled bool
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
		return loadedMsg{cwd: cwd, err: fmt.Errorf("read current directory: %w", err)}
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
		entries = append(entries, browser.Entry{Entry: entry, Facts: facts, Icon: icon.Icon, IconColor: icon.Color})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Entry.IsDirectory() != entries[j].Entry.IsDirectory() {
			return entries[i].Entry.IsDirectory()
		}
		return entries[i].Entry.NameKey < entries[j].Entry.NameKey
	})
	return loadedMsg{cwd: cwd, entries: entries}
}

func loadPreview(pool *PreviewPool, entry browser.Entry, width, codeWindow int, syntaxStyle string) tea.Cmd {
	done := pool.Submit(entry, width, syntaxStyle, codeWindow, true)
	return func() tea.Msg {
		return <-done
	}
}

func loadItemCounts(cwd string, entries []browser.Entry, forPreview bool) tea.Cmd {
	var directories []string
	for _, entry := range entries {
		if entry.Entry.IsDirectory() {
			directories = append(directories, entry.Entry.Path)
		}
	}
	if len(directories) == 0 {
		return nil
	}
	return func() tea.Msg {
		counts := make(map[string]int, len(directories))
		for _, path := range directories {
			if count := visibleItemCount(path); count != nil {
				counts[path] = *count
			}
		}
		return itemCountsMsg{cwd: cwd, counts: counts, preview: forPreview}
	}
}

// applyItemCounts returns a copy of entries with the counts filled in.
func applyItemCounts(entries []browser.Entry, counts map[string]int) []browser.Entry {
	updated := slices.Clone(entries)
	for i := range updated {
		if count, ok := counts[updated[i].Entry.Path]; ok {
			updated[i].ItemCount = &count
		}
	}
	return updated
}

func visibleItemCount(path string) *int {
	directory, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer directory.Close()
	count := 0
	for {
		// Read in batches so huge directories don't allocate every name at once.
		names, err := directory.Readdirnames(1024)
		for _, name := range names {
			// exclude hidden files and directories from the count. example: .git, .vscode
			if !strings.HasPrefix(name, ".") {
				count++
			}
		}
		if errors.Is(err, io.EOF) {
			return &count
		}
		if err != nil {
			return nil
		}
	}
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
		// Item counts are filled in later (loadItemCounts): each needs its own
		// directory read, which would hold up the preview.
		entries = append(entries, browser.Entry{Entry: entry, Icon: icon.Icon, IconColor: icon.Color})
	}
	return entries
}

func iconFor(entry core.Entry) icons.Style {
	if entry.IsDirectory() {
		if icon, ok := icons.Folders[entry.Name]; ok {
			return icon
		}
		return icons.Current().Folder
	}
	name := strings.ToLower(entry.Name)
	// Some icons are keyed by full file name, such as gruntfile.js or jenkinsfile.
	if icon, ok := icons.Icons[name]; ok {
		return icon
	}
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
