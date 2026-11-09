package test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/urdadx/nukri/internal/kittydnd"
)

// sequences splits terminal output into the metadata and payload of each
// OSC 72 sequence.
func sequences(data []byte) [][2]string {
	var result [][2]string
	for _, part := range strings.Split(string(data), "\x1b\\") {
		body, found := strings.CutPrefix(part, "\x1b]72;")
		if !found {
			continue
		}
		metadata, payload, _ := strings.Cut(body, ";")
		result = append(result, [2]string{metadata, payload})
	}
	return result
}

// keys parses sequence metadata into its keys.
func keys(metadata string) map[string]string {
	result := map[string]string{}
	for _, field := range strings.Split(metadata, ":") {
		if key, value, found := strings.Cut(field, "="); found {
			result[key] = value
		}
	}
	return result
}

/*
fakeTerminal plays a terminal on another machine than nukri, as over SSH: it
answers requests for dropped files by reading them from its own disk. Replies
go out from a goroutine, as they would from a real terminal.
*/
type fakeTerminal struct {
	t         *testing.T
	uris      []string // the drop's paths, by 1-based URI index
	denied    map[string]bool
	chunk     int  // encoded bytes per chunk
	strayEnds bool // send a second end marker after each entry

	mu      sync.Mutex
	handles map[int]string
	names   map[int][]string
	closed  map[int]bool
	replies chan []byte
}

func newFakeTerminal(t *testing.T, input *io.PipeWriter, uris ...string) *fakeTerminal {
	f := &fakeTerminal{t: t, uris: uris, denied: map[string]bool{}, chunk: 8,
		handles: map[int]string{}, names: map[int][]string{}, closed: map[int]bool{}, replies: make(chan []byte, 1024)}
	go func() {
		for reply := range f.replies {
			if _, err := input.Write(reply); err != nil {
				return
			}
		}
	}()
	return f
}

func (f *fakeTerminal) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, sequence := range sequences(p) {
		k := keys(sequence[0])
		if k["t"] != "r" {
			continue
		}
		x, _ := strconv.Atoi(k["x"])
		switch {
		case k["y"] != "":
			index, _ := strconv.Atoi(k["y"])
			f.serve(f.uris[index-1], fmt.Sprintf("t=r:x=%d:y=%d", x, index))
		case k["Y"] != "" && k["x"] != "":
			handle, _ := strconv.Atoi(k["Y"])
			f.serve(filepath.Join(f.handles[handle], f.names[handle][x-1]), fmt.Sprintf("t=r:Y=%d:x=%d", handle, x))
		case k["Y"] != "":
			handle, _ := strconv.Atoi(k["Y"])
			f.closed[handle] = true
		}
	}
	return len(p), nil
}

// serve replies with a file, symlink or folder listing.
func (f *fakeTerminal) serve(path, metadata string) {
	if f.denied[path] {
		f.replies <- []byte("\x1b]72;t=R:" + strings.TrimPrefix(metadata, "t=r:") + ";EPERM:Permission denied\x1b\\")
		return
	}
	info, err := os.Lstat(path)
	if err != nil {
		f.t.Errorf("terminal asked for missing %s", path)
		return
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, _ := os.Readlink(path)
		f.send(metadata+":X=1", []byte(target))
	case info.IsDir():
		entries, _ := os.ReadDir(path)
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		handle := len(f.handles) + 2
		f.handles[handle], f.names[handle] = path, names
		f.send(fmt.Sprintf("%s:X=%d", metadata, handle), []byte(strings.Join(names, "\x00")))
	default:
		data, _ := os.ReadFile(path)
		f.send(metadata+":X=0", data)
	}
}

// send chunks data: metadata on the first chunk only, then an empty end.
func (f *fakeTerminal) send(metadata string, data []byte) {
	encoded := base64.StdEncoding.EncodeToString(data)
	var reply bytes.Buffer
	for start := 0; start < len(encoded); start += f.chunk {
		prefix := "m=1"
		if start == 0 {
			prefix = metadata + ":m=1"
		}
		fmt.Fprintf(&reply, "\x1b]72;%s;%s\x1b\\", prefix, encoded[start:min(start+f.chunk, len(encoded))])
	}
	end := "m=0"
	if len(encoded) == 0 {
		end = metadata + ":m=0"
	}
	fmt.Fprintf(&reply, "\x1b]72;%s;\x1b\\", end)
	if f.strayEnds {
		fmt.Fprintf(&reply, "\x1b]72;%s:m=0;\x1b\\", metadata)
	}
	f.replies <- reply.Bytes()
}

