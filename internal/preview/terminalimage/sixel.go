package terminalimage

import (
	"bytes"
	"fmt"
	"image/png"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/mattn/go-sixel"
	"github.com/urdadx/nukri/internal/preview"
)

const (
	defaultCellPixelWidth  = 8
	defaultCellPixelHeight = 16
)

type Sixel struct {
	writer          io.Writer
	supported       bool
	cellPixelWidth  int
	cellPixelHeight int
	mu              sync.Mutex
	nextID          uint32
	placements      map[uint32]RenderedImage
}

func NewSixel(writer io.Writer) *Sixel {
	return NewSixelWithSupport(writer, IsSixelTerminal())
}

func NewSixelWithSupport(writer io.Writer, supported bool) *Sixel {
	return &Sixel{
		writer: writer, supported: supported,
		cellPixelWidth: defaultCellPixelWidth, cellPixelHeight: defaultCellPixelHeight,
		nextID: 1, placements: make(map[uint32]RenderedImage),
	}
}

func IsSixelTerminal() bool {
	if os.Getenv("NUKRI_SIXEL") == "1" {
		return true
	}
	term := strings.ToLower(os.Getenv("TERM"))
	program := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	return strings.Contains(term, "sixel") || strings.Contains(term, "mlterm") ||
		strings.Contains(term, "yaft") || strings.Contains(term, "wezterm") ||
		strings.Contains(program, "wezterm")
}

func (s *Sixel) Supported() bool {
	return s != nil && s.supported && s.writer != nil
}

func (s *Sixel) Place(value preview.Image, placement Placement) (RenderedImage, error) {
	if !s.Supported() {
		return RenderedImage{}, ErrUnsupportedTerminal
	}
	if value.MediaType != "image/png" || len(value.Data) == 0 {
		return RenderedImage{}, ErrInvalidImage
	}
	if placement.X < 0 || placement.Y < 0 || placement.Columns <= 0 || placement.Rows <= 0 {
		return RenderedImage{}, ErrInvalidPlacement
	}
	image, err := png.Decode(bytes.NewReader(value.Data))
	if err != nil {
		return RenderedImage{}, fmt.Errorf("decode Sixel source PNG: %w", err)
	}
	var encoded bytes.Buffer
	encoder := sixel.NewEncoder(&encoded)
	encoder.Width = placement.Columns * s.cellPixelWidth
	encoder.Height = placement.Rows * s.cellPixelHeight
	encoder.Colors = 256
	encoder.Dither = true
	encoder.Transparent = true
	if err := encoder.Encode(image); err != nil {
		return RenderedImage{}, fmt.Errorf("encode Sixel image: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.allocateID()
	rendered := RenderedImage{
		ImageID: id, PlacementID: id, X: placement.X, Y: placement.Y,
		Columns: placement.Columns, Rows: placement.Rows,
	}
	sequence := fmt.Sprintf("\x1b7\x1b[%d;%dH%s\x1b8", placement.Y+1, placement.X+1, encoded.Bytes())
	if _, err := io.WriteString(s.writer, sequence); err != nil {
		return RenderedImage{}, fmt.Errorf("place Sixel image: %w", err)
	}
	s.placements[id] = rendered
	return rendered, nil
}

func (s *Sixel) DeletePlacement(image RenderedImage) error {
	return s.Delete(image)
}

func (s *Sixel) Delete(image RenderedImage) error {
	if !s.Supported() {
		return ErrUnsupportedTerminal
	}
	if image.PlacementID == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	placed, ok := s.placements[image.PlacementID]
	if !ok {
		return nil
	}
	if err := s.clearPlacement(placed); err != nil {
		return err
	}
	delete(s.placements, image.PlacementID)
	return nil
}

func (s *Sixel) DeleteAll() error {
	if !s.Supported() {
		return ErrUnsupportedTerminal
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, placement := range s.placements {
		if err := s.clearPlacement(placement); err != nil {
			return err
		}
		delete(s.placements, id)
	}
	return nil
}

func (s *Sixel) allocateID() uint32 {
	id := s.nextID
	s.nextID++
	if s.nextID == 0 {
		s.nextID = 1
	}
	return id
}

func (s *Sixel) clearPlacement(placement RenderedImage) error {
	var sequence strings.Builder
	sequence.WriteString("\x1b7")
	blank := strings.Repeat(" ", placement.Columns)
	for row := 0; row < placement.Rows; row++ {
		fmt.Fprintf(&sequence, "\x1b[%d;%dH%s", placement.Y+row+1, placement.X+1, blank)
	}
	sequence.WriteString("\x1b8")
	if _, err := io.WriteString(s.writer, sequence.String()); err != nil {
		return fmt.Errorf("clear Sixel image: %w", err)
	}
	return nil
}
