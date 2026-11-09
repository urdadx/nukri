package theme

import (
	"math"
	"strconv"
	"strings"
)

// minimumContrast is the lowest text-to-background contrast ratio a selected
// row may have: 3:1, the WCAG minimum for large or bold text, which selected
// rows are.
const minimumContrast = 3.0

/*
ensureReadableSelection fixes selected rows whose text would vanish into their
background, as when a theme gives both the same color. The text color is
replaced with the theme's most readable color for that background, or black or
white when none of the theme's colors is readable enough.
*/
func (t *Theme) ensureReadableSelection() {
	t.FilePanelItemSelectedFG = t.readableOn(t.FilePanelItemSelectedFG, t.FilePanelItemSelectedBG)
	t.SidebarItemSelectedFG = t.readableOn(t.SidebarItemSelectedFG, t.SidebarItemSelectedBG)
}

func (t *Theme) readableOn(foreground, background string) string {
	if background == "" {
		background = t.FilePanelBG // an empty background shows the panel's
	}
	if contrastRatio(foreground, background) >= minimumContrast {
		return foreground
	}
	best, bestRatio := foreground, contrastRatio(foreground, background)
	for _, candidate := range []string{t.FilePanelBG, t.FullScreenBG, t.FilePanelFG, t.FullScreenFG, "#000000", "#ffffff"} {
		if _, ok := relativeLuminance(candidate); !ok {
			continue // unset or not a hex color
		}
		// On the panel's own background, the panel's text color would make the
		// selected row look like every other row.
		if strings.EqualFold(background, t.FilePanelBG) && (strings.EqualFold(candidate, t.FilePanelFG) || strings.EqualFold(candidate, t.FullScreenFG)) {
			continue
		}
		if ratio := contrastRatio(candidate, background); ratio > bestRatio {
			best, bestRatio = candidate, ratio
			if ratio >= 4.5 { // comfortably readable; prefer the theme's own color
				break
			}
		}
	}
	return best
}

// contrastRatio is the WCAG contrast ratio of two "#rrggbb" colors, from 1
// (identical) to 21 (black on white). When either color can't be judged, such
// as an unset one that leaves the terminal's own color, it reports infinity so
// the pair is left as it is.
func contrastRatio(first, second string) float64 {
	a, okA := relativeLuminance(first)
	b, okB := relativeLuminance(second)
	if !okA || !okB {
		return math.Inf(1)
	}
	return (max(a, b) + 0.05) / (min(a, b) + 0.05)
}

func relativeLuminance(color string) (float64, bool) {
	hex := strings.TrimPrefix(color, "#")
	if len(hex) != 6 {
		return 0, false
	}
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, false
	}
	channel := func(shift uint) float64 {
		c := float64(value>>shift&0xff) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(16) + 0.7152*channel(8) + 0.0722*channel(0), true
}