// waitFor returns the next event of kind.
func waitFor(t *testing.T, events <-chan kittydnd.Event, kind kittydnd.EventKind) kittydnd.Event {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case event := <-events:
			if event.Kind == kind {
				return event
			}
		case <-timeout:
			t.Fatalf("no event of kind %d", kind)
		}
	}
}

// makeTree builds a folder with a nested folder, a file spanning many chunks,
// an empty file and a symlink, and returns its path.
func makeTree(t *testing.T, parent string) string {
	t.Helper()
	root := filepath.Join(parent, "docs")
	must(t, os.MkdirAll(filepath.Join(root, "sub", "deeper"), 0o755))
	large := bytes.Repeat([]byte("0123456789abcdefghij"), 600)
	must(t, os.WriteFile(filepath.Join(root, "large.bin"), large, 0o644))
	must(t, os.WriteFile(filepath.Join(root, "empty"), nil, 0o644))
	must(t, os.WriteFile(filepath.Join(root, "sub", "a b.txt"), []byte("in sub"), 0o644))
	must(t, os.WriteFile(filepath.Join(root, "sub", "deeper", "c"), []byte("deep"), 0o644))
	must(t, os.Symlink("large.bin", filepath.Join(root, "link")))
	return root
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// describe lists a tree: each path with its file size and hash, or symlink
// target.
func describe(t *testing.T, root string) []string {
	t.Helper()
	var result []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, _ := os.Readlink(path)
			result = append(result, relative+" -> "+target)
		case info.IsDir():
			result = append(result, relative+"/")
		default:
			data, _ := os.ReadFile(path)
			result = append(result, fmt.Sprintf("%s %d bytes %x", relative, len(data), sha256.Sum256(data)))
		}
		return nil
	})
	must(t, err)
	sort.Strings(result)
	return result
}

func fetchRemote(t *testing.T, strayEnds bool) {
	local := t.TempDir() // the terminal's machine
	tree := makeTree(t, local)
	single := filepath.Join(local, "notes.txt")
	must(t, os.WriteFile(single, []byte("notes"), 0o644))
	secret := filepath.Join(local, "secret")
	must(t, os.WriteFile(secret, []byte("no"), 0o600))

	destination := t.TempDir()
	must(t, os.WriteFile(filepath.Join(destination, "notes.txt"), []byte("already here"), 0o644))

	reader, writer := io.Pipe()
	terminal := newFakeTerminal(t, writer, "", tree, single, secret)
	terminal.denied[secret] = true
	terminal.strayEnds = strayEnds
	events := make(chan kittydnd.Event, 8)
	session := kittydnd.NewSession(terminal, events)
	go session.Filter(reader, io.Discard)
	t.Cleanup(func() { writer.Close() })

	err := session.FetchRemoteDrop(kittydnd.RemoteDrop{MIME: 1, Destination: destination, Entries: []kittydnd.DropEntry{
		{Index: 2, Path: tree}, {Index: 3, Path: single}, {Index: 4, Path: secret},
	}})
	must(t, err)
	done := waitFor(t, events, kittydnd.RemoteDropDone)

	want := []string{filepath.Join(destination, "docs"), filepath.Join(destination, "notes_1.txt")}
	if done.Completed != 2 || strings.Join(done.Paths, ",") != strings.Join(want, ",") {
		t.Fatalf("done = %d %v, want 2 %v", done.Completed, done.Paths, want)
	}
	if !strings.Contains(done.Error, "secret: Permission denied") {
		t.Fatalf("error = %q, want the denied file reported", done.Error)
	}
	if got, expected := describe(t, filepath.Join(destination, "docs")), describe(t, tree); strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("received tree:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(expected, "\n"))
	}
	if data, _ := os.ReadFile(filepath.Join(destination, "notes_1.txt")); string(data) != "notes" {
		t.Fatalf("notes_1.txt = %q", data)
	}
	if _, err := os.Lstat(filepath.Join(destination, "secret")); err == nil {
		t.Fatal("denied file was created")
	}
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	for handle := range terminal.handles {
		if !terminal.closed[handle] {
			t.Errorf("folder handle %d was never released", handle)
		}
	}
}

