package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/config/icons"
	"github.com/urdadx/nukri/internal/kittydnd"
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
	model := ui.New(selectedTheme)
	options := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	var input *kittydnd.Input
	if runtime := kittydnd.DetectRuntime(); runtime.Enabled {
		input, err = kittydnd.OpenInput()
		if err == nil {
			defer input.Close()
			options = append(options, tea.WithInput(input))
			model = model.WithDragOutput(os.Stdout)
			_, _ = os.Stdout.WriteString(kittydnd.EnableSequence(runtime.MachineID))
			defer os.Stdout.WriteString(kittydnd.DisableSequence())
		}
	}
	program := tea.NewProgram(model, options...)
	if input != nil {
		go func() {
			for event := range input.Events() {
				program.Send(event)
			}
		}()
	}
	if _, err := program.Run(); err != nil {
		log.Fatal(err)
	}
}
