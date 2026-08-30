package browser

import (
	"math"
	"testing"
)

func TestFitImageToPanePreservesAspect(t *testing.T) {
	cellAspect := 2.0
	tests := []struct {
		name                          string
		imgW, imgH, maxCols, maxRows int
	}{
		{"square in wide pane", 100, 100, 40, 20},
		{"landscape in wide pane", 200, 100, 40, 20},
		{"portrait in wide pane", 100, 200, 40, 20},
		{"landscape in short pane", 400, 100, 40, 10},
		{"portrait in short pane", 100, 500, 30, 10},
		{"small in large pane", 50, 25, 40, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, rows := fitImageToPane(tt.imgW, tt.imgH, tt.maxCols, tt.maxRows, cellAspect)
			if cols <= 0 || rows <= 0 {
				t.Fatalf("got non-positive placement %dx%d", cols, rows)
			}
			if cols > tt.maxCols || rows > tt.maxRows {
				t.Fatalf("placement %dx%d exceeds pane %dx%d", cols, rows, tt.maxCols, tt.maxRows)
			}
			// Placed pixel-space aspect (cellHeight = cellAspect x cellWidth)
			// must match the image's pixel aspect.
			imageAspect := float64(tt.imgW) / float64(tt.imgH)
			placedAspect := float64(cols) / (float64(rows) * cellAspect)
			if math.Abs(placedAspect-imageAspect) > 0.01 {
				t.Errorf("aspect mismatch: placed=%v want=%v (cols=%d rows=%d)", placedAspect, imageAspect, cols, rows)
			}
		})
	}
}
