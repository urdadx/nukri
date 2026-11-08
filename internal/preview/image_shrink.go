package preview

import (
	"image"
	"image/color"
)

/*
shrinkForScaling box-averages src by a whole factor so it stays at least twice
maxDimension on its longest side, or returns src unchanged when it is already
that small or its pixel layout has no fast path.

The final resize uses CatmullRom, whose cost grows with the source size: on a
24-megapixel photo it takes over a second. Averaging blocks first is a single
cheap pass, doesn't alias the way point sampling does, and leaves CatmullRom a
small image to finish with full quality.
*/
func shrinkForScaling(src image.Image, maxDimension int) image.Image {
	bounds := src.Bounds()
	factor := max(bounds.Dx(), bounds.Dy()) / (2 * maxDimension)
	// The fast paths index pixel buffers from the origin, as decoders produce.
	if factor < 2 || bounds.Min != (image.Point{}) {
		return src
	}
	width, height := bounds.Dx()/factor, bounds.Dy()/factor
	switch source := src.(type) {
	case *image.YCbCr:
		return shrinkYCbCr(source, factor, width, height)
	case *image.RGBA:
		return &image.RGBA{Pix: shrinkFourChannel(source.Pix, source.Stride, bounds, factor, width, height), Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
	case *image.NRGBA:
		return &image.NRGBA{Pix: shrinkFourChannel(source.Pix, source.Stride, bounds, factor, width, height), Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
	case *image.Gray:
		return shrinkGray(source, factor, width, height)
	}
	return src
}

// shrinkFourChannel averages factor×factor blocks of an RGBA-style pixel buffer.
func shrinkFourChannel(pix []byte, stride int, bounds image.Rectangle, factor, width, height int) []byte {
	out := make([]byte, width*height*4)
	area := uint32(factor * factor)
	sums := make([]uint32, width*4)
	for y := range height {
		clear(sums)
		for row := y * factor; row < (y+1)*factor; row++ {
			line := pix[row*stride:]
			for x := range width {
				sum := sums[x*4 : x*4+4]
				for pixel := x * factor * 4; pixel < (x+1)*factor*4; pixel += 4 {
					sum[0] += uint32(line[pixel])
					sum[1] += uint32(line[pixel+1])
					sum[2] += uint32(line[pixel+2])
					sum[3] += uint32(line[pixel+3])
				}
			}
		}
		target := out[y*width*4:]
		for index, sum := range sums {
			target[index] = uint8(sum / area)
		}
	}
	return out
}

func shrinkGray(src *image.Gray, factor, width, height int) *image.Gray {
	out := image.NewGray(image.Rect(0, 0, width, height))
	area := uint32(factor * factor)
	sums := make([]uint32, width)
	for y := range height {
		clear(sums)
		for row := y * factor; row < (y+1)*factor; row++ {
			line := src.Pix[row*src.Stride:]
			for x := range width {
				for pixel := x * factor; pixel < (x+1)*factor; pixel++ {
					sums[x] += uint32(line[pixel])
				}
			}
		}
		for x, sum := range sums {
			out.Pix[y*out.Stride+x] = uint8(sum / area)
		}
	}
	return out
}

// shrinkYCbCr averages luma over each block and chroma over the block's chroma
// samples, then converts once per output pixel instead of once per input pixel.
func shrinkYCbCr(src *image.YCbCr, factor, width, height int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	origin := src.Rect.Min
	for y := range height {
		for x := range width {
			var luma, blue, red, samples uint32
			for row := y * factor; row < (y+1)*factor; row++ {
				line := src.Y[row*src.YStride:]
				for column := x * factor; column < (x+1)*factor; column++ {
					luma += uint32(line[column])
				}
			}
			// Chroma is often subsampled, so step through its own grid.
			lastC := -1
			for row := y * factor; row < (y+1)*factor; row++ {
				for column := x * factor; column < (x+1)*factor; column++ {
					offset := src.COffset(origin.X+column, origin.Y+row)
					if offset == lastC {
						continue
					}
					lastC = offset
					blue += uint32(src.Cb[offset])
					red += uint32(src.Cr[offset])
					samples++
				}
			}
			r, g, b := color.YCbCrToRGB(uint8(luma/uint32(factor*factor)), uint8(blue/samples), uint8(red/samples))
			pixel := out.PixOffset(x, y)
			out.Pix[pixel], out.Pix[pixel+1], out.Pix[pixel+2], out.Pix[pixel+3] = r, g, b, 0xff
		}
	}
	return out
}
