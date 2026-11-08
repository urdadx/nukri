package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	fontparser "github.com/tdewolff/font"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
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
	specimen, err := s.renderFontSpecimen(data)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &FontPreview{Font: value, Specimen: specimen, Metadata: fontFields(value)}, nil
}

// Specimen layout in pixels.
const (
	fontSpecimenWidth  = 800
	fontSpecimenHeight = 340
	fontSpecimenMargin = 40
	fontSpecimenSize   = 34
)

func (s *Service) renderFontSpecimen(data []byte) (Image, error) {
	// x/image reads plain SFNT only; ToSFNT unpacks WOFF, WOFF2 and EOT.
	sfntData, err := fontparser.ToSFNT(data)
	if err != nil {
		return Image{}, fmt.Errorf("load font for layout: %w", err)
	}
	parsed, err := parseFirstFont(sfntData)
	if err != nil {
		return Image{}, fmt.Errorf("load font for layout: %w", err)
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: fontSpecimenSize, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return Image{}, fmt.Errorf("load font for layout: %w", err)
	}
	defer face.Close()

	if fontSpecimenWidth > s.maxImageDimension || fontSpecimenHeight > s.maxImageDimension {
		return Image{}, ErrOutputTooLarge
	}
	rendered := image.NewRGBA(image.Rect(0, 0, fontSpecimenWidth, fontSpecimenHeight))
	draw.Draw(rendered, rendered.Bounds(), image.White, image.Point{}, draw.Src)
	drawer := &font.Drawer{Dst: rendered, Src: image.Black, Face: face}
	metrics := face.Metrics()
	lineHeight := metrics.Height.Mul(fixed.I(5)) / 4
	y := fixed.I(fontSpecimenMargin) + metrics.Ascent
	for _, line := range wrapSpecimen(drawer, fontSpecimen, fixed.I(fontSpecimenWidth-2*fontSpecimenMargin)) {
		if (y + metrics.Descent).Ceil() > fontSpecimenHeight-fontSpecimenMargin/2 {
			break
		}
		drawer.Dot = fixed.Point26_6{X: fixed.I(fontSpecimenMargin), Y: y}
		drawer.DrawString(line)
		y += lineHeight
	}

	output := &limitedBuffer{limit: s.maxImageBytes}
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(output, rendered); err != nil {
		return Image{}, fmt.Errorf("encode font specimen: %w", err)
	}
	return Image{
		MediaType: "image/png",
		Data:      output.buffer.Bytes(),
		Width:     fontSpecimenWidth,
		Height:    fontSpecimenHeight,
	}, nil
}

// parseFirstFont parses a single font, or the first font of a collection.
func parseFirstFont(data []byte) (*opentype.Font, error) {
	if bytes.HasPrefix(data, []byte("ttcf")) {
		collection, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, err
		}
		return collection.Font(0)
	}
	return opentype.Parse(data)
}

// wrapSpecimen splits text into lines that fit width, breaking at spaces.
func wrapSpecimen(drawer *font.Drawer, text string, width fixed.Int26_6) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if line != "" && drawer.MeasureString(candidate) > width {
				lines = append(lines, line)
				candidate = word
			}
			line = candidate
		}
		lines = append(lines, line)
	}
	return lines
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
