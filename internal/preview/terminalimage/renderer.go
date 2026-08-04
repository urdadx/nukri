package terminalimage

import (
	"io"

	"github.com/urdadx/nukri/internal/preview"
)

// NewRenderer selects the best supported image protocol. Kitty is preferred
// because it supports persistent image data and native placement deletion.
func NewRenderer(writer io.Writer) ImageRenderer {
	if IsKittyTerminal() {
		return NewKittyWithSupport(writer, true)
	}
	if IsSixelTerminal() {
		return NewSixelWithSupport(writer, true)
	}
	return &unsupportedRenderer{}
}

type unsupportedRenderer struct{}

func (*unsupportedRenderer) Supported() bool { return false }

func (*unsupportedRenderer) Place(_ preview.Image, _ Placement) (RenderedImage, error) {
	return RenderedImage{}, ErrUnsupportedTerminal
}

func (*unsupportedRenderer) DeletePlacement(RenderedImage) error { return ErrUnsupportedTerminal }
func (*unsupportedRenderer) Delete(RenderedImage) error          { return ErrUnsupportedTerminal }
func (*unsupportedRenderer) DeleteAll() error                    { return ErrUnsupportedTerminal }
