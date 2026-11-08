// this code is from https://github.com/yorukot/superfile/blob/main/src/config/icon/function.go

package icons

const noIconColor = "NONE"

func init() { InitIcon(true, "") }

/*
InitIcon sets the place and folder icons. With nerdfont false every icon is
empty, for terminals without a Nerd Font. directoryIconColor colors the generic
folder icon; empty means no color.
*/
func InitIcon(nerdfont bool, directoryIconColor string) {
	icon := func(glyph string) string {
		if nerdfont {
			return glyph
		}
		return ""
	}
	Home = icon("\U000f02dc")            // 󰋜
	Desktop = icon("\U000f01c4")         // 󰇄
	Downloads = icon("\U000f03d4")       // 󰏔
	Documents = icon("\U000f0219")       // 󰈙
	Pictures = icon("\U000f02e9")        // 󰋩
	Videos = icon("\U000f0381")          // 󰎁
	Music = icon("♬")                    // ♬
	Trash = icon("\uf1f8")               //
	Root = icon("\U000f02ca")            // 󰋊
	CustomPlace = icon("\U000f024b")     // 󰉋
	LinkedPlace = icon("\uf482")         //
	BrokenPlace = icon("\U000f033a")     // 󰌺
	FixedDevice = icon("\U000f02ca")     // 󰋊
	RemovableDevice = icon("\U000f0553") // 󰕓
	Directory = icon("\uf07b")           //

	if directoryIconColor == "" {
		directoryIconColor = noIconColor
	}
	Folders["folder"] = Style{Icon: Directory, Color: directoryIconColor}
}
