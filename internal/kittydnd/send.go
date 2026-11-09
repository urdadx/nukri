package kittydnd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// dragEntry is a dragged path whose contents the terminal asked for.
type dragEntry struct {
	index int // 1-based, in the drag's URI list
	path  string
}

// maxSendBatch is how much protocol data is written to the terminal at once.
// Every write holds whole sequences, so screen updates never land inside one.
const maxSendBatch = 64 << 10

/*
SendDragEntry sends the terminal the contents of a dragged path, which it
asked for (a DragEntryRequested event) because the drop target is on another
machine. index is the path's 1-based position in the drag. Entries are sent
in the order asked by a goroutine, so a large file doesn't hold up the caller.
Folders are sent with everything in them.
*/
func (s *Session) SendDragEntry(index int, path string) {
	if s.terminal == nil {
		return
	}
	s.sendOnce.Do(func() {
		s.sends = make(chan dragEntry, 1024)
		go s.sendLoop()
	})
	s.sends <- dragEntry{index: index, path: path}
}

func (s *Session) sendLoop() {
	for entry := range s.sends {
		batch := &sequenceBatch{terminal: s.terminal}
		err := s.sendEntry(batch, entry)
		if err != nil {
			// The protocol has no per-entry error for drags: abort the drag.
			batch.add(fmt.Sprintf("\x1b]72;t=E;EIO:%s\x1b\\", safeText(err.Error())))
		}
		_ = batch.flush()
	}
}

// sendEntry sends a dragged path and, for a folder, everything in it, breadth
// first. Each folder gets a handle its children are sent under.
func (s *Session) sendEntry(batch *sequenceBatch, entry dragEntry) error {
	type pending struct {
		handle int
		path   string
		names  []string
	}
	top := fmt.Sprintf("t=k:x=%d", entry.index)
	handle, names, err := s.sendPath(batch, entry.path, top)
	if err != nil {
		return err
	}
	var queue []pending
	if handle != 0 {
		queue = append(queue, pending{handle, entry.path, names})
	}
	for len(queue) > 0 {
		folder := queue[0]
		queue = queue[1:]
		for number, name := range folder.names {
			metadata := fmt.Sprintf("t=k:x=%d:Y=%d:y=%d", entry.index, folder.handle, number+1)
			path := filepath.Join(folder.path, name)
			handle, names, err := s.sendPath(batch, path, metadata)
			if err != nil {
				return err
			}
			if handle != 0 {
				queue = append(queue, pending{handle, path, names})
			}
		}
	}
	return batch.flush()
}

/*
sendPath sends one file, symlink or folder listing under metadata, marked
with X: 0 for a file, 1 for a symlink, a new handle for a folder. For a folder
it returns the handle and the names listed, whose contents the caller sends.
*/
func (s *Session) sendPath(batch *sequenceBatch, path, metadata string) (int, []string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, nil, err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return 0, nil, err
		}
		batch.add(entrySequences(metadata+":X=1", []byte(target)))
		return 0, nil, nil
	case info.IsDir():
		entries, err := os.ReadDir(path)
		if err != nil {
			return 0, nil, err
		}
		var names []string
		for _, entry := range entries {
			// Only files, folders and symlinks may be sent.
			if mode := entry.Type(); mode.IsRegular() || mode.IsDir() || mode&os.ModeSymlink != 0 {
				names = append(names, entry.Name())
			}
		}
		s.handles++
		handle := s.handles + 1 // handles 0 and 1 mean file and symlink
		batch.add(entrySequences(fmt.Sprintf("%s:X=%d", metadata, handle), []byte(strings.Join(names, "\x00"))))
		return handle, names, nil
	case info.Mode().IsRegular():
		return 0, nil, s.sendFile(batch, path, metadata+":X=0")
	default:
		return 0, nil, fmt.Errorf("%s is not a file, folder or symlink", filepath.Base(path))
	}
}

// sendFile streams a file in chunks: its data, then an empty last chunk.
func (s *Session) sendFile(batch *sequenceBatch, path, metadata string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	block := make([]byte, chunkSize/4*3) // encodes to one chunk
	first := true
	for {
		n, err := io.ReadFull(file, block)
		if n > 0 {
			prefix := "m=1"
			if first {
				prefix, first = metadata+":m=1", false
			}
			batch.add("\x1b]72;" + prefix + ";" + base64.StdEncoding.EncodeToString(block[:n]) + "\x1b\\")
			if err := batch.maybeFlush(); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	batch.add("\x1b]72;" + metadata + ":m=0;\x1b\\")
	return nil
}

// entrySequences sends an entry's data under metadata: chunks marked m=1,
// then an empty chunk with m=0 to end it.
func entrySequences(metadata string, data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var result strings.Builder
	for start := 0; start < len(encoded); start += chunkSize {
		prefix := "m=1"
		if start == 0 {
			prefix = metadata + ":m=1"
		}
		fmt.Fprintf(&result, "\x1b]72;%s;%s\x1b\\", prefix, encoded[start:min(start+chunkSize, len(encoded))])
	}
	fmt.Fprintf(&result, "\x1b]72;%s:m=0;\x1b\\", metadata)
	return result.String()
}

// sequenceBatch gathers whole sequences and writes them in large writes.
type sequenceBatch struct {
	terminal io.Writer
	data     []byte
}

func (b *sequenceBatch) add(sequence string) { b.data = append(b.data, sequence...) }

func (b *sequenceBatch) maybeFlush() error {
	if len(b.data) < maxSendBatch {
		return nil
	}
	return b.flush()
}

func (b *sequenceBatch) flush() error {
	if len(b.data) == 0 {
		return nil
	}
	_, err := b.terminal.Write(b.data)
	b.data = b.data[:0]
	return err
}

// safeText keeps an error description to printable ASCII without the
// protocol's separators.
func safeText(text string) string {
	return strings.Map(func(r rune) rune {
		if r < ' ' || r > '~' || r == ';' {
			return ' '
		}
		return r
	}, text)
}
