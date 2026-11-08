package test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/urdadx/nukri/internal/apps"
)

type env struct {
	dataHome, sysData, configHome string
}

// setupEnv points every XDG variable at fresh temp directories.
func setupEnv(t *testing.T) env {
	t.Helper()
	root := t.TempDir()
	e := env{
		dataHome:   filepath.Join(root, "home", "share"),
		sysData:    filepath.Join(root, "usr", "share"),
		configHome: filepath.Join(root, "home", "config"),
	}
	t.Setenv("XDG_DATA_HOME", e.dataHome)
	t.Setenv("XDG_DATA_DIRS", e.sysData)
	t.Setenv("XDG_CONFIG_HOME", e.configHome)
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(root, "etc", "xdg"))
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	return e
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func desktop(name, exec, mime, extra string) string {
	return "[Desktop Entry]\nType=Application\nName=" + name + "\nName[de]=Lokal\nExec=" + exec +
		"\nMimeType=" + mime + "\n" + extra + "\n[Desktop Action new]\nName=Ignored\nExec=ignored\n"
}

func ids(list []apps.App) []string {
	out := []string{}
	for _, a := range list {
		out = append(out, a.ID)
	}
	return out
}

func mustApps(t *testing.T, mime string) []string {
	t.Helper()
	list, err := apps.AppsFor(mime)
	if err != nil {
		t.Fatal(err)
	}
	return ids(list)
}

