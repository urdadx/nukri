package image_protocols

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/mattn/go-sixel"
	"github.com/urdadx/nukri/internal/preview"
	"golang.org/x/image/draw"
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

	// The last decoded source and its last encoding. Sixel has no stored images,
	// so a move or redraw must re-send pixels; these keep it from re-decoding the
	// PNG, and from re-encoding when the size is unchanged.
	source      image.Image
	sourceData  *byte
	sourceSize  int
	encoded     []byte
	encodedFor  *byte
	encodedCols int
	encodedRows int
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

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadSource(value.Data); err != nil {
		return RenderedImage{}, err
	}
	id := s.allocateID()
	return s.draw(RenderedImage{ImageID: id, PlacementID: id}, placement)
}

// Move redraws the last placed image at a new position or size.
func (s *Sixel) Move(image RenderedImage, placement Placement) (RenderedImage, error) {
	if !s.Supported() {
		return RenderedImage{}, ErrUnsupportedTerminal
	}
	if placement.X < 0 || placement.Y < 0 || placement.Columns <= 0 || placement.Rows <= 0 {
		return RenderedImage{}, ErrInvalidPlacement
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	placed, ok := s.placements[image.PlacementID]
	if !ok || s.source == nil {
		return RenderedImage{}, ErrInvalidImage
	}
	if err := s.clearPlacement(placed); err != nil {
		return RenderedImage{}, err
	}
	return s.draw(image, placement)
}

func (s *Sixel) loadSource(data []byte) error {
	if s.source != nil && s.sourceData == &data[0] && s.sourceSize == len(data) {
		return nil
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode Sixel source PNG: %w", err)
	}
	s.source, s.sourceData, s.sourceSize = decoded, &data[0], len(data)
	s.encoded = nil
	return nil
}

// draw writes the current source at placement, encoding it only when the cell
// size differs from the last encoding.
func (s *Sixel) draw(rendered RenderedImage, placement Placement) (RenderedImage, error) {
	if s.encoded == nil || s.encodedFor != s.sourceData || s.encodedCols != placement.Columns || s.encodedRows != placement.Rows {
		encoded, err := s.encode(placement)
		if err != nil {
			return RenderedImage{}, err
		}
		s.encoded, s.encodedFor, s.encodedCols, s.encodedRows = encoded, s.sourceData, placement.Columns, placement.Rows
	}
	rendered.X, rendered.Y, rendered.Columns, rendered.Rows = placement.X, placement.Y, placement.Columns, placement.Rows
	sequence := fmt.Sprintf("\x1b7\x1b[%d;%dH%s\x1b8", placement.Y+1, placement.X+1, s.encoded)
	if _, err := io.WriteString(s.writer, sequence); err != nil {
		return RenderedImage{}, fmt.Errorf("place Sixel image: %w", err)
	}
	s.placements[rendered.PlacementID] = rendered
	return rendered, nil
}

/*
encode scales the source to fit the placement's pixel box before quantizing.
The encoder's Width and Height only crop, so without scaling a large image would
be cut off, and dithering would run over every source pixel.
*/
func (s *Sixel) encode(placement Placement) ([]byte, error) {
	boxWidth := placement.Columns * s.cellPixelWidth
	boxHeight := placement.Rows * s.cellPixelHeight
	source := s.source
	bounds := source.Bounds()
	if bounds.Dx() > boxWidth || bounds.Dy() > boxHeight {
		scale := min(float64(boxWidth)/float64(bounds.Dx()), float64(boxHeight)/float64(bounds.Dy()))
		width := max(1, int(float64(bounds.Dx())*scale))
		height := max(1, int(float64(bounds.Dy())*scale))
		scaled := image.NewNRGBA(image.Rect(0, 0, width, height))
		draw.ApproxBiLinear.Scale(scaled, scaled.Bounds(), source, bounds, draw.Src, nil)
		source = scaled
	}
	var encoded bytes.Buffer
	encoder := sixel.NewEncoder(&encoded)
	// source now fits the box; its own size keeps the raster attributes exact.
	encoder.Width = min(source.Bounds().Dx(), boxWidth)
	encoder.Height = min(source.Bounds().Dy(), boxHeight)
	encoder.Colors = 256
	encoder.Dither = true
	encoder.Transparent = true
	if err := encoder.Encode(source); err != nil {
		return nil, fmt.Errorf("encode Sixel image: %w", err)
	}
	return encoded.Bytes(), nil
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
