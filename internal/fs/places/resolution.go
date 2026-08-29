/*
*
	Resolves configured places into rows displayed by the terminal file manager's sidebar. This includes built-in places (Home, Desktop, Documents, Downloads, Pictures, Music, Videos, Root, Trash) and custom places defined by the user.
	  *
	- The resolution process involves:
	- 1. Determining the actual filesystem paths for built-in places based on the user's environment (e.g., XDG user directories).
	- 2. Validating that these paths exist and are directories.
	- 3. Creating SidebarItem instances for each valid place, including title, icon, and identity path.
	- 4. Handling custom places specified in the configuration, resolving their paths and icons.
	- 5. Optionally including mounted devices if configured to do so.
	  *
*/

package places

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/urdadx/nukri/internal/config"
	"github.com/urdadx/nukri/internal/config/icons"
	"github.com/urdadx/nukri/internal/core"
)

type PlaceResolutionContext struct {
	Home      string
	Desktop   *string
	Documents *string
	Downloads *string
	Pictures  *string
	Music     *string
	Videos    *string
	Root      *string
	Trash     *string
}

func BuildSidebarRows() []core.SidebarRow {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/"
	}
	context := systemPlaceResolutionContext(home)
	return buildSidebarRowsWithContext(config.DefaultPlacesConfig(), &context)
}

func buildSidebarRowsWithContext(places config.PlacesConfig, context *PlaceResolutionContext) []core.SidebarRow {
	pinnedItems := buildPinnedSidebarItems(places, context)
	pinnedPaths := make(map[string]struct{}, len(pinnedItems))
	rows := make([]core.SidebarRow, 0, len(pinnedItems))
	for i := range pinnedItems {
		pinnedPaths[pinnedItems[i].IdentityPath] = struct{}{}
		item := pinnedItems[i]
		rows = append(rows, core.SidebarRow{Item: &item})
	}

	if places.ShowDevices {
		devices := mountedDeviceItems(context.Home, pinnedPaths)
		if len(devices) > 0 {
			rows = append(rows, core.SidebarRow{Section: "Devices"})
			for i := range devices {
				item := devices[i]
				rows = append(rows, core.SidebarRow{Item: &item})
			}
		}
	}
	return rows
}

func systemPlaceResolutionContext(home string) PlaceResolutionContext {
	root := "/"
	return PlaceResolutionContext{
		Home:      home,
		Desktop:   existingDir(xdg.UserDirs.Desktop),
		Documents: existingDir(xdg.UserDirs.Documents),
		Downloads: existingDir(xdg.UserDirs.Download),
		Pictures:  existingDir(xdg.UserDirs.Pictures),
		Music:     existingDir(xdg.UserDirs.Music),
		Videos:    existingDir(xdg.UserDirs.Videos),
		Root:      &root,
		Trash:     trashDir(home),
	}
}

func existingDir(path string) *string {
	if path == "" {
		return nil
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return nil
	}
	return &path
}

func buildPinnedSidebarItems(places config.PlacesConfig, context *PlaceResolutionContext) []core.SidebarItem {
	items := make([]core.SidebarItem, 0, len(places.Entries))
	seen := make(map[string]struct{}, len(places.Entries))
	for _, entry := range places.Entries {
		item, ok := resolvePlaceEntry(entry, context)
		if !ok {
			continue
		}
		if _, exists := seen[item.IdentityPath]; exists {
			continue
		}
		seen[item.IdentityPath] = struct{}{}
		items = append(items, item)
	}
	return items
}

func resolvePlaceEntry(entry config.PlaceEntrySpec, context *PlaceResolutionContext) (core.SidebarItem, bool) {
	if entry.Builtin != nil {
		return resolveBuiltinPlace(*entry.Builtin, entry.Icon, context)
	}
	return sidebarItem(core.Custom, entry.Title, placeIcon(entry.Path, entry.Icon, icons.CustomPlace), entry.Path), true
}

func resolveBuiltinPlace(place config.BuiltinPlace, icon string, context *PlaceResolutionContext) (core.SidebarItem, bool) {
	type placeData struct {
		kind      core.SidebarItemKind
		title     string
		icon      string
		path      *string
		localized bool
	}
	data := map[config.BuiltinPlace]placeData{
		config.Home:      {core.Home, "Home", icons.Home, &context.Home, false},
		config.Desktop:   {core.Desktop, "Desktop", icons.Desktop, context.Desktop, true},
		config.Documents: {core.Documents, "Documents", icons.Documents, context.Documents, true},
		config.Downloads: {core.Downloads, "Downloads", icons.Downloads, context.Downloads, true},
		config.Pictures:  {core.Pictures, "Pictures", icons.Pictures, context.Pictures, true},
		config.Music:     {core.Music, "Music", icons.Music, context.Music, true},
		config.Videos:    {core.Videos, "Videos", icons.Videos, context.Videos, true},
		config.Root:      {core.Root, "Root", icons.Root, context.Root, false},
		config.Trash:     {core.Trash, "Trash", icons.Trash, context.Trash, false},
	}[place]
	if data.path == nil {
		return core.SidebarItem{}, false
	}
	if data.localized {
		if title := filepath.Base(*data.path); title != "." && title != string(filepath.Separator) {
			data.title = title
		}
	}
	return sidebarItem(data.kind, data.title, placeIcon(*data.path, icon, data.icon), *data.path), true
}

func placeIcon(path, override, defaultIcon string) string {
	if override != "" {
		return override
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return defaultIcon
	}
	if target, err := os.Stat(path); err == nil && target.IsDir() {
		return icons.LinkedPlace
	}
	return icons.BrokenPlace
}

func sidebarItem(kind core.SidebarItemKind, title, icon, path string) core.SidebarItem {
	return core.NewSidebarItem(kind, title, icon, path, pathIdentityKey(path))
}

func pathIdentityKey(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

func trashDir(home string) *string {
	dataHome := xdg.DataHome
	if processHome, err := os.UserHomeDir(); err != nil || processHome != home {
		dataHome = filepath.Join(home, ".local", "share")
	}
	path := filepath.Join(dataHome, "Trash", "files")
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return &path
	}
	return nil
}
