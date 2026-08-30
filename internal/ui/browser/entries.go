package browser

func RenderEntries(width, height int, s Styles, data Data) string {
	title := data.CWD
	if title == "" {
		title = "Loading..."
	}
	return panel(RenderList(width-4, height-4, s, data), " 󰉖  "+truncate(title, max(1, width-10))+" ", width, height, s.PanelFG, s.PanelBG, s.ActiveBorder, s.Path)
}
