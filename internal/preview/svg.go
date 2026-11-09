package preview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"strconv"
)

/*
renderSVG rasterises an SVG with rsvg-convert (from librsvg), or with
ImageMagick's renderer when rsvg-convert is missing: many systems have
ImageMagick but not librsvg's tools. The result is cached on disk by file
identity and pane size, so a revisit runs neither.
*/
func (s *Service) renderSVG(ctx context.Context, path string, cellWidth int) (*SVGPreview, error) {
	if s.tools.RSVGConvert == "" && s.tools.ImageMagick == "" {
		return nil, fmt.Errorf("SVG preview: %w", ToolUnavailable("rsvg-convert"))
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("open SVG: %w", err)
	}
	if info.Size() > s.maxSVGBytes {
		return nil, ErrOutputTooLarge
	}
	targetSize := s.imageTargetSize(cellWidth)
	cacheKey := s.imageCache.Key(path, info.Size(), info.ModTime(), targetSize)
	if cached, ok := s.imageCache.Get(cacheKey); ok {
		return &SVGPreview{Image: Image{MediaType: "image/png", Data: cached.Data, Width: cached.Width, Height: cached.Height}}, nil
	}

	var image Image
	if s.tools.RSVGConvert != "" {
		// Render at the SVG's own size first; only oversized drawings are
		// re-rendered to fit, so small icons are not scaled up.
		image, err = s.rasterizeSVG(ctx, path, 0)
		if errors.Is(err, ErrOutputTooLarge) {
			image, err = s.rasterizeSVG(ctx, path, s.maxImageDimension)
		}
	} else {
		// ImageMagick renders at the SVG's own size, then shrinks to the pane.
		// Its own SVG renderer misses parts of CSS and text styling; when it
		// fails, rsvg-convert is the fix, so the preview names it.
		image, err = s.imageMagickStill(ctx, path, targetSize, "-background", "none")
		if err != nil && ctx.Err() == nil {
			err = fmt.Errorf("ImageMagick could not render this SVG (%v): %w", err, ToolUnavailable("rsvg-convert"))
		}
	}
	if err != nil {
		return nil, err
	}
	s.imageCache.Put(cacheKey, image.Data, image.Width, image.Height, "svg", 0, 0, info.Size(), nil)
	return &SVGPreview{Image: image}, nil
}

// rasterizeSVG renders path to PNG, fitting it within fit pixels when fit > 0.
func (s *Service) rasterizeSVG(ctx context.Context, path string, fit int) (Image, error) {
	arguments := []string{"--format=png"}
	if fit > 0 {
		size := strconv.Itoa(fit)
		arguments = append(arguments, "--keep-aspect-ratio", "--width="+size, "--height="+size)
	}
	output, err := runCommand(ctx, s.maxImageBytes, s.tools.RSVGConvert, append(arguments, path)...)
	if err != nil {
		if errors.Is(err, ErrOutputTooLarge) {
			return Image{}, ErrOutputTooLarge
		}
		return Image{}, fmt.Errorf("render SVG: %w", err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(output))
	if err != nil {
		return Image{}, fmt.Errorf("render SVG: %w", err)
	}
	if config.Width > s.maxImageDimension || config.Height > s.maxImageDimension {
		return Image{}, ErrOutputTooLarge
	}
	return Image{MediaType: "image/png", Data: output, Width: config.Width, Height: config.Height}, nil
}
