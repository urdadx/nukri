package preview

import (
	"context"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
	fontparser "github.com/tdewolff/font"
)

const fontSpecimen = `ABCDEFGHIJKLMNOPQRSTUVWXYZ
abcdefghijklmnopqrstuvwxyz
0123456789 !?@#$%&
The quick brown fox jumps over the lazy dog.`

func (s *Service) renderFont(ctx context.Context, path string) (*FontPreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open font: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, s.maxFontBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read font: %w", err)
	}
	if int64(len(data)) > s.maxFontBytes {
		return nil, ErrOutputTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sfnt, err := fontparser.ParseSFNT(data, 0)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	familyName := fontName(sfnt, fontparser.NamePreferredFamily, fontparser.NameFontFamily)
	if familyName == "" {
		familyName = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	value := Font{
		Family:     familyName,
		Subfamily:  fontName(sfnt, fontparser.NamePreferredSubfamily, fontparser.NameFontSubfamily),
		FullName:   fontName(sfnt, fontparser.NameFull),
		Version:    fontName(sfnt, fontparser.NameVersion),
		Format:     fontFormat(path, sfnt),
		Glyphs:     int(sfnt.NumGlyphs()),
		UnitsPerEm: sfnt.UnitsPerEm(),
	}
	specimen, err := s.renderFontSpecimen(data, familyName)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &FontPreview{Font: value, Specimen: specimen, Metadata: fontFields(value)}, nil
}

func (s *Service) renderFontSpecimen(data []byte, familyName string) (Image, error) {
	family := canvas.NewFontFamily(familyName)
	defer family.Destroy()
	if err := family.LoadFont(data, 0, canvas.FontRegular); err != nil {
		return Image{}, fmt.Errorf("load font for layout: %w", err)
	}
	const width, height = 200.0, 120.0
	value := canvas.New(width, height)
	ctx := canvas.NewContext(value)
	ctx.SetFillColor(color.White)
	ctx.DrawPath(0, 0, canvas.Rectangle(width, height))
	face := family.Face(24, canvas.FontRegular, color.Black)
	text := canvas.NewTextBox(face, fontSpecimen, 180, 100, canvas.Left, canvas.Top, &canvas.TextOptions{
		LineStretch: 0.25,
	})
	ctx.DrawText(10, 110, text)
	rendered := rasterizer.Draw(value, canvas.DPMM(4), canvas.SRGBColorSpace{})
	bounds := rendered.Bounds()
	if bounds.Dx() > s.maxImageDimension || bounds.Dy() > s.maxImageDimension {
		return Image{}, ErrOutputTooLarge
	}
	output := &limitedBuffer{limit: s.maxImageBytes}
	if err := png.Encode(output, rendered); err != nil {
		return Image{}, fmt.Errorf("encode font specimen: %w", err)
	}
	return Image{
		MediaType: "image/png",
		Data:      append([]byte(nil), output.buffer.Bytes()...),
		Width:     bounds.Dx(),
		Height:    bounds.Dy(),
	}, nil
}

func fontName(sfnt *fontparser.SFNT, names ...fontparser.NameID) string {
	for _, name := range names {
		records := sfnt.Name.Get(name)
		if len(records) != 0 {
			return safeText(strings.TrimSpace(records[0].String()))
		}
	}
	return ""
}

func fontFormat(path string, sfnt *fontparser.SFNT) string {
	extension := strings.ToUpper(strings.TrimPrefix(filepath.Ext(path), "."))
	if extension != "" {
		return extension
	}
	if sfnt.IsCFF {
		return "OpenType/CFF"
	}
	if sfnt.IsTrueType {
		return "TrueType"
	}
	return sfnt.Version
}

func fontFields(value Font) []Field {
	fields := make([]Field, 0, 7)
	for _, field := range []Field{
		{Name: "Family", Value: value.Family},
		{Name: "Style", Value: value.Subfamily},
		{Name: "Full name", Value: value.FullName},
		{Name: "Version", Value: value.Version},
		{Name: "Format", Value: value.Format},
		{Name: "Glyphs", Value: fmt.Sprint(value.Glyphs)},
		{Name: "Units per em", Value: fmt.Sprint(value.UnitsPerEm)},
	} {
		if field.Value != "" && field.Value != "0" {
			fields = append(fields, field)
		}
	}
	return fields
}
