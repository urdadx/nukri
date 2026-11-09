package theme

import (
	"fmt"
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
	rgb, ok := parseHex(color)
	if !ok {
		return 0, false
	}
	channel := func(value float64) float64 {
		c := value / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(rgb[0]) + 0.7152*channel(rgb[1]) + 0.0722*channel(rgb[2]), true
}

// Contrast targets for pop-up text, from WCAG: 4.5:1 keeps normal text
// readable, 7:1 makes it stand out.
const (
	readableText  = 4.5
	prominentText = 7.0
)

// ModalTitle is the color for pop-up titles: the theme's accent when it stands
// out clearly on the pop-up background, else the pop-up's text color.
func (t Theme) ModalTitle() string {
	if contrastRatio(t.ModalBorderActive, t.ModalBG) >= prominentText {
		return t.ModalBorderActive
	}
	return t.ModalFG
}

// ModalMuted is the color for secondary text in pop-ups, such as details and
// key hints: the theme's divider color, brightened toward the text color until
// it is readable.
func (t Theme) ModalMuted() string { return legibleOn(t.SidebarDivider, t.ModalFG, t.ModalBG) }

// ModalError is the color for errors in pop-ups, made readable the same way.
func (t Theme) ModalError() string { return legibleOn(t.Cancel, t.ModalFG, t.ModalBG) }

/*
legibleOn returns color if it is readable on background, else color mixed
toward toward, in tenths, until it is. Mixing keeps the color's character,
so muted text still looks secondary. Colors that can't be judged are kept.
*/
func legibleOn(color, toward, background string) string {
	if contrastRatio(color, background) >= readableText {
		return color
	}
	from, okFrom := parseHex(color)
	to, okTo := parseHex(toward)
	if !okFrom || !okTo {
		return color
	}
	for step := 1; step < 10; step++ {
		var mixed [3]float64
		for channel := range mixed {
			mixed[channel] = from[channel] + (to[channel]-from[channel])*float64(step)/10
		}
		candidate := fmt.Sprintf("#%02x%02x%02x", int(math.Round(mixed[0])), int(math.Round(mixed[1])), int(math.Round(mixed[2])))
		if contrastRatio(candidate, background) >= readableText {
			return candidate
		}
	}
	return toward
}

// parseHex splits a "#rrggbb" color into its channels.
func parseHex(color string) ([3]float64, bool) {
	hex := strings.TrimPrefix(color, "#")
	if len(hex) != 6 {
		return [3]float64{}, false
	}
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return [3]float64{}, false
	}
	return [3]float64{float64(value >> 16 & 0xff), float64(value >> 8 & 0xff), float64(value & 0xff)}, true
}
