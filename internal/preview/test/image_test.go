package test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/fileinfo"
	"github.com/urdadx/nukri/internal/preview"
)

// gradient.avif is a 320x200 AVIF, decoded by ImageMagick (or ffmpeg).
func TestRenderAVIFImagePreview(t *testing.T) {
	service := preview.NewService()
	requireCapability(t, service.Capabilities().AVIF, "imagemagick or ffmpeg")
	// Copy to a fresh path so the on-disk image cache can't answer.
	data, err := os.ReadFile("testdata/gradient.avif")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gradient.avif")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectPath(path, core.File)
	if facts.BuiltinClass != core.FileClassImage {
		t.Fatalf("expected AVIF to be classified as image, got %v", facts.BuiltinClass)
	}
	result, err := service.Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: 60})
	if err != nil {
		t.Fatalf("render AVIF: %v", err)
	}
	image, ok := result.(*preview.ImagePreview)
	if !ok {
		t.Fatalf("expected *ImagePreview, got %T", result)
	}
	if image.Image.Width != 320 || image.Image.Height != 200 {
		t.Fatalf("AVIF preview is %dx%d, want 320x200", image.Image.Width, image.Image.Height)
	}
	if image.Image.MediaType != "image/png" || len(image.Image.Data) == 0 {
		t.Fatalf("expected PNG data for AVIF preview, got media=%q len=%d", image.Image.MediaType, len(image.Image.Data))
	}
}

func TestRenderPNGImagePreview(t *testing.T) {
	path := "testdata/shopping.png"
	f := fileinfo.InspectPath(path, core.File)
	result, err := preview.NewService().Render(context.Background(), preview.Request{Path: path, Facts: f, Width: 40})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	image, ok := result.(*preview.ImagePreview)
	if !ok {
		t.Fatalf("expected *ImagePreview, got %T", result)
	}
	if image.Image.MediaType != "image/png" {
		t.Errorf("expected PNG media type, got %q", image.Image.MediaType)
	}
	if image.Image.Width == 0 || image.Image.Height == 0 {
		t.Errorf("expected non-zero dimensions, got %dx%d", image.Image.Width, image.Image.Height)
	}
	if len(image.Image.Data) == 0 {
		t.Error("expected PNG data")
	}
	if len(image.Metadata) != 0 {
		t.Errorf("expected no metadata fields, got %#v", image.Metadata)
	}

	view, err := preview.BuildView(result, preview.ViewOptions{Width: 40})
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if view.Visual == nil {
		t.Error("expected visual image in view")
	}
	for _, line := range view.Lines {
		for _, r := range line {
			if r < 0x20 && r != '\t' {
				t.Errorf("unexpected control character %q in metadata line %q", r, line)
			}
		}
	}
}

func TestRenderLargeJPEGDownscales(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.jpg")
	// Build a 6000x3000 photographic-style JPEG so lossless PNG re-encoding
	// would exceed the output limit if it weren't downscaled first.
	img := image.NewRGBA(image.Rect(0, 0, 6000, 3000))
	for y := 0; y < 3000; y++ {
		for x := 0; x < 6000; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), uint8((x + y) % 255), 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	file.Close()

	facts := fileinfo.InspectPath(path, core.File)
	result, err := preview.NewService().Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: 60})
	if err != nil {
		t.Fatalf("render large JPEG: %v", err)
	}
	image, ok := result.(*preview.ImagePreview)
	if !ok {
		t.Fatalf("expected *ImagePreview, got %T", result)
	}
	if image.Image.Width > 1600 || image.Image.Height > 1600 {
		t.Errorf("preview not downscaled: got %dx%d, want longest side <= 1600", image.Image.Width, image.Image.Height)
	}
	if image.Image.Width == 0 || image.Image.Height == 0 {
		t.Error("expected non-zero preview dimensions")
	}
	if len(image.Image.Data) == 0 {
		t.Error("expected PNG data")
	}
	if len(image.Image.Data) > 16<<20 {
		t.Errorf("preview PNG exceeds output limit: %d bytes", len(image.Image.Data))
	}
}

func TestRenderJPEGRejectsEnormousSource(t *testing.T) {
	// The source file itself exceeding the byte limit should still be rejected
	// by the input-size guard rather than being decoded.
	path := filepath.Join(t.TempDir(), "huge.jpg")
	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = 0xff
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err := file.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	file.Close()

	facts := fileinfo.InspectPath(path, core.File)
	_, err = preview.NewService().Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: 60})
	if !errors.Is(err, preview.ErrOutputTooLarge) {
		t.Fatalf("expected ErrOutputTooLarge for over-limit source, got %v", err)
	}
}

// renderJPEGAt writes a w*h gradient JPEG to a temp path and returns the path.
func renderJPEGAt(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), uint8((x + y) % 255), 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	file.Close()
	return path
}

func renderImagePreview(t *testing.T, path string, width int) *preview.ImagePreview {
	t.Helper()
	facts := fileinfo.InspectPath(path, core.File)
	result, err := preview.NewService().Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: width})
	if err != nil {
		t.Fatalf("render %q at width %d: %v", path, width, err)
	}
	img, ok := result.(*preview.ImagePreview)
	if !ok {
		t.Fatalf("expected *ImagePreview, got %T", result)
	}
	return img
}

