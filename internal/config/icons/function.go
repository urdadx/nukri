// this code is from https://github.com/yorukot/superfile/blob/main/src/config/icon/function.go

package icons

const noIconColor = "NONE"

// InitIcon initializes the icon configuration for the application.
// It sets up different icons based on whether nerd fonts are enabled and configures directory icon colors.
//
// Parameters:
//   - nerdfont: boolean flag to determine if nerd fonts should be used
//     When false, uses simple ASCII characters for icons
//     When true, uses nerd font icons (default behavior)
//   - directoryIconColor: string representing the color for directory icons
//     If empty, defaults to "NONE" (dark yellowish)
//
// The function configures various icons for:
//   - System directories (Home, Download, Documents, etc.)
//   - File operations (Compress, Extract, Copy, Cut, Delete)
//   - UI elements (Cursor, Browser, Select, etc.)
//   - Status indicators (Error, Warn, Done, InOperation)
//   - Navigation and sorting (Directory, Search, SortAsc, SortDesc)
func InitIcon(nerdfont bool, directoryIconColor string) {
	resetNerdFontIcons()

	// Make sure that these alternatives are ASCII characters only.
	// Dont place any special unicode characters here.
	if !nerdfont {
		// When nerdfont is disabled, we use simple ASCII characters
		// Space is set to empty string because we don't need special spacing
		// for ASCII characters, unlike nerd fonts which often need proper spacing
		// to display correctly
		Space = ""
		SuperfileIcon = ""

		Home = ""
		Desktop = ""
		Downloads = ""
		Documents = ""
		Pictures = ""
		Videos = ""
		Music = ""
		Templates = ""
		PublicShare = ""
		Trash = ""
		Root = ""
		CustomPlace = ""
		LinkedPlace = ""
		BrokenPlace = ""
		FixedDevice = ""
		RemovableDevice = ""

		// file operations
		CompressFile = ""
		ExtractFile = ""
		Copy = ""
		Cut = ""
		Delete = ""

		// other
		Cursor = ">"
		Browser = "B"
		Select = "S"
		CheckboxEmpty = "[ ]"
		CheckboxChecked = "[x]"
		Error = ""
		Warn = ""
		Done = ""
		InOperation = ""
		Directory = ""
		Search = ""
		SortAsc = "^"
		SortDesc = "v"
		Terminal = ""
		Pinned = ""
		Disk = ""
	}

	if directoryIconColor == "" {
		directoryIconColor = noIconColor // Dark yellowish
	}
	folderIcon := Directory
	Folders["folder"] = Style{
		Icon:  folderIcon,
		Color: directoryIconColor,
	}
}

func resetNerdFontIcons() {
	Space = " "
	SuperfileIcon = "\ue6ad"
	Home = "\U000f02dc"
	Desktop = "\U000f01c4"
	Downloads = "\U000f03d4"
	Documents = "\U000f0219"
	Pictures = "\U000f02e9"
	Videos = "\U000f0381"
	Music = "♬"
	Templates = "\U000f03e2"
	PublicShare = "\uf0ac"
	Trash = "\uf1f8"
	Root = "\U000f02ca"
	CustomPlace = "\U000f024b"
	LinkedPlace = "\uf482"
	BrokenPlace = "\U000f033a"
	FixedDevice = "\U000f02ca"
	RemovableDevice = "\U000f0553"
	CompressFile = "\U000f05c4"
	ExtractFile = "\U000f06eb"
	Copy = "\U000f018f"
	Cut = "\U000f0190"
	Delete = "\U000f01b4"
	Cursor = "\uf054"
	Browser = "\U000f0208"
	Select = "\U000f01bd"
	CheckboxEmpty = "\U000f0131"
	CheckboxChecked = "\U000f0856"
	Error = "\uf530"
	Warn = "\uf071"
	Done = "\uf4a4"
	InOperation = "\U000f0954"
	Directory = "\uf07b"
	Search = "\ue68f"
	SortAsc = "\uf0de"
	SortDesc = "\uf0dd"
	Terminal = "\ue795"
	Pinned = "\U000f0403"
	Disk = "\U000f11f0"
}

func GetCopyOrCutIcon(cut bool) string {
	if cut {
		return Cut
	}
	return Copy
}
