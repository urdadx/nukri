// this code is from https://github.com/yorukot/superfile/blob/main/src/config/icon/function.go

package icons

import "sync/atomic"

const noIconColor = "NONE"

/*
Set holds the place icons and the generic folder style for one icon mode.
A Set is never changed after InitIcon stores it, so preview workers can read
it while the theme picker swaps in a new one.
*/
type Set struct {
	Home, Desktop, Downloads, Documents, Pictures, Videos, Music string
	Trash, Root, CustomPlace, LinkedPlace, BrokenPlace           string
	FixedDevice, RemovableDevice, Directory                      string
	Folder                                                       Style // generic folder icon
}

var current atomic.Pointer[Set]

func init() { InitIcon(true, "") }

// Current returns the icon set in use. Callers must not modify it.
func Current() *Set { return current.Load() }

/*
InitIcon sets the place and folder icons. With nerdfont false every icon is
empty, for terminals without a Nerd Font. directoryIconColor colors the generic
folder icon; empty means no color. It is safe to call while icons are read.
*/
func InitIcon(nerdfont bool, directoryIconColor string) {
	icon := func(glyph string) string {
		if nerdfont {
			return glyph
		}
		return ""
	}
	set := &Set{
		Home:            icon("\U000f02dc"), // 󰋜
		Desktop:         icon("\U000f01c4"), // 󰇄
		Downloads:       icon("\U000f03d4"), // 󰏔
		Documents:       icon("\U000f0219"), // 󰈙
		Pictures:        icon("\U000f02e9"), // 󰋩
		Videos:          icon("\U000f0381"), // 󰎁
		Music:           icon("♬"),          // ♬
		Trash:           icon(""),          //
		Root:            icon("\U000f02ca"), // 󰋊
		CustomPlace:     icon("\U000f024b"), // 󰉋
		LinkedPlace:     icon(""),          //
		BrokenPlace:     icon("\U000f033a"), // 󰌺
		FixedDevice:     icon("\U000f02ca"), // 󰋊
		RemovableDevice: icon("\U000f0553"), // 󰕓
		Directory:       icon(""),          //
	}
	if directoryIconColor == "" {
		directoryIconColor = noIconColor
	}
	set.Folder = Style{Icon: set.Directory, Color: directoryIconColor}
	current.Store(set)
}
