package preview

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"os"

	_ "github.com/gen2brain/avif"
	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	fileinfo "github.com/urdadx/nukri/internal/file_info"
)

/*
cellPixelWidth approximates the pixel width of a terminal cell when the true
cell size is unknown. It is used to translate a preview pane's width in cells
into a target pixel size so images are downscaled to roughly the right display
resolution instead of a fixed, often-larger cap.
*/
const cellPixelWidth = 8.0

/*
renderImage decodes a raster image file and re-encodes it as a PNG preview.
Images larger than the display dimension are downscaled first so the losslessly
encoded PNG stays small enough to transmit to the terminal (a raw-sized JPEG
would otherwise inflate well beyond the output limit when re-encoded as PNG).

Downscaling targets the pane's display size (derived from the pane width in
cells), capped at maxImageDimension. The resized PNG is cached on disk keyed by
the source file identity and the target size, so revisiting a file or rendering
the same size again skips the decode + scale + encode pass entirely.
*/
func (s *Service) renderImage(ctx context.Context, path string, facts fileinfo.FileFacts, cellWidth int) (*ImagePreview, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat image preview: %w", err)
	}
	if info.Size() > s.maxImageBytes {
		return nil, ErrOutputTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	targetSize := s.imageTargetSize(cellWidth)
	cacheKey := s.imageCache.Key(path, info.Size(), info.ModTime(), targetSize)
	if cached, ok := s.imageCache.Get(cacheKey); ok {
		return &ImagePreview{
			Image: Image{
				MediaType: "image/png",
				Data:      append([]byte(nil), cached.Data...),
				Width:     cached.Width,
				Height:    cached.Height,
			},
		}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image preview: %w", err)
	}
	defer file.Close()
	decoded, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode image preview: %w", err)
	}
	bounds := decoded.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	preview, err := fitImage(decoded, width, height, targetSize)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output := &limitedBuffer{limit: s.maxImageBytes}
	if err := png.Encode(output, preview); err != nil {
		return nil, fmt.Errorf("encode image preview: %w", err)
	}
	previewBounds := preview.Bounds()

	cacheW, cacheH := previewBounds.Dx(), previewBounds.Dy()
	if cacheW > 0 && cacheH > 0 {
		s.imageCache.Put(cacheKey, output.buffer.Bytes(), cacheW, cacheH, format, width, height, info.Size(), nil)
	}

	return &ImagePreview{
		Image: Image{
			MediaType: "image/png",
			Data:      append([]byte(nil), output.buffer.Bytes()...),
			Width:     cacheW,
			Height:    cacheH,
		},
	}, nil
}

/*
imageTargetSize returns the longest-side pixel dimension the preview should be
downscaled to. It prefers the display size derived from the pane width in cells
(target width = cells * cellPixelWidth), falling back to maxImageDimension when
the pane width is unknown, and never exceeds maxImageDimension.
*/
func (s *Service) imageTargetSize(cellWidth int) int {
	display := int(float64(max(cellWidth, 0)) * cellPixelWidth)
	if display < 1 {
		return s.maxImageDimension
	}
	return min(display, s.maxImageDimension)
}

/*
fitImage returns the decoded image scaled down to fit within maxDimension on
its longest side, or the original when it already fits. Images that fit keep
their original type; oversized ones are scaled onto a new RGBA canvas.
*/
func fitImage(src image.Image, width, height, maxDimension int) (image.Image, error) {
	if width <= maxDimension && height <= maxDimension {
		return src, nil
	}
	scale := float64(maxDimension) / float64(max(width, height))
	dstWidth := max(1, int(float64(width)*scale))
	dstHeight := max(1, int(float64(height)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst, nil
}
