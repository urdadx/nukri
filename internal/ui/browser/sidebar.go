package browser

import (
	"strings"

	"github.com/urdadx/nukri/internal/core"

	"github.com/charmbracelet/lipgloss"
)

// sidebarCompact reports whether the sidebar is too narrow for titles and
// section headings, showing icons only.
func sidebarCompact(width int) bool { return width <= 5 }

// SidebarPathAt returns the path of the place drawn on content row row of a
// sidebar width cells wide, or "" when that row holds no place.
func SidebarPathAt(width int, places []core.SidebarRow, row int) string {
	compact := sidebarCompact(width)
	rendered := 0
	for _, place := range places {
		if place.Section != "" {
			if !compact {
				rendered++
			}
			continue
		}
		item := place.Item
		if item == nil {
			continue // not drawn, see RenderSidebar
		}
		if rendered == row {
			return item.Path
		}
		rendered++
	}
	return ""
}

func RenderSidebar(width, height int, s Styles, data Data) string {
	compact := sidebarCompact(width)
	content := make([]string, 0, len(data.Places)+1)
	sectionStyle := lipgloss.NewStyle().Foreground(s.SidebarTitle).Background(s.SidebarBG).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(s.SidebarFG).Background(s.SidebarBG)
	selectedStyle := rowStyle.Foreground(s.SidebarSelectedFG).Background(s.SidebarSelectedBG).Bold(true)
	iconStyle := lipgloss.NewStyle().Foreground(s.SidebarIcon).Background(s.SidebarBG)
	selectedIconStyle := iconStyle.Background(s.SidebarSelectedBG).Bold(true)
	rowWidth := max(1, width-4)
	for _, row := range data.Places {
		if row.Section != "" {
			if !compact {
				content = append(content, sectionStyle.Render(" "+row.Section))
			}
			continue
		}
		item := row.Item
		if item == nil {
			continue
		}
		prefix := " "
		text := ""
		if !compact {
			text = "  " + SafeName(item.Title)
		}
		textStyle := rowStyle
		placeIconStyle := iconStyle
		if item.Path == data.CWD {
			prefix = " "
			textStyle = selectedStyle
			placeIconStyle = selectedIconStyle
		}
		used := lipgloss.Width(prefix) + lipgloss.Width(item.Icon) + lipgloss.Width(text)
		filler := strings.Repeat(" ", max(0, rowWidth-used))
		content = append(content, textStyle.Render(prefix)+placeIconStyle.Render(item.Icon)+textStyle.Render(text+filler))
	}
	title := " Places "
	if compact {
		title = ""
	}
	return panel(strings.Join(content, "\n"), title, width, height, s.SidebarFG, s.SidebarBG, s.SidebarBorder, s.SidebarTitle)
}
