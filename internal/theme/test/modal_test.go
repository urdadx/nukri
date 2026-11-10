package test

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/urdadx/nukri/configs"
	"github.com/urdadx/nukri/internal/theme"
)

// contrast is the WCAG contrast ratio of two "#rrggbb" colors.
func contrast(t *testing.T, first, second string) float64 {
	t.Helper()
	luminance := func(color string) float64 {
		value, err := strconv.ParseUint(strings.TrimPrefix(color, "#"), 16, 32)
		if err != nil || len(color) != 7 {
			t.Fatalf("not a #rrggbb color: %q", color)
		}
		channel := func(shift uint) float64 {
			c := float64(value>>shift&0xff) / 255
			if c <= 0.03928 {
				return c / 12.92
			}
			return math.Pow((c+0.055)/1.055, 2.4)
		}
		return 0.2126*channel(16) + 0.7152*channel(8) + 0.0722*channel(0)
	}
	a, b := luminance(first), luminance(second)
	return (max(a, b) + 0.05) / (min(a, b) + 0.05)
}

// Every bundled theme's pop-ups must be readable: secondary text and errors at
// 4.5:1 or better, and titles at least as clear as the pop-up's text.
func TestEveryThemeHasReadablePopups(t *testing.T) {
	for _, name := range theme.Names("", configs.Themes) {
		loaded, err := theme.Named(name, "", configs.Themes)
		if err != nil {
			t.Fatal(err)
		}
		background := loaded.ModalBG
		if got := contrast(t, loaded.ModalMuted(), background); got < 4.5 {
			t.Errorf("%s: secondary pop-up text contrast %.2f, want 4.5", name, got)
		}
		if got := contrast(t, loaded.ModalError(), background); got < 4.5 {
			t.Errorf("%s: pop-up error contrast %.2f, want 4.5", name, got)
		}
		if got, text := contrast(t, loaded.ModalTitle(), background), contrast(t, loaded.ModalFG, background); got < min(text, 7) {
			t.Errorf("%s: pop-up title contrast %.2f, want %.2f", name, got, min(text, 7))
		}
	}
}

func TestPopupColorsKeepReadableThemeColors(t *testing.T) {
	bright := theme.Theme{ModalFG: "#ffffff", ModalBG: "#000000", ModalBorderActive: "#ffcc00", SidebarDivider: "#999999", Cancel: "#ff5555"}
	if bright.ModalTitle() != "#ffcc00" || bright.ModalMuted() != "#999999" || bright.ModalError() != "#ff5555" {
		t.Fatalf("readable colors were replaced: title %s, muted %s, error %s", bright.ModalTitle(), bright.ModalMuted(), bright.ModalError())
	}
}

// A dim color is brightened toward the text color only as far as needed, so
// it still looks secondary.
func TestPopupMutedTextIsBrightenedJustEnough(t *testing.T) {
	dim := theme.Theme{ModalFG: "#ffffff", ModalBG: "#000000", ModalBorderActive: "#333333", SidebarDivider: "#333333"}
	muted := dim.ModalMuted()
	if got := contrast(t, muted, "#000000"); got < 4.5 || muted == "#ffffff" {
		t.Fatalf("muted = %s (contrast %.2f), want readable but not the text color", muted, got)
	}
	if dim.ModalTitle() != "#ffffff" {
		t.Fatalf("title = %s, want the text color in place of a dim accent", dim.ModalTitle())
	}
}

// Colors that can't be judged, such as unset ones, are left alone.
func TestPopupColorsKeepUnsetColors(t *testing.T) {
	unset := theme.Theme{ModalFG: "#ffffff", SidebarDivider: "#333333"}
	if unset.ModalMuted() != "#333333" {
		t.Fatalf("muted = %s with no background to judge against", unset.ModalMuted())
	}
}
