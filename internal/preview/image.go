package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/urdadx/nukri/internal/fileinfo"
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
				Data:      cached.Data,
				Width:     cached.Width,
				Height:    cached.Height,
			},
		}, nil
	}

	if strings.EqualFold(filepath.Ext(path), ".avif") {
		return s.renderAVIF(ctx, path, cacheKey, targetSize, info.Size())
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image preview: %w", err)
	}
	defer file.Close()
	config, sourceFormat, err := image.DecodeConfig(file)
	if err != nil {
		return nil, fmt.Errorf("decode image preview: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > DefaultMaxDecodePixels {
		return nil, ErrOutputTooLarge
	}
	/*
		A JPEG can be decoded straight to a fraction of its size, which Go's
		decoder can't do; ImageMagick (libjpeg) can, and is several times faster on
		camera photos. Go stays the fallback when it's missing or fails.
	*/
	if sourceFormat == "jpeg" && s.tools.ImageMagick != "" && max(config.Width, config.Height) > 2*targetSize {
		if preview, err := s.imageMagickStill(ctx, path, targetSize, "-define", fmt.Sprintf("jpeg:size=%dx%d", 2*targetSize, 2*targetSize)); err == nil {
			s.imageCache.Put(cacheKey, preview.Data, preview.Width, preview.Height, sourceFormat, config.Width, config.Height, info.Size(), nil)
			return &ImagePreview{Image: preview}, nil
		} else if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("decode image preview: %w", err)
	}
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
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(output, preview); err != nil {
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
			Data:      output.buffer.Bytes(),
			Width:     cacheW,
			Height:    cacheH,
		},
	}, nil
}

/*
renderAVIF decodes AVIF with an external tool; Go's image packages have no AVIF
decoder. ImageMagick is tried first because ffmpeg only reads still AVIF images
from version 6 or so, and many distributions ship older builds.
*/
func (s *Service) renderAVIF(ctx context.Context, path, cacheKey string, targetSize int, size int64) (*ImagePreview, error) {
	var preview Image
	var err error
	switch {
	case s.tools.ImageMagick != "":
		preview, err = s.imageMagickStill(ctx, path, targetSize)
	case s.tools.FFmpeg != "":
		preview, err = s.ffmpegStill(ctx, path, targetSize, true)
	default:
		return nil, fmt.Errorf("AVIF preview: %w", ToolUnavailable("imagemagick"))
	}
	if err != nil {
		return nil, fmt.Errorf("decode image preview: %w", err)
	}
	s.imageCache.Put(cacheKey, preview.Data, preview.Width, preview.Height, "avif", 0, 0, size, nil)
	return &ImagePreview{Image: preview}, nil
}

// imageMagickStill converts the first frame of path to a PNG no larger than
// targetSize on either side. readOptions go before the input, where decoder
// hints such as jpeg:size apply.
func (s *Service) imageMagickStill(ctx context.Context, path string, targetSize int, readOptions ...string) (Image, error) {
	fit := fmt.Sprintf("%dx%d>", targetSize, targetSize) // ">" only ever shrinks
	arguments := append(readOptions, path+"[0]", "-resize", fit, "-define", "png:compression-level=1", "png:-")
	output, err := runCommand(ctx, s.maxImageBytes, s.tools.ImageMagick, arguments...)
	if err != nil {
		return Image{}, err
	}
	config, err := png.DecodeConfig(bytes.NewReader(output))
	if err != nil {
		return Image{}, err
	}
	return Image{MediaType: "image/png", Data: output, Width: config.Width, Height: config.Height}, nil
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
	src = shrinkForScaling(src, maxDimension)
	scale := float64(maxDimension) / float64(max(width, height))
	dstWidth := max(1, int(float64(width)*scale))
	dstHeight := max(1, int(float64(height)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst, nil
}
