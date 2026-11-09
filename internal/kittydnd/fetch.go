package kittydnd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urdadx/nukri/internal/fs/fileops"
)

// RemoteDrop asks for the contents of files dropped from another machine.
type RemoteDrop struct {
	MIME        int         // index of text/uri-list in the drop, from DropData
	Entries     []DropEntry // from DropData
	Destination string      // directory the files are written into
}

// fetchIdleTimeout ends a remote drop the terminal stops answering.
const fetchIdleTimeout = 30 * time.Second

/*
FetchRemoteDrop starts copying the files of a drop from another machine into
drop.Destination, asking the terminal for their contents. It returns at once;
a RemoteDropDone event reports the result. Folders are copied with everything
in them; symlinks are recreated. One remote drop runs at a time.
*/
func (s *Session) FetchRemoteDrop(drop RemoteDrop) error {
	if len(drop.Entries) == 0 {
		return errors.New("drop contains no files")
	}
	if s.terminal == nil {
		return errors.New("no terminal to fetch files through")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fetch != nil {
		return errors.New("another drop is still being received")
	}
	f := &fetch{session: s, drop: drop}
	s.fetch = f
	f.timer = time.AfterFunc(fetchIdleTimeout, f.timeout)
	f.next()
	return nil
}

/*
fetch is a remote drop being received. Requests go out one at a time: each
top-level entry, then the children of every folder received, breadth first.
Its fields are guarded by the session's mutex.
*/
type fetch struct {
	session *Session
	drop    RemoteDrop
	timer   *time.Timer

	entry   int        // next top-level entry to request
	folders []folder   // folders whose children are still to be requested
	current *receiving // entry whose reply is awaited, or nil
	created []string   // top-level paths written
	errs    []error
}

// folder is a received folder: the terminal's handle for it, where it was
// created, and the names of its children.
type folder struct {
	handle int
	path   string
	names  []string
	next   int // index of the next child to request
}

type entryKind uint8

const (
	fileEntry entryKind = iota
	symlinkEntry
	folderEntry
)

// receiving is an entry being received.
type receiving struct {
	path     string
	topLevel bool
	started  bool // its first reply arrived
	kind     entryKind
	handle   int      // folderEntry: the terminal's handle for it
	file     *os.File // fileEntry: written as chunks arrive
	data     []byte   // symlink target or folder listing
	carry    string   // base64 not yet decoded
	err      error

	// The keys its replies carry: y for a top-level entry, Y and x for a child.
	subIndex, handleKey, number int
}

// next requests the next entry, or finishes when none is left.
func (f *fetch) next() {
	for {
		if f.entry < len(f.drop.Entries) {
			entry := f.drop.Entries[f.entry]
			f.entry++
			name := filepath.Base(entry.Path)
			if !validName(name) {
				f.errs = append(f.errs, fmt.Errorf("%q: invalid name", name))
				continue
			}
			path := fileops.AvailablePath(filepath.Join(f.drop.Destination, name))
			if f.request(&receiving{path: path, topLevel: true, subIndex: entry.Index}, fmt.Sprintf("t=r:x=%d:y=%d", f.drop.MIME, entry.Index)) {
				return
			}
			continue
		}
		if len(f.folders) > 0 {
			folder := &f.folders[0]
			if folder.next < len(folder.names) {
				name := folder.names[folder.next]
				folder.next++
				if !validName(name) {
					f.errs = append(f.errs, fmt.Errorf("%q: invalid name", name))
					continue
				}
				child := &receiving{path: filepath.Join(folder.path, name), handleKey: folder.handle, number: folder.next}
				if f.request(child, fmt.Sprintf("t=r:Y=%d:x=%d", folder.handle, folder.next)) {
					return
				}
				continue
			}
			// Done with this folder: let the terminal free its handle.
			_ = f.session.writeSequence(fmt.Sprintf("t=r:Y=%d", folder.handle))
			f.folders = f.folders[1:]
			continue
		}
		f.finish()
		return
	}
}

// request asks the terminal for an entry and reports whether it was sent.
func (f *fetch) request(entry *receiving, metadata string) bool {
	if err := f.session.writeSequence(metadata); err != nil {
		f.errs = append(f.errs, err)
		return false
	}
	f.current = entry
	return true
}

/*
receive hands a sequence to the remote drop in progress if it is a reply to
its request: the first chunk of the reply or an error carries the request's
keys, later chunks only m. A reply naming another entry, such as a stray end
marker, is dropped. It reports whether the sequence was taken.
*/
func (s *Session) receive(sequence []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.fetch
	if f == nil || f.current == nil {
		return false
	}
	metadata, payload := splitSequence(sequence)
	fields := parseFields(metadata)
	keyed := fields.y >= 1 || fields.upperY >= 2
	switch {
	case fields.typeCode == 'R' || fields.typeCode == 'r' && keyed:
		if keyed && !f.current.answeredBy(fields) {
			return true
		}
	case f.current.started && (fields.typeCode == 0 || fields.typeCode == 'r'):
	default:
		return false
	}
	f.timer.Reset(fetchIdleTimeout)
	f.reply(fields, payload)
	return true
}

// answeredBy reports whether a reply's keys name this entry.
func (r *receiving) answeredBy(fields fields) bool {
	if r.topLevel {
		return fields.upperY < 2 && fields.y == r.subIndex
	}
	return fields.upperY == r.handleKey && fields.x == r.number
}

// reply takes one reply sequence for the current entry.
func (f *fetch) reply(fields fields, payload string) {
	entry := f.current
	if fields.typeCode == 'R' {
		f.current = nil
		entry.discard()
		f.errs = append(f.errs, fmt.Errorf("%s: %s", filepath.Base(entry.path), errorText(payload)))
		if fields.y < 1 && fields.upperY < 2 {
			f.finish() // an error for no particular entry ends the drop
			return
		}
		f.next()
		return
	}
	if !entry.started {
		entry.start(fields.upperX)
	}
	last := fields.more == nil || !*fields.more
	entry.add(payload, last)
	if !last {
		return
	}
	f.current = nil
	if err := entry.complete(); err != nil {
		f.errs = append(f.errs, fmt.Errorf("%s: %w", filepath.Base(entry.path), err))
		if entry.kind == folderEntry {
			_ = f.session.writeSequence(fmt.Sprintf("t=r:Y=%d", entry.handle))
		}
	} else {
		if entry.kind == folderEntry {
			f.folders = append(f.folders, folder{handle: entry.handle, path: entry.path, names: listing(entry.data)})
		}
		if entry.topLevel {
			f.created = append(f.created, entry.path)
		}
	}
	f.next()
}

// timeout ends the drop when the terminal has stopped answering.
func (f *fetch) timeout() {
	f.session.mu.Lock()
	defer f.session.mu.Unlock()
	if f.session.fetch != f {
		return
	}
	if f.current != nil {
		f.current.discard()
		f.current = nil
	}
	f.errs = append(f.errs, errors.New("the terminal stopped sending the dropped files"))
	f.finish()
}

// finish ends the drop and reports it with a RemoteDropDone event.
func (f *fetch) finish() {
	f.timer.Stop()
	f.session.fetch = nil
	event := Event{Kind: RemoteDropDone, Paths: f.created, Completed: len(f.created)}
	if err := errors.Join(f.errs...); err != nil {
		event.Error = err.Error()
	}
	// Sent from a goroutine: finish may run on the caller of FetchRemoteDrop,
	// which may be the one draining events.
	events := f.session.events
	go func() { events <- event }()
}

// start sets up an entry when its first reply arrives. The terminal marks
// folders with their handle in X, symlinks with X=1 and files with X=0.
func (r *receiving) start(kind int) {
	r.started = true
	switch {
	case kind >= 2:
		r.kind, r.handle = folderEntry, kind
	case kind == 1:
		r.kind = symlinkEntry
	default:
		r.kind = fileEntry
		r.file, r.err = os.OpenFile(r.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	}
}

// add decodes a chunk of the entry's base64 data and stores it. Chunks are
// normally split on 4-byte boundaries; what isn't is kept for the next chunk.
func (r *receiving) add(payload string, last bool) {
	if r.err != nil {
		return
	}
	data := r.carry + payload
	r.carry = ""
	var decoded []byte
	if last || strings.Contains(data, "=") {
		decoded, r.err = decodeBase64(data)
	} else {
		whole := len(data) / 4 * 4
		r.carry = data[whole:]
		decoded, r.err = base64.RawStdEncoding.DecodeString(data[:whole])
	}
	if r.err != nil {
		return
	}
	if r.kind == fileEntry {
		_, r.err = r.file.Write(decoded)
		return
	}
	if len(r.data)+len(decoded) > maxDropBytes {
		r.err = errors.New("too large")
		return
	}
	r.data = append(r.data, decoded...)
}

// complete writes out a fully received entry.
func (r *receiving) complete() error {
	switch r.kind {
	case fileEntry:
		if r.file != nil {
			if err := r.file.Close(); r.err == nil {
				r.err = err
			}
		}
		if r.err != nil {
			r.discard()
		}
		return r.err
	case symlinkEntry:
		if r.err != nil {
			return r.err
		}
		return os.Symlink(string(r.data), r.path)
	default:
		if r.err != nil {
			return r.err
		}
		return os.Mkdir(r.path, 0o755)
	}
}

// discard removes a partly written file.
func (r *receiving) discard() {
	if r.file != nil {
		_ = r.file.Close()
		_ = os.Remove(r.path)
		r.file = nil
	}
}

func decodeBase64(data string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(data); err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(strings.TrimRight(data, "="))
}

// listing splits a folder listing: names separated by null bytes.
func listing(data []byte) []string {
	var names []string
	for _, name := range strings.Split(string(data), "\x00") {
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// validName reports whether name is a plain file name, safe to create in the
// destination: no path separator or null byte, and not . or ..
func validName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\x00")
}

// errorText turns an error reply, "ENAME:description", into readable text.
func errorText(payload string) string {
	name, description, _ := strings.Cut(payload, ":")
	if description != "" {
		return description
	}
	switch name {
	case "EPERM", "EACCES":
		return "permission denied"
	case "ENOENT":
		return "no such file"
	case "EIO":
		return "input/output error"
	case "":
		return "unknown error"
	}
	return name
}
