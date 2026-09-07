package browser

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func RenderList(width, height int, s Styles, data Data) string {
	if data.LoadError != "" {
		return lipgloss.NewStyle().Foreground(s.Muted).Background(s.PanelBG).Render(data.LoadError)
	}
	if len(data.Entries) == 0 {
		return lipgloss.NewStyle().Foreground(s.Muted).Background(s.PanelBG).Render("This folder is empty")
	}
	start := 0
	if data.Selected >= height {
		start = data.Selected - height + 1
	}
	end := min(len(data.Entries), start+height)
	rows := make([]string, 0, max(0, end-start))
	rowStyle := lipgloss.NewStyle().Foreground(s.PanelFG).Background(s.PanelBG)
	selectedStyle := rowStyle.Foreground(s.SelectedFG).Background(s.SelectedBG).Bold(true)
	metadataStyle := rowStyle.Foreground(s.SidebarFG)
	nameColumn := max(1, width-27)
	for i := start; i < end; i++ {
		item := data.Entries[i]
		entry := item.Entry
		metadata := ""
		nameWidth := width - 4
		if width >= 34 {
			nameWidth = nameColumn
			detail := formatSize(entry.Size)
			if entry.IsDirectory() {
				detail = "folder"
				if item.ItemCount != nil {
					detail = fmt.Sprintf("%d items", *item.ItemCount)
				}
			}
			metadata = fmt.Sprintf("%10s %9s", detail, formatModified(entry.Modified))
		}
		name := truncate(entry.Name, max(1, nameWidth))
		prefix := "  "
		nameText := fmt.Sprintf(" %-*s", max(1, nameWidth), name)
		iconStyle := rowStyle
		if item.IconColor != "" && item.IconColor != "NONE" {
			iconStyle = iconStyle.Foreground(lipgloss.Color(item.IconColor))
		}
		if i == data.Selected {
			row := "▌ " + item.Icon + nameText + metadata
			row = truncate(row, width)
			row += strings.Repeat(" ", max(0, width-lipgloss.Width(row)))
			rows = append(rows, selectedStyle.Render(row))
			continue
		}
		used := lipgloss.Width(prefix) + lipgloss.Width(item.Icon) + lipgloss.Width(nameText) + lipgloss.Width(metadata)
		filler := strings.Repeat(" ", max(0, width-used))
		rows = append(rows, metadataStyle.Render(prefix)+iconStyle.Render(item.Icon)+metadataStyle.Render(nameText+metadata+filler))
	}
	return strings.Join(rows, "\n")
}

func formatSize(size int64) string {
	if size < 1000 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"kB", "MB", "GB", "TB"}
	value := float64(size)
	for _, unit := range units {
		value /= 1000
		if value < 1000 || unit == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%d B", size)
}

func formatModified(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	now := time.Now()
	age := now.Sub(value)
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	}
	months := (now.Year()-value.Year())*12 + int(now.Month()-value.Month())
	if months > 0 && now.Before(value.AddDate(0, months, 0)) {
		months--
	}
	if months > 0 {
		return fmt.Sprintf("%dmo ago", months)
	}
	return fmt.Sprintf("%dd ago", int(age.Hours()/24))
}