func TestFetchRemoteDropCopiesTree(t *testing.T) { fetchRemote(t, false) }

// A terminal that ends an entry twice must not confuse the next request.
func TestFetchRemoteDropIgnoresStrayEndMarkers(t *testing.T) { fetchRemote(t, true) }

func TestFetchRemoteDropNeedsEntries(t *testing.T) {
	session := kittydnd.NewSession(io.Discard, make(chan kittydnd.Event, 1))
	if err := session.FetchRemoteDrop(kittydnd.RemoteDrop{MIME: 1, Destination: t.TempDir()}); err == nil {
		t.Fatal("an empty remote drop was started")
	}
}

// lockedBuffer collects what the session writes from its goroutine.
type lockedBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}

func (b *lockedBuffer) bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.data.Bytes()...)
}

/*
rebuild plays the terminal receiving dragged entries: it rebuilds every
entry sent for drag index into destination and returns the top path. Chunks
after the first carry only m, so they belong to the entry last started.
*/
func rebuild(t *testing.T, output []byte, index int, destination string) string {
	t.Helper()
	type entry struct {
		meta map[string]string
		data []byte
	}
	var entries []*entry
	var current *entry
	encoded := ""
	for _, sequence := range sequences(output) {
		k := keys(sequence[0])
		if k["t"] == "k" && current == nil {
			current, encoded = &entry{meta: k}, ""
		}
		encoded += sequence[1]
		if k["m"] == "0" {
			data, err := base64.StdEncoding.DecodeString(encoded)
			must(t, err)
			current.data = data
			entries = append(entries, current)
			current = nil
		}
	}
	folders := map[string]string{} // handle -> path
	names := map[string][]string{}
	top := ""
	for _, e := range entries {
		if e.meta["x"] != strconv.Itoa(index) {
			continue
		}
		var path string
		if handle, child := e.meta["Y"]; child {
			number, _ := strconv.Atoi(e.meta["y"])
			path = filepath.Join(folders[handle], names[handle][number-1])
		} else {
			path = filepath.Join(destination, fmt.Sprintf("entry%d", index))
			top = path
		}
		switch e.meta["X"] {
		case "0":
			must(t, os.WriteFile(path, e.data, 0o644))
		case "1":
			must(t, os.Symlink(string(e.data), path))
		default:
			must(t, os.Mkdir(path, 0o755))
			folders[e.meta["X"]] = path
			if len(e.data) > 0 {
				names[e.meta["X"]] = strings.Split(string(e.data), "\x00")
			}
		}
	}
	return top
}

func TestSendDragEntrySendsTree(t *testing.T) {
	local := t.TempDir()
	tree := makeTree(t, local)
	single := filepath.Join(local, "notes.txt")
	must(t, os.WriteFile(single, []byte("notes"), 0o644))

	var output lockedBuffer
	session := kittydnd.NewSession(&output, make(chan kittydnd.Event, 1))
	session.SendDragEntry(1, tree)
	session.SendDragEntry(2, single)
	// Entries go out in order, so the second one ending means both are sent.
	deadline := time.Now().Add(10 * time.Second)
	for !bytes.Contains(output.bytes(), []byte("\x1b]72;t=k:x=2:X=0:m=0;\x1b\\")) {
		if time.Now().After(deadline) {
			t.Fatal("dragged entries were not sent")
		}
		time.Sleep(10 * time.Millisecond)
	}

	received := t.TempDir()
	top := rebuild(t, output.bytes(), 1, received)
	if got, want := describe(t, top), describe(t, tree); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("sent tree:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if data, _ := os.ReadFile(rebuild(t, output.bytes(), 2, received)); string(data) != "notes" {
		t.Fatalf("sent file = %q", data)
	}
}