func TestParsingAndShadowing(t *testing.T) {
	e := setupEnv(t)
	sys := filepath.Join(e.sysData, "applications")
	user := filepath.Join(e.dataHome, "applications")
	writeFile(t, filepath.Join(sys, "viewer.desktop"), desktop("System Viewer", "sysview %f", "image/png;", ""))
	writeFile(t, filepath.Join(user, "viewer.desktop"), desktop("User Viewer", "userview %f", "image/png;", "Terminal=true\nIcon=eye"))
	writeFile(t, filepath.Join(sys, "kde", "gwenview.desktop"), desktop("Gwenview", "gwenview %U", "image/png;", ""))

	list, err := apps.AppsFor("image/png")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(list), []string{"kde-gwenview.desktop", "viewer.desktop"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	v := list[1]
	if v.Name != "User Viewer" || v.Exec != "userview %f" || !v.Terminal || v.Icon != "eye" {
		t.Fatalf("user entry did not shadow system entry: %+v", v)
	}
}

func TestHiddenShadowsAndFiltering(t *testing.T) {
	e := setupEnv(t)
	sys := filepath.Join(e.sysData, "applications")
	user := filepath.Join(e.dataHome, "applications")
	writeFile(t, filepath.Join(sys, "a.desktop"), desktop("A", "a", "text/x-c;", ""))
	writeFile(t, filepath.Join(user, "a.desktop"), desktop("A", "a", "text/x-c;", "Hidden=true"))
	writeFile(t, filepath.Join(sys, "b.desktop"), desktop("B", "b", "text/x-c;", "TryExec=definitely-not-a-binary-xyz"))
	writeFile(t, filepath.Join(sys, "c.desktop"), desktop("C", "c", "text/x-c;", "TryExec=/bin/sh"))
	writeFile(t, filepath.Join(sys, "d.desktop"), desktop("D", "d", "text/x-c;", "NoDisplay=true"))
	writeFile(t, filepath.Join(sys, "e.desktop"), "[Desktop Entry]\nType=Link\nName=E\nExec=e\nMimeType=text/x-c;\n")

	if got, want := mustApps(t, "text/x-c"), []string{"c.desktop"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}

	// NoDisplay apps appear when explicitly associated.
	writeFile(t, filepath.Join(e.configHome, "mimeapps.list"), "[Added Associations]\ntext/x-c=d.desktop;\n")
	if got, want := mustApps(t, "text/x-c"), []string{"d.desktop", "c.desktop"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
}

func TestMimeappsOrdering(t *testing.T) {
	e := setupEnv(t)
	sys := filepath.Join(e.sysData, "applications")
	for _, n := range []string{"alpha", "beta", "gamma", "delta"} {
		writeFile(t, filepath.Join(sys, n+".desktop"), desktop(n, n+" %F", "application/pdf;", ""))
	}
	writeFile(t, filepath.Join(sys, "extra.desktop"), desktop("extra", "extra", "text/plain;", ""))
	writeFile(t, filepath.Join(e.configHome, "mimeapps.list"), `[Default Applications]
application/pdf=missing.desktop;gamma.desktop;

[Added Associations]
application/pdf=extra.desktop;delta.desktop;

[Removed Associations]
application/pdf=beta.desktop;
`)
	// Lower-precedence file: its default is overridden, its added entry for a
	// removed app is ignored.
	writeFile(t, filepath.Join(sys, "mimeapps.list"), `[Default Applications]
application/pdf=alpha.desktop

[Added Associations]
application/pdf=beta.desktop
`)
	want := []string{"gamma.desktop", "extra.desktop", "delta.desktop", "alpha.desktop"}
	if got := mustApps(t, "application/pdf"); !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	def, ok, err := apps.DefaultFor("application/pdf")
	if err != nil || !ok || def.ID != "gamma.desktop" {
		t.Fatalf("DefaultFor = %v %v %v", def.ID, ok, err)
	}
}

func TestWildcardAndTextTail(t *testing.T) {
	e := setupEnv(t)
	sys := filepath.Join(e.sysData, "applications")
	writeFile(t, filepath.Join(sys, "img.desktop"), desktop("Img", "img", "image/*;", ""))
	writeFile(t, filepath.Join(sys, "go.desktop"), desktop("Zed Go", "zgo", "text/x-go;", ""))
	writeFile(t, filepath.Join(sys, "edit.desktop"), desktop("Editor", "edit", "text/plain;", ""))

	if got, want := mustApps(t, "image/webp"), []string{"img.desktop"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("wildcard ids = %v, want %v", got, want)
	}
	if got, want := mustApps(t, "text/x-go"), []string{"go.desktop", "edit.desktop"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("text tail ids = %v, want %v", got, want)
	}
	if got := mustApps(t, "video/mp4"); len(got) != 0 {
		t.Fatalf("unexpected apps %v", got)
	}
	if _, err := apps.AppsFor("bogus"); err == nil {
		t.Fatal("expected error for invalid mime type")
	}
}

func TestCommands(t *testing.T) {
	files := []string{"/tmp/a b.txt", "/tmp/c.txt"}
	cases := []struct {
		name  string
		app   apps.App
		paths []string
		want  [][]string
	}{
		{"multi %F", apps.App{Exec: "editor --new %F"}, files,
			[][]string{{"editor", "--new", "/tmp/a b.txt", "/tmp/c.txt"}}},
		{"single %f splits", apps.App{Exec: "viewer %f"}, files,
			[][]string{{"viewer", "/tmp/a b.txt"}, {"viewer", "/tmp/c.txt"}}},
		{"%U uris", apps.App{Exec: "browser %U"}, files,
			[][]string{{"browser", "file:///tmp/a%20b.txt", "file:///tmp/c.txt"}}},
		{"%u single", apps.App{Exec: "browser %u"}, files[:1],
			[][]string{{"browser", "file:///tmp/a%20b.txt"}}},
		{"no codes appends", apps.App{Exec: "plain -x"}, files,
			[][]string{{"plain", "-x", "/tmp/a b.txt", "/tmp/c.txt"}}},
		{"literal percent", apps.App{Exec: "printf 100%% %f"}, files[1:],
			[][]string{{"printf", "100%", "/tmp/c.txt"}}},
		{"icon name file", apps.App{Exec: "app %i --title=%c %k %F", Icon: "ico", Name: "My App", File: "/d/app.desktop"}, files[1:],
			[][]string{{"app", "--icon", "ico", "--title=My App", "/d/app.desktop", "/tmp/c.txt"}}},
		{"no icon drops %i", apps.App{Exec: "app %i %f"}, files[1:],
			[][]string{{"app", "/tmp/c.txt"}}},
		{"deprecated dropped", apps.App{Exec: "old %d %D %n %N %v %m %f"}, files[1:],
			[][]string{{"old", "/tmp/c.txt"}}},
		{"quoting", apps.App{Exec: "sh -c \"echo \\\"hi\\\" \\$HOME \\\\ \\` done\" %f"}, files[1:],
			[][]string{{"sh", "-c", "echo \"hi\" $HOME \\ ` done", "/tmp/c.txt"}}},
		{"quoted program with space", apps.App{Exec: `"/opt/my app/bin" %F`}, nil,
			[][]string{{"/opt/my app/bin"}}},
		{"empty quoted arg kept", apps.App{Exec: `tool "" %f`}, files[1:],
			[][]string{{"tool", "", "/tmp/c.txt"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.app.Commands(tc.paths); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Commands = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExecEscapesFromDesktopFile(t *testing.T) {
	e := setupEnv(t)
	// The file-level \\ escape becomes \ before quote parsing.
	writeFile(t, filepath.Join(e.sysData, "applications", "q.desktop"),
		desktop("Q", `sh -c "echo \\"x y\\"" %f`, "text/x-q;", ""))
	list, err := apps.AppsFor("text/x-q")
	if err != nil || len(list) == 0 {
		t.Fatalf("AppsFor: %v %v", list, err)
	}
	name, args, err := list[0].Command([]string{"/f"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "sh" || !reflect.DeepEqual(args, []string{"-c", `echo "x y"`, "/f"}) {
		t.Fatalf("Command = %q %q", name, args)
	}
}

func TestCommandErrors(t *testing.T) {
	if _, _, err := (apps.App{Exec: `bad "unterminated`}).Command(nil); err == nil {
		t.Fatal("expected unterminated quote error")
	}
	if got := (apps.App{Exec: `bad "unterminated`}).Commands(nil); got != nil {
		t.Fatalf("Commands = %q, want nil", got)
	}
	_, _, err := (apps.App{Exec: "viewer %f"}).Command([]string{"a", "b"})
	if !errors.Is(err, apps.ErrMultipleLaunches) {
		t.Fatalf("err = %v, want ErrMultipleLaunches", err)
	}
	if _, _, err := (apps.App{Exec: ""}).Command(nil); err == nil {
		t.Fatal("expected empty exec error")
	}
	if err := (apps.App{Exec: "true", Terminal: true}).Start(nil); err == nil {
		t.Fatal("expected Start to refuse terminal apps")
	}
}

func TestStartDetached(t *testing.T) {
	out := filepath.Join(t.TempDir(), "ran")
	app := apps.App{Exec: `sh -c "touch \"$0\"" %f`}
	if err := app.Start([]string{out}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(out); err == nil {
			return
		}
		// Give the detached child a moment to run.
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("detached process did not run")
}

func TestMIMETypeFallback(t *testing.T) {
	saved := apps.XDGMimeCommand
	apps.XDGMimeCommand = ""
	t.Cleanup(func() { apps.XDGMimeCommand = saved })

	dir := t.TempDir()
	png := filepath.Join(dir, "pic.png")
	writeFile(t, png, "not really a png")
	sniff := filepath.Join(dir, "noext")
	writeFile(t, sniff, "%PDF-1.4 hello")

	cases := map[string]string{png: "image/png", sniff: "application/pdf", dir: "inode/directory"}
	for path, want := range cases {
		got, err := apps.MIMEType(context.Background(), path)
		if err != nil || got != want {
			t.Errorf("MIMEType(%s) = %q, %v; want %q", path, got, err, want)
		}
		if got, err := apps.FallbackMIMEType(path); err != nil || got != want {
			t.Errorf("FallbackMIMEType(%s) = %q, %v; want %q", path, got, err, want)
		}
	}
	if _, err := apps.MIMEType(context.Background(), filepath.Join(dir, "missing")); err == nil {
		t.Error("expected error for missing file")
	}
}
