package preview

import (
	"bytes"
	"fmt"
	"image"
	"strconv"
)

/*
decodePPM decodes a binary PPM (P6, 8-bit), the raw format pdftoppm writes. It
exists because asking poppler for PNG directly is slow: its encoder compresses
hard, which takes several times longer than rendering the page.
*/
func decodePPM(data []byte, maximumDimension int) (*image.NRGBA, error) {
	fields, pixels, err := ppmHeader(data)
	if err != nil {
		return nil, err
	}
	width, height, maxValue := fields[0], fields[1], fields[2]
	if width <= 0 || height <= 0 || width > maximumDimension || height > maximumDimension {
		return nil, fmt.Errorf("PPM size %dx%d out of range", width, height)
	}
	if maxValue != 255 {
		return nil, fmt.Errorf("unsupported PPM depth %d", maxValue)
	}
	if len(pixels) < width*height*3 {
		return nil, fmt.Errorf("PPM data truncated")
	}
	decoded := image.NewNRGBA(image.Rect(0, 0, width, height))
	for source, target := 0, 0; target < len(decoded.Pix); source, target = source+3, target+4 {
		decoded.Pix[target] = pixels[source]
		decoded.Pix[target+1] = pixels[source+1]
		decoded.Pix[target+2] = pixels[source+2]
		decoded.Pix[target+3] = 0xff
	}
	return decoded, nil
}

// ppmHeader parses "P6 width height maxval" and returns those three numbers
// and the pixel data after the single whitespace that ends the header.
func ppmHeader(data []byte) ([3]int, []byte, error) {
	var fields [3]int
	if !bytes.HasPrefix(data, []byte("P6")) {
		return fields, nil, fmt.Errorf("not a binary PPM")
	}
	position := 2
	for index := range fields {
		// Skip whitespace and # comments before each number.
		for position < len(data) {
			if data[position] == '#' {
				for position < len(data) && data[position] != '\n' {
					position++
				}
				continue
			}
			if !isPPMSpace(data[position]) {
				break
			}
			position++
		}
		start := position
		for position < len(data) && data[position] >= '0' && data[position] <= '9' {
			position++
		}
		value, err := strconv.Atoi(string(data[start:position]))
		if err != nil {
			return fields, nil, fmt.Errorf("malformed PPM header")
		}
		fields[index] = value
	}
	if position >= len(data) || !isPPMSpace(data[position]) {
		return fields, nil, fmt.Errorf("malformed PPM header")
	}
	return fields, data[position+1:], nil
}

func isPPMSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}
