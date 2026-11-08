package apps

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// App is a launchable desktop application.
type App struct {
	ID       string // desktop file ID, e.g. "org.gnome.gedit.desktop"
	Name     string
	Exec     string
	Terminal bool
	Icon     string
	File     string // absolute path of the .desktop file (%k)
	Dir      string // working directory from the Path key, if any

	mimeTypes []string
	noDisplay bool
	hidden    bool
}

// homeDir returns $HOME, or "" when unknown.
func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitDirs(list string) []string {
	var out []string
	for _, d := range strings.Split(list, ":") {
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}

func dataHome() string {
	return envOr("XDG_DATA_HOME", filepath.Join(homeDir(), ".local", "share"))
}

func dataDirs() []string {
	return splitDirs(envOr("XDG_DATA_DIRS", "/usr/local/share:/usr/share"))
}

// applicationDirs lists the applications directories in precedence order.
func applicationDirs() []string {
	dirs := []string{filepath.Join(dataHome(), "applications")}
	for _, d := range dataDirs() {
		dirs = append(dirs, filepath.Join(d, "applications"))
	}
	return dirs
}

// loadApps scans every applications directory and returns all entries by ID,
// including hidden ones (they still shadow lower-precedence IDs).
func loadApps() map[string]*App {
	apps := make(map[string]*App)
	for _, root := range applicationDirs() {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() && path != root {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() || !strings.HasSuffix(path, ".desktop") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			id := strings.ReplaceAll(filepath.ToSlash(rel), "/", "-")
			if _, seen := apps[id]; seen {
				return nil
			}
			app, err := parseDesktopFile(path)
			if err != nil {
				return nil
			}
			app.ID = id
			apps[id] = app
			return nil
		})
	}
	return apps
}

// parseDesktopFile reads the [Desktop Entry] group of a desktop file. Entries
// that are not applications or whose TryExec is missing come back hidden.
func parseDesktopFile(path string) (*App, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open desktop file: %w", err)
	}
	defer f.Close()

	app := &App{File: path}
	var typ, tryExec string
	inGroup := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		if line[0] == '[' {
			inGroup = line == "[Desktop Entry]"
			continue
		}
		if !inGroup {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if strings.Contains(key, "[") {
			continue // localized key
		}
		switch key {
		case "Type":
			typ = val
		case "Name":
			app.Name = unescapeValue(val)
		case "Exec":
			app.Exec = unescapeValue(val)
		case "Icon":
			app.Icon = unescapeValue(val)
		case "Path":
			app.Dir = unescapeValue(val)
		case "TryExec":
			tryExec = unescapeValue(val)
		case "MimeType":
			app.mimeTypes = splitList(val)
		case "Terminal":
			app.Terminal = val == "true"
		case "NoDisplay":
			app.noDisplay = val == "true"
		case "Hidden":
			app.hidden = val == "true"
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read desktop file: %w", err)
	}
	if typ != "Application" || app.Exec == "" || (tryExec != "" && !executableExists(tryExec)) {
		app.hidden = true
	}
	return app, nil
}

// unescapeValue applies the spec's string escapes: \s \n \t \r \\.
func unescapeValue(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 == len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 's':
			b.WriteByte(' ')
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// splitList splits a semicolon-separated list, honouring "\;" escapes.
func splitList(s string) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == ';' {
			cur.WriteByte(';')
			i++
			continue
		}
		if s[i] == ';' {
			if v := strings.TrimSpace(cur.String()); v != "" {
				out = append(out, v)
			}
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	if v := strings.TrimSpace(cur.String()); v != "" {
		out = append(out, v)
	}
	return out
}

func executableExists(name string) bool {
	if filepath.IsAbs(name) {
		info, err := os.Stat(name)
		return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
	}
	_, err := exec.LookPath(name)
	return err == nil
}
