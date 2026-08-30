package browser

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func RenderPreview(width, height int, s Styles, data Data) string {
	bodyWidth := max(1, width-4)
	muted := lipgloss.NewStyle().Foreground(s.Muted).Background(s.PanelBG)
	title := " Preview "
	text := []string{"Nothing selected"}
	if data.Selected >= 0 && data.Selected < len(data.Entries) {
		entry := data.Entries[data.Selected]
		title = " " + entry.Icon + "  " + entry.Entry.Name + " "
		switch {
		case data.PreviewLoading:
			text = []string{muted.Render("Loading preview...")}
		case data.PreviewError != "":
			text = []string{muted.Render(data.PreviewError)}
		default:
			text = append([]string{muted.Render(joinDetail(data.Preview.Title, data.Preview.Detail)), ""}, data.Preview.Lines...)
			if data.Preview.Visual != nil {
				text = append(text, "", muted.Render("Visual preview available"))
			}
			if data.Preview.Footer != "" {
				text = append(text, "", muted.Render(data.Preview.Footer))
			}
		}
	}
	for i := range text {
		text[i] = truncate(text[i], bodyWidth)
	}
	return panel(strings.Join(text, "\n"), truncate(title, max(1, width-6)), width, height, s.PanelFG, s.PanelBG, s.PanelBorder, s.Path)
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
