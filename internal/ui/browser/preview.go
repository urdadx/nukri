package browser

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func RenderPreview(width, height, originX, originY int, visualState *VisualState, visualWriter io.Writer, s Styles, data Data) string {
	bodyWidth := max(1, width-4)
	muted := lipgloss.NewStyle().Foreground(s.Muted).Background(s.PanelBG)
	title := " Preview "
	text := []string{"Nothing selected"}
	showingVisual := false
	if data.Selected >= 0 && data.Selected < len(data.Entries) {
		entry := data.Entries[data.Selected]
		title = " " + entry.Icon + "  " + entry.Entry.Name + " "
		switch {
		case data.PreviewLoading:
			text = []string{muted.Render("Loading preview...")}
		case data.PreviewError != "":
			text = []string{muted.Render(data.PreviewError)}
		case data.PreviewIsDir:
			text = renderPreviewEntries(data.PreviewEntries, data.PreviewSelected, bodyWidth, s)
			if len(text) == 0 {
				text = []string{muted.Render("Folder is empty")}
			}
		default:
			text = append([]string{muted.Render(joinDetail(data.Preview.Title, data.Preview.Detail)), ""}, data.Preview.Lines...)
			if data.Preview.Visual != nil && visualState != nil {
				showingVisual = true
				columns, rows := fitImageToPane(data.Preview.Visual.Width, data.Preview.Visual.Height, max(1, bodyWidth), max(1, height-4), defaultCellAspect)
				visualState.renderVisual(visualWriter, data.Preview.Visual, originX, originY, columns, rows)
			}
			if data.Preview.Footer != "" {
				text = append(text, "", muted.Render(data.Preview.Footer))
			}
		}
	}
	if !showingVisual && visualState != nil {
		visualState.Clear()
	}
	visibleLines := max(0, height-4)
	totalLines := len(text)
	maxOffset := max(0, totalLines-visibleLines)
	offset := min(max(data.PreviewOffset, 0), maxOffset)
	end := min(len(text), offset+visibleLines)
	text = text[offset:end]
	lineStyle := lipgloss.NewStyle().Foreground(s.PanelFG).Background(s.PanelBG)
	backgroundPrefix := stylePrefix(lineStyle)
	for i := range text {
		text[i] = truncate(text[i], bodyWidth)
		text[i] = strings.ReplaceAll(text[i], "\x1b[0m", "\x1b[0m"+backgroundPrefix)
		text[i] += strings.Repeat(" ", max(0, bodyWidth-lipgloss.Width(text[i])))
		text[i] = lineStyle.Render(text[i])
	}
	return panel(strings.Join(text, "\n"), truncate(title, max(1, width-6)), width, height, s.PanelFG, s.PanelBG, s.PanelBorder, s.Path)
}

func stylePrefix(style lipgloss.Style) string {
	rendered := style.Render("x")
	if index := strings.IndexByte(rendered, 'x'); index >= 0 {
		return rendered[:index]
	}
	return ""
}

func renderPreviewEntries(entries []Entry, selected, width int, s Styles) []string {
	rows := make([]string, 0, len(entries))
	textStyle := lipgloss.NewStyle().Foreground(s.SidebarFG).Background(s.PanelBG)
	baseIconStyle := lipgloss.NewStyle().Foreground(s.Directory).Background(s.PanelBG)
	selectedStyle := lipgloss.NewStyle().Foreground(s.SelectedFG).Background(s.SelectedBG).Bold(true)
	for index, item := range entries {
		iconStyle := baseIconStyle
		if item.IconColor != "" && item.IconColor != "NONE" {
			iconStyle = iconStyle.Foreground(lipgloss.Color(item.IconColor))
		}
		detail := formatSize(item.Entry.Size)
		if item.Entry.IsDirectory() {
			detail = "folder"
			if item.ItemCount != nil {
				detail = fmt.Sprintf("%d items", *item.ItemCount)
			}
		}
		nameWidth := max(1, width-lipgloss.Width(detail)-5)
		name := truncate(item.Entry.Name, nameWidth)
		gap := strings.Repeat(" ", max(1, width-lipgloss.Width(item.Icon)-lipgloss.Width(name)-lipgloss.Width(detail)-2))
		if index == selected {
			row := item.Icon + " " + name + gap + detail
			row = truncate(row, width)
			row += strings.Repeat(" ", max(0, width-lipgloss.Width(row)))
			rows = append(rows, selectedStyle.Render(row))
			continue
		}
		rows = append(rows, textStyle.Render(" ")+iconStyle.Render(item.Icon)+textStyle.Render(" "+name+gap+detail))
	}
	return rows
}

func joinDetail(left, right string) string {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	return left + " • " + right
}
