package browser

type Layout struct {
	SidebarWidth, FilesWidth, PreviewWidth int
	FilesHeight, PreviewHeight             int
	Stacked                                bool
}

func ResolveLayout(width, height int) Layout {
	if width < 34 {
		return Layout{FilesWidth: width, FilesHeight: height}
	}
	sidebar := 23
	if width < 104 {
		sidebar -= (104 - width) / 4
	}
	if sidebar < 11 {
		sidebar = 5
	}
	remaining := width - sidebar
	if width <= 54 {
		filesHeight := height * 54 / 100
		if filesHeight < 6 {
			return Layout{SidebarWidth: sidebar, FilesWidth: remaining, FilesHeight: height}
		}
		return Layout{SidebarWidth: sidebar, FilesWidth: remaining, PreviewWidth: remaining, FilesHeight: filesHeight, PreviewHeight: height - filesHeight, Stacked: true}
	}
	files := remaining * 54 / 100
	preview := remaining - files
	if files < 18 || preview < 14 {
		return Layout{SidebarWidth: sidebar, FilesWidth: remaining, FilesHeight: height}
	}
	return Layout{SidebarWidth: sidebar, FilesWidth: files, PreviewWidth: preview, FilesHeight: height, PreviewHeight: height}
}
