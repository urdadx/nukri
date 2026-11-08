package apps

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// associations is the merged view of every mimeapps.list file.
type associations struct {
	defaults map[string][]string        // mime -> candidate IDs, highest precedence first
	added    map[string][]string        // mime -> IDs in precedence order
	removed  map[string]map[string]bool // mime -> removed IDs
}

// mimeappsFiles lists mimeapps.list paths in precedence order.
func mimeappsFiles() []string {
	var desktops []string
	for _, d := range strings.Split(os.Getenv("XDG_CURRENT_DESKTOP"), ":") {
		if d = strings.ToLower(strings.TrimSpace(d)); d != "" {
			desktops = append(desktops, d)
		}
	}
	var dirs []string
	dirs = append(dirs, envOr("XDG_CONFIG_HOME", filepath.Join(homeDir(), ".config")))
	dirs = append(dirs, splitDirs(envOr("XDG_CONFIG_DIRS", "/etc/xdg"))...)
	dirs = append(dirs, applicationDirs()...)

	var files []string
	for _, dir := range dirs {
		for _, d := range desktops {
			files = append(files, filepath.Join(dir, d+"-mimeapps.list"))
		}
		files = append(files, filepath.Join(dir, "mimeapps.list"))
	}
	return files
}

func loadAssociations() associations {
	a := associations{
		defaults: make(map[string][]string),
		added:    make(map[string][]string),
		removed:  make(map[string]map[string]bool),
	}
	for _, path := range mimeappsFiles() {
		groups, err := parseMimeapps(path)
		if err != nil {
			continue
		}
		for mt, ids := range groups["Default Applications"] {
			a.defaults[mt] = append(a.defaults[mt], ids...)
		}
		// Added entries are kept unless a higher-precedence file removed them.
		for mt, ids := range groups["Added Associations"] {
			for _, id := range ids {
				if !a.removed[mt][id] {
					a.added[mt] = append(a.added[mt], id)
				}
			}
		}
		for mt, ids := range groups["Removed Associations"] {
			if a.removed[mt] == nil {
				a.removed[mt] = make(map[string]bool)
			}
			for _, id := range ids {
				a.removed[mt][id] = true
			}
		}
	}
	return a
}

// parseMimeapps returns group -> mime -> IDs for one mimeapps.list file.
func parseMimeapps(path string) (map[string]map[string][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open mimeapps.list: %w", err)
	}
	defer f.Close()
	groups := make(map[string]map[string][]string)
	var cur map[string][]string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			name := line[1 : len(line)-1]
			if groups[name] == nil {
				groups[name] = make(map[string][]string)
			}
			cur = groups[name]
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok || cur == nil {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		cur[key] = append(cur[key], splitList(val)...)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read mimeapps.list: %w", err)
	}
	return groups, nil
}

// AppsFor lists the applications that can open mimeType: the default app
// first, then added associations, then every other handler sorted by name.
// For text/* types, text/plain handlers follow as a lower-priority tail.
func AppsFor(mimeType string) ([]App, error) {
	mimeType = stripParams(mimeType)
	major, _, ok := strings.Cut(mimeType, "/")
	if !ok || major == "" {
		return nil, fmt.Errorf("invalid mime type %q", mimeType)
	}
	installed := loadApps()
	assoc := loadAssociations()

	var out []App
	seen := make(map[string]bool)
	removed := func(id string, keys []string) bool {
		for _, k := range keys {
			if assoc.removed[k][id] {
				return true
			}
		}
		return false
	}
	add := func(id string, explicit bool) bool {
		app, ok := installed[id]
		if !ok || seen[id] || app.hidden || (app.noDisplay && !explicit) {
			return false
		}
		seen[id] = true
		out = append(out, *app)
		return true
	}

	tiers := [][]string{{mimeType, major + "/*"}}
	if major == "text" && mimeType != "text/plain" {
		tiers = append(tiers, []string{"text/plain"})
	}
	for _, keys := range tiers {
		for _, k := range keys {
			if done := addFirst(assoc.defaults[k], add); done {
				break
			}
		}
		for _, k := range keys {
			for _, id := range assoc.added[k] {
				add(id, true)
			}
		}
		var rest []*App
		for id, app := range installed {
			if !seen[id] && handles(app, keys) && !removed(id, append(keys, mimeType)) {
				rest = append(rest, app)
			}
		}
		sort.Slice(rest, func(i, j int) bool {
			ni, nj := strings.ToLower(rest[i].Name), strings.ToLower(rest[j].Name)
			if ni != nj {
				return ni < nj
			}
			return rest[i].ID < rest[j].ID
		})
		for _, app := range rest {
			add(app.ID, false)
		}
	}
	return out, nil
}

// DefaultFor returns the preferred application for mimeType, if any.
func DefaultFor(mimeType string) (App, bool, error) {
	list, err := AppsFor(mimeType)
	if err != nil || len(list) == 0 {
		return App{}, false, err
	}
	return list[0], true, nil
}

// addFirst adds the first installable ID and reports whether one was added.
func addFirst(ids []string, add func(string, bool) bool) bool {
	for _, id := range ids {
		if add(id, true) {
			return true
		}
	}
	return false
}

// handles reports whether app lists any of keys, or a wildcard covering them.
func handles(app *App, keys []string) bool {
	for _, mt := range app.mimeTypes {
		mt = strings.ToLower(mt)
		for _, k := range keys {
			if mt == k {
				return true
			}
			if strings.HasSuffix(mt, "/*") && strings.HasPrefix(k, strings.TrimSuffix(mt, "*")) {
				return true
			}
		}
	}
	return false
}
