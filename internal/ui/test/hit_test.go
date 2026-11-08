package test

import (
	"testing"

	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/ui/browser"
)

func TestSidebarPathAtMatchesDrawnRows(t *testing.T) {
	home := core.NewSidebarItem(core.Home, "Home", "h", "/home/me", "/home/me")
	root := core.NewSidebarItem(core.Root, "Root", "r", "/", "/")
	places := []core.SidebarRow{
		{Section: "Places"},
		{Item: &home},
		{}, // no item: not drawn, so it must not take up a row
		{Section: "Devices"},
		{Item: &root},
	}
	// Full width: section headings take rows.
	for row, want := range []string{"", "/home/me", "", "/"} {
		if got := browser.SidebarPathAt(23, places, row); got != want {
			t.Errorf("wide row %d = %q, want %q", row, got, want)
		}
	}
	// Compact: icons only, no headings.
	for row, want := range []string{"/home/me", "/", ""} {
		if got := browser.SidebarPathAt(5, places, row); got != want {
			t.Errorf("compact row %d = %q, want %q", row, got, want)
		}
	}
}

func TestListWindowStartKeepsSelectionVisible(t *testing.T) {
	for _, test := range []struct{ selected, visible, want int }{
		{0, 10, 0}, {9, 10, 0}, {10, 10, 1}, {25, 10, 16},
	} {
		if got := browser.ListWindowStart(test.selected, test.visible); got != test.want {
			t.Errorf("ListWindowStart(%d, %d) = %d, want %d", test.selected, test.visible, got, test.want)
		}
	}
}

func TestSafeNameHidesControlCharacters(t *testing.T) {
	if got := browser.SafeName("evil\r\x1b[2Jname\t.txt"); got != "evil??[2Jname?.txt" {
		t.Fatalf("SafeName = %q", got)
	}
	if got := browser.SafeName("plain é name.txt"); got != "plain é name.txt" {
		t.Fatalf("SafeName changed a normal name: %q", got)
	}
}
