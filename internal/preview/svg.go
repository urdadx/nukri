package preview

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
	"os"

	"github.com/xo/resvg"
)

func (s *Service) renderSVG(ctx context.Context, path string) (*SVGPreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open SVG: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, s.maxSVGBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read SVG: %w", err)
	}
	if int64(len(data)) > s.maxSVGBytes {
		return nil, ErrOutputTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	config, err := resvg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("inspect SVG: %w", err)
	}
	var options []resvg.Option
	if config.Width > s.maxImageDimension || config.Height > s.maxImageDimension {
		options = append(options,
			resvg.WithWidth(s.maxImageDimension),
			resvg.WithHeight(s.maxImageDimension),
			resvg.WithScaleMode(resvg.ScaleBestFit),
		)
	}
	image, err := resvg.Render(data, options...)
	if err != nil {
		return nil, fmt.Errorf("render SVG: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bounds := image.Bounds()
	if bounds.Dx() > s.maxImageDimension || bounds.Dy() > s.maxImageDimension {
		return nil, ErrOutputTooLarge
	}
	output := &limitedBuffer{limit: s.maxImageBytes}
	if err := png.Encode(output, image); err != nil {
		return nil, fmt.Errorf("encode SVG preview: %w", err)
	}
	return &SVGPreview{
		Image: Image{
			MediaType: "image/png",
			Data:      append([]byte(nil), output.buffer.Bytes()...),
			Width:     bounds.Dx(),
			Height:    bounds.Dy(),
		},
	}, nil
}
