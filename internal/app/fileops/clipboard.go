package fileops

import fsfileops "github.com/urdadx/nukri/internal/fs/fileops"

type Clipboard struct {
	Source    string
	Operation fsfileops.Operation
}

func (c *Clipboard) Copy(source string) {
	c.Source = source
	c.Operation = fsfileops.Copy
}

func (c *Clipboard) Cut(source string) {
	c.Source = source
	c.Operation = fsfileops.Move
}

func (c Clipboard) Empty() bool {
	return c.Source == ""
}

func (c *Clipboard) Clear() {
	*c = Clipboard{}
}

func (c *Clipboard) ClearIf(source string, operation fsfileops.Operation) {
	if c.Source == source && c.Operation == operation {
		*c = Clipboard{}
	}
}
