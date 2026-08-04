package terminalimage

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/urdadx/nukri/internal/preview"
)

var (
	ErrUnsupportedTerminal = errors.New("terminal does not support Kitty graphics")
	ErrInvalidImage        = errors.New("invalid preview image")
	ErrInvalidPlacement    = errors.New("invalid image placement")
)

const kittyChunkSize = 4096

type Placement struct {
	X       int
	Y       int
	Columns int
	Rows    int
	ZIndex  int
}

type RenderedImage struct {
	ImageID     uint32
	PlacementID uint32
	X           int
	Y           int
	Columns     int
	Rows        int
}

type ImageRenderer interface {
	Supported() bool
	Place(preview.Image, Placement) (RenderedImage, error)
	DeletePlacement(RenderedImage) error
	Delete(RenderedImage) error
	DeleteAll() error
}

type Kitty struct {
	writer    io.Writer
	supported bool
	mu        sync.Mutex
	nextID    uint32
}

func NewKitty(writer io.Writer) *Kitty {
	return NewKittyWithSupport(writer, IsKittyTerminal())
}

func NewKittyWithSupport(writer io.Writer, supported bool) *Kitty {
	return &Kitty{writer: writer, supported: supported, nextID: 1}
}

func IsKittyTerminal() bool {
	return os.Getenv("KITTY_WINDOW_ID") != "" || strings.EqualFold(os.Getenv("TERM"), "xterm-kitty")
}

func (k *Kitty) Supported() bool {
	return k != nil && k.supported && k.writer != nil
}

func (k *Kitty) Place(image preview.Image, placement Placement) (RenderedImage, error) {
	if !k.Supported() {
		return RenderedImage{}, ErrUnsupportedTerminal
	}
	if image.MediaType != "image/png" || len(image.Data) == 0 {
		return RenderedImage{}, ErrInvalidImage
	}
	if placement.X < 0 || placement.Y < 0 || placement.Columns <= 0 || placement.Rows <= 0 {
		return RenderedImage{}, ErrInvalidPlacement
	}

	k.mu.Lock()
	defer k.mu.Unlock()
	imageID := k.allocateID()
	placementID := k.allocateID()
	if err := k.transmitPNG(imageID, image.Data); err != nil {
		return RenderedImage{}, err
	}
	if err := k.place(imageID, placementID, placement); err != nil {
		_ = k.deleteImage(imageID)
		return RenderedImage{}, err
	}
	return RenderedImage{
		ImageID: imageID, PlacementID: placementID,
		X: placement.X, Y: placement.Y, Columns: placement.Columns, Rows: placement.Rows,
	}, nil
}

func (k *Kitty) Delete(image RenderedImage) error {
	if !k.Supported() {
		return ErrUnsupportedTerminal
	}
	if image.ImageID == 0 {
		return nil
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.deleteImage(image.ImageID)
}

func (k *Kitty) DeletePlacement(image RenderedImage) error {
	if !k.Supported() {
		return ErrUnsupportedTerminal
	}
	if image.ImageID == 0 || image.PlacementID == 0 {
		return nil
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	_, err := io.WriteString(k.writer, kittyCommand(
		fmt.Sprintf("a=d,d=p,i=%d,p=%d,q=2", image.ImageID, image.PlacementID), "",
	))
	return err
}

func (k *Kitty) DeleteAll() error {
	if !k.Supported() {
		return ErrUnsupportedTerminal
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	_, err := io.WriteString(k.writer, kittyCommand("a=d,d=A,q=2", ""))
	return err
}

func (k *Kitty) allocateID() uint32 {
	id := k.nextID
	k.nextID++
	if k.nextID == 0 {
		k.nextID = 1
	}
	return id
}

func (k *Kitty) transmitPNG(imageID uint32, data []byte) error {
	encoded := base64.StdEncoding.EncodeToString(data)
	for offset := 0; offset < len(encoded); offset += kittyChunkSize {
		end := min(offset+kittyChunkSize, len(encoded))
		more := end < len(encoded)
		control := "m=0"
		if more {
			control = "m=1"
		}
		if offset == 0 {
			control = fmt.Sprintf("a=t,f=100,i=%d,q=2,t=d,%s", imageID, control)
		}
		if _, err := io.WriteString(k.writer, kittyCommand(control, encoded[offset:end])); err != nil {
			return fmt.Errorf("transmit Kitty image: %w", err)
		}
	}
	return nil
}

func (k *Kitty) place(imageID, placementID uint32, placement Placement) error {
	control := fmt.Sprintf(
		"a=p,c=%d,i=%d,p=%d,q=2,r=%d,z=%d",
		placement.Columns, imageID, placementID, placement.Rows, placement.ZIndex,
	)
	sequence := fmt.Sprintf("\x1b7\x1b[%d;%dH%s\x1b8", placement.Y+1, placement.X+1, kittyCommand(control, ""))
	if _, err := io.WriteString(k.writer, sequence); err != nil {
		return fmt.Errorf("place Kitty image: %w", err)
	}
	return nil
}

func (k *Kitty) deleteImage(imageID uint32) error {
	_, err := io.WriteString(k.writer, kittyCommand(fmt.Sprintf("a=d,d=I,i=%d,q=2", imageID), ""))
	return err
}

func kittyCommand(control, payload string) string {
	return "\x1b_G" + control + ";" + payload + "\x1b\\"
}
