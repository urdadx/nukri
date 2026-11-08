package browser

import (
	"io"

	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/preview/image_protocols"
)

// VisualState caches the terminal image renderer and the last transmitted image
// so that unchanged selections are not re-encoded or re-transmitted every frame.
// Reuse of the same renderer instance also keeps the Kitty image ID stable, so a
// retransmit overwrites the previous placement instead of allocating a new slot.
type VisualState struct {
	renderer image_protocols.ImageRenderer
	last     image_protocols.RenderedImage
	data     *byte // identity of the placed image's bytes; see sameImageData
	size     int
	x, y     int
	columns  int
	rows     int
	placed   bool
}

// renderVisual transmits the given image only when it differs from the last
// placed image (by content and pane geometry). When unchanged it performs no
// work.
func (s *VisualState) renderVisual(writer io.Writer, image *preview.Image, x, y, columns, rows int) {
	if image == nil || writer == nil {
		return
	}
	if s.placed && s.sameImageData(image.Data) && s.x == x && s.y == y && s.columns == columns && s.rows == rows {
		return
	}
	if s.renderer == nil {
		s.renderer = image_protocols.NewRenderer(writer)
		if !s.renderer.Supported() {
			s.placed = false
			return
		}
	}
	placement := image_protocols.Placement{X: x, Y: y, Columns: columns, Rows: rows}
	// Same image at a new position or size: move it instead of re-sending it.
	if s.placed && s.sameImageData(image.Data) && s.last.ImageID != 0 {
		if moved, err := s.renderer.Move(s.last, placement); err == nil {
			s.last = moved
			s.x, s.y, s.columns, s.rows = x, y, columns, rows
			return
		}
	}
	if s.last.ImageID != 0 {
		_ = s.renderer.Delete(s.last)
	}
	placed, err := s.renderer.Place(*image, placement)
	if err != nil {
		s.placed = false
		return
	}
	s.last = placed
	s.data, s.size = dataIdentity(image.Data)
	s.x, s.y, s.columns, s.rows = x, y, columns, rows
	s.placed = true
}

// Clear deletes any placed image and drops cached state.
func (s *VisualState) Clear() {
	if s.renderer != nil && s.last.ImageID != 0 {
		_ = s.renderer.Delete(s.last)
	}
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
