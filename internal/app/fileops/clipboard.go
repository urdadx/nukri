package fileops

import (
	"slices"

	fsfileops "github.com/urdadx/nukri/internal/fs/fileops"
)

// Clipboard holds the items copied or cut for the next paste.
type Clipboard struct {
	Sources   []string
	Operation fsfileops.Operation
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

func (c *Clipboard) Clear() {
	*c = Clipboard{}
}

// ClearIf clears the clipboard only if it still holds sources for operation,
// so a paste that finishes late doesn't drop a newer copy or cut.
func (c *Clipboard) ClearIf(sources []string, operation fsfileops.Operation) {
	if c.Operation == operation && slices.Equal(c.Sources, sources) {
		*c = Clipboard{}
	}
}