func TestImagePreviewDownscalesToDisplaySize(t *testing.T) {
	path := renderJPEGAt(t, t.TempDir(), "big.jpg", 2000, 1000)

	// A small pane width (60 cells -> 480px target) produces a preview bounded
	// by the display-derived target, far below maxImageDimension.
	small := renderImagePreview(t, path, 60)
	if small.Image.Width > 480 || small.Image.Height > 480 {
		t.Fatalf("expected preview bounded by display target 480px, got %dx%d", small.Image.Width, small.Image.Height)
	}

	// An unknown/zero width falls back to maxImageDimension (1600).
	wide := renderImagePreview(t, path, 0)
	if wide.Image.Width > 1600 || wide.Image.Height > 1600 {
		t.Fatalf("expected preview bounded by maxImageDimension, got %dx%d", wide.Image.Width, wide.Image.Height)
	}
	if wide.Image.Width <= small.Image.Width {
		t.Fatalf("expected wider target to yield a larger preview; small=%dx%d wide=%dx%d",
			small.Image.Width, small.Image.Height, wide.Image.Width, wide.Image.Height)
	}
}

func TestImagePreviewServesFromDiskCache(t *testing.T) {
	path := renderJPEGAt(t, t.TempDir(), "photo.jpg", 1200, 800)
	src, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	size, mod := src.Size(), src.ModTime()

	first := renderImagePreview(t, path, 50)

	// Overwrite the source with garbage but preserve its size and modification
	// time, so the cache key is unchanged. A successful second render with
	// identical bytes can only come from the on-disk cache (the source is now
	// undecodable garbage).
	garbage := make([]byte, size)
	for i := range garbage {
		garbage[i] = 0xAB
	}
	if err := os.WriteFile(path, garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}

	second := renderImagePreview(t, path, 50)
	if string(second.Image.Data) != string(first.Image.Data) {
		t.Fatal("cached preview bytes differ from the first render")
	}
	if second.Image.Width != first.Image.Width || second.Image.Height != first.Image.Height {
		t.Fatalf("cached dimensions differ: first=%dx%d second=%dx%d",
			first.Image.Width, first.Image.Height, second.Image.Width, second.Image.Height)
	}
}

// A tiny PNG may declare enormous dimensions; it must be rejected before the
// decoder allocates the full bitmap.
func TestImagePreviewRejectsDecompressionBomb(t *testing.T) {
	// A real PNG header (IHDR) declaring 100000x100000 pixels, with no image data.
	var header bytes.Buffer
	if err := png.Encode(&header, image.NewGray(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	data := header.Bytes()
	// IHDR width and height live at bytes 16..23; the IHDR CRC follows.
	binary.BigEndian.PutUint32(data[16:], 100_000)
	binary.BigEndian.PutUint32(data[20:], 100_000)
	binary.BigEndian.PutUint32(data[29:], crc32.ChecksumIEEE(data[12:29]))
	path := filepath.Join(t.TempDir(), "bomb.png")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	facts := fileinfo.InspectPath(path, core.File)
	_, err := preview.NewService().Render(context.Background(), preview.Request{Path: path, Facts: facts, Width: 40})
	if !errors.Is(err, preview.ErrOutputTooLarge) {
		t.Fatalf("error = %v, want ErrOutputTooLarge", err)
	}
}

// Large images are box-averaged before the final resize; each pixel layout has
// its own fast path, so check every one keeps colors and proportions.
func TestImagePreviewShrinksEveryPixelLayout(t *testing.T) {
	want := color.NRGBA{R: 200, G: 100, B: 50, A: 255}
	gray := color.Gray{Y: 140}
	layouts := []struct {
		name   string
		ext    string
		image  image.Image
		encode func(io.Writer, image.Image) error
		want   color.Color
	}{
		{"rgba png", ".png", image.NewRGBA(image.Rect(0, 0, 3000, 1500)), png.Encode, want},
		// Translucent, so the PNG decodes as NRGBA rather than RGBA.
		{"nrgba png", ".png", image.NewNRGBA(image.Rect(0, 0, 3000, 1500)), png.Encode, color.NRGBA{R: 200, G: 100, B: 50, A: 160}},
		{"gray png", ".png", image.NewGray(image.Rect(0, 0, 3000, 1500)), png.Encode, gray},
		{"ycbcr jpeg", ".jpg", image.NewRGBA(image.Rect(0, 0, 3000, 1500)), func(w io.Writer, m image.Image) error {
			return jpeg.Encode(w, m, &jpeg.Options{Quality: 95})
		}, want},
	}
	// Without ImageMagick, JPEGs take the Go decoder and its YCbCr fast path.
	tools := preview.DiscoverTools()
	tools.ImageMagick = ""
	for _, layout := range layouts {
		t.Run(layout.name, func(t *testing.T) {
			draw.Draw(layout.image.(draw.Image), layout.image.Bounds(), image.NewUniform(layout.want), image.Point{}, draw.Src)
			path := filepath.Join(t.TempDir(), "large"+layout.ext)
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := layout.encode(file, layout.image); err != nil {
				t.Fatal(err)
			}
			file.Close()
			result, err := preview.NewServiceWithTools(tools).Render(context.Background(), preview.Request{
				Path: path, Facts: fileinfo.InspectPath(path, core.File), Width: 30,
			})
			if err != nil {
				t.Fatal(err)
			}
			data := result.(*preview.ImagePreview).Image.Data
			decoded, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if bounds := decoded.Bounds(); bounds.Dx() != 240 || bounds.Dy() != 120 {
				t.Fatalf("preview is %dx%d, want 240x120", bounds.Dx(), bounds.Dy())
			}
			got := color.NRGBAModel.Convert(decoded.At(120, 60)).(color.NRGBA)
			expected := color.NRGBAModel.Convert(layout.want).(color.NRGBA)
			if diff(got.R, expected.R) > 3 || diff(got.G, expected.G) > 3 || diff(got.B, expected.B) > 3 {
				t.Fatalf("center pixel = %v, want about %v", got, expected)
			}
		})
	}
}

func diff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
