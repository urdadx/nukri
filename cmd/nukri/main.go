package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/internal/config/icons"
	"github.com/urdadx/nukri/internal/kittydnd"
	"github.com/urdadx/nukri/internal/theme"
	"github.com/urdadx/nukri/internal/ui"
	nukriconfig "github.com/urdadx/nukri/nukri_config"
)

func main() {
	os.Exit(run())
}

// run starts the UI and returns the exit code. Errors return rather than exit,
// so the deferred terminal cleanup always runs.
func run() int {
	userThemes := filepath.Join(xdg.ConfigHome, "nukri", "themes")
	themeName := flag.String("theme", "catppuccin-mocha", "theme name: a bundled theme, or <name>.toml in "+userThemes)
	flag.Parse()
	selectedTheme, err := theme.Named(*themeName, userThemes, nukriconfig.Themes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nukri:", err)
		return 1
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
		fmt.Fprintln(os.Stderr, "nukri:", err)
		return 1
	}
	return 0
}
