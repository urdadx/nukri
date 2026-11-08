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

// renderSVG rasterises an SVG with rsvg-convert (from librsvg).
func (s *Service) renderSVG(ctx context.Context, path string) (*SVGPreview, error) {
	if s.tools.RSVGConvert == "" {
		return nil, fmt.Errorf("SVG preview: %w", ToolUnavailable("rsvg-convert"))
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("open SVG: %w", err)
	}
	if info.Size() > s.maxSVGBytes {
		return nil, ErrOutputTooLarge
	}
	// Render at the SVG's own size first; only oversized drawings are re-rendered
	// to fit, so small icons are not scaled up.
	image, err := s.rasterizeSVG(ctx, path, 0)
	if errors.Is(err, ErrOutputTooLarge) {
		image, err = s.rasterizeSVG(ctx, path, s.maxImageDimension)
	}
	if err != nil {
		return nil, err
	}
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
