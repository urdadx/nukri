package main

import (
	"flag"
	"log"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/config/icons"
	"github.com/urdadx/nukri/internal/theme"
	"github.com/urdadx/nukri/internal/ui"
)

func main() {
	themeName := flag.String("theme", "catppuccin-mocha", "theme name from nukri_config/themes")
	flag.Parse()
	themePath := filepath.Join("nukri_config", "themes", *themeName+".toml")
	selectedTheme, err := theme.Load(themePath)
	if err != nil {
		log.Fatal(err)
	}
	icons.InitIcon(true, selectedTheme.DirectoryIconColor)
	if _, err := tea.NewProgram(ui.New(selectedTheme), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		log.Fatal(err)
	}
}
