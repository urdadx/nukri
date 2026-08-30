package browser

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/urdadx/nukri/internal/core"
	fileinfo "github.com/urdadx/nukri/internal/file_info"
	"github.com/urdadx/nukri/internal/preview"
)

type Entry struct {
	Entry     core.Entry
	Facts     fileinfo.FileFacts
	Icon      string
	IconColor string
	ItemCount *int
}

type Data struct {
	CWD             string
	Places          []core.SidebarRow
	Entries         []Entry
	Selected        int
	Preview         preview.View
	PreviewEntries  []Entry
	PreviewIsDir    bool
	PreviewSelected int
	PreviewOffset   int
	PreviewLoading  bool
	PreviewError    string
	LoadError       string
}

type Styles struct {
	RootFG, RootBG, PanelFG, PanelBG, PanelBorder, ActiveBorder    lipgloss.Color
	Path, Directory, SelectedFG, SelectedBG, Muted                 lipgloss.Color
	SidebarFG, SidebarBG, SidebarTitle, SidebarBorder, SidebarIcon lipgloss.Color
	SidebarSelectedFG, SidebarSelectedBG, Cursor                   lipgloss.Color
}

func Render(width, height int, styles Styles, data Data) string {
	layout := ResolveLayout(width, height)
	if layout.FilesWidth == width {
		return RenderEntries(width, height, styles, data)
	}
	if layout.Stacked {
		content := lipgloss.JoinVertical(lipgloss.Left,
			RenderEntries(layout.FilesWidth, layout.FilesHeight, styles, data),
			RenderPreview(layout.PreviewWidth, layout.PreviewHeight, styles, data),
		)
		return lipgloss.JoinHorizontal(lipgloss.Top, RenderSidebar(layout.SidebarWidth, height, styles, data), content)
	}
	parts := []string{RenderSidebar(layout.SidebarWidth, height, styles, data), RenderEntries(layout.FilesWidth, height, styles, data)}
	if layout.PreviewWidth > 0 {
		parts = append(parts, RenderPreview(layout.PreviewWidth, height, styles, data))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
