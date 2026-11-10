package fileops

import (
	"slices"

	fsfileops "github.com/urdadx/nukri/internal/fs/fileops"
)

// Clipboard holds the items copied or cut for the next paste.
type Clipboard struct {
	Sources   []string
	Operation fsfileops.Operation
	pasting   bool // a paste is running; see BeginPaste
}

func (c *Clipboard) Copy(sources []string) {
	c.Sources = slices.Clone(sources)
	c.Operation = fsfileops.Copy
}

func (c *Clipboard) Cut(sources []string) {
	c.Sources = slices.Clone(sources)
	c.Operation = fsfileops.Move
}

func (c Clipboard) Empty() bool {
	return len(c.Sources) == 0
}

// Contains reports whether path is waiting to be pasted.
func (c Clipboard) Contains(path string) bool {
	return slices.Contains(c.Sources, path)
}

// Clear empties the clipboard. A paste already running still finishes.
func (c *Clipboard) Clear() {
	c.Sources, c.Operation = nil, 0
}

// BeginPaste marks a paste as running and reports whether one may start: only
// one runs at a time, so pressing paste twice doesn't paste everything twice.
func (c *Clipboard) BeginPaste() bool {
	if c.pasting {
		return false
	}
	c.pasting = true
	return true
}

/*
Pasted ends the paste of sources for operation. Copied items stay on the
clipboard, so they can be pasted into more folders. After a cut, the items
that failed to move stay so the move can be retried, and the clipboard is
emptied when none failed. A newer copy or cut made while the paste ran is
left alone.
*/
func (c *Clipboard) Pasted(sources []string, operation fsfileops.Operation, failed []string) {
	c.pasting = false
	if operation == fsfileops.Copy || c.Operation != operation || !slices.Equal(c.Sources, sources) {
		return
	}
	if len(failed) == 0 {
		c.Clear()
		return
	}
	c.Sources = slices.Clone(failed)
}
