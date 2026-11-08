package browser

import (
	"io"
	"sync"

	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/preview/image_protocols"
)

/*
VisualState tracks the preview image. Rendering only records where the image
should be (Show, Clear); Flush writes the protocol sequences, called by the
terminal output right after bubbletea writes a frame. That keeps View free of
I/O, keeps image bytes from interleaving with frame output, and draws images
after the text they sit on.

The last placed image is remembered so an unchanged image isn't re-sent, and a
moved one is re-placed rather than re-transmitted where the protocol allows.
*/
type VisualState struct {
	mu       sync.Mutex
	renderer image_protocols.ImageRenderer

	// What the last render asked for.
	want      *preview.Image
	wantPlace image_protocols.Placement

	// What is on screen.
	last   image_protocols.RenderedImage
	data   *byte // identity of the placed image's bytes; see sameImageData
	size   int
	place  image_protocols.Placement
	placed bool
}

// Show asks for image to be drawn at the given cells on the next flush.
func (s *VisualState) Show(image *preview.Image, x, y, columns, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.want = image
	s.wantPlace = image_protocols.Placement{X: x, Y: y, Columns: columns, Rows: rows}
}

// Clear asks for the image to be removed on the next flush.
func (s *VisualState) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.want = nil
}

/*
Flush brings the terminal in line with the last Show or Clear, writing to
terminal. It runs after each frame is written: an unchanged image costs nothing,
except with protocols whose images the frame's text can overwrite, which are
redrawn.
*/
func (s *VisualState) Flush(terminal io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.renderer == nil {
		s.renderer = image_protocols.NewRenderer(terminal)
	}
	if !s.renderer.Supported() {
		return
	}
	if s.want == nil || len(s.want.Data) == 0 {
		if s.placed {
			_ = s.renderer.Delete(s.last)
		}
		s.forget()
		return
	}
	same := s.placed && s.sameImageData(s.want.Data)
	if same && s.place == s.wantPlace && s.renderer.Persistent() {
		return
	}
	// Same image at a new position or size (or one that needs redrawing): move
	// it instead of re-sending it.
	if same {
		if moved, err := s.renderer.Move(s.last, s.wantPlace); err == nil {
			s.last, s.place = moved, s.wantPlace
			return
		}
	}
	if s.placed {
		_ = s.renderer.Delete(s.last)
	}
	placed, err := s.renderer.Place(*s.want, s.wantPlace)
	if err != nil {
		s.forget()
		return
	}
	s.last, s.place, s.placed = placed, s.wantPlace, true
	s.data, s.size = dataIdentity(s.want.Data)
}

func (s *VisualState) forget() {
	s.last = image_protocols.RenderedImage{}
	s.data, s.size = nil, 0
	s.placed = false
}

// sameImageData reports whether data is the exact buffer last placed. Preview
// results are cached and never mutated, so buffer identity stands in for
// content: this runs every frame and must not hash megabytes of image data.
func (s *VisualState) sameImageData(data []byte) bool {
	pointer, size := dataIdentity(data)
	return pointer == s.data && size == s.size
}

func dataIdentity(data []byte) (*byte, int) {
	if len(data) == 0 {
		return nil, 0
	}
	return &data[0], len(data)
}

// defaultCellAspect is the pixel height to pixel width ratio of a terminal cell
// (typical cells are ~2x taller than wide). Used to translate image pixel aspect
// into a cell grid so placed images keep their proportions instead of stretching
// to fill the pane.
const defaultCellAspect = 2.0

// fitImageToPane returns the largest cell grid (columns x rows) that fits within
// maxCols x maxRows while preserving the image's aspect ratio, accounting for the
// terminal cell pixel aspect. Returns (0,0) for a degenerate image.
func fitImageToPane(imgWidth, imgHeight, maxCols, maxRows int, cellAspect float64) (int, int) {
	if imgWidth <= 0 || imgHeight <= 0 || maxCols <= 0 || maxRows <= 0 {
		return 0, 0
	}
	// Pixel-space rectangle the pane can cover.
	availPW := float64(maxCols) * 1.0
	availPH := float64(maxRows) * cellAspect
	// Scale the image to fit that pixel rectangle, preserving aspect.
	scale := min(availPW/float64(imgWidth), availPH/float64(imgHeight))
	if scale <= 0 {
		return 0, 0
	}
	cols := int(float64(imgWidth) * scale)
	rows := int(float64(imgHeight) * scale / cellAspect)
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	return cols, rows
}
