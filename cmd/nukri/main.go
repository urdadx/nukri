package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/adrg/xdg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/urdadx/nukri/configs"
	"github.com/urdadx/nukri/internal/config"
	"github.com/urdadx/nukri/internal/config/icons"
	"github.com/urdadx/nukri/internal/kittydnd"
	"github.com/urdadx/nukri/internal/theme"
	"github.com/urdadx/nukri/internal/ui"
)

const defaultTheme = "catppuccin-mocha"

// version is set by release builds (-ldflags "-X main.version=...").
var version = ""

// versionString is the release version, else the version Go recorded at build
// time (the module version for go install, a git-based one in a checkout),
// else "dev".
func versionString() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func main() {
	os.Exit(run())
}

// run starts the UI and returns the exit code. Errors return rather than exit,
// so the deferred terminal cleanup always runs.
func run() int {
	configDir := filepath.Join(xdg.ConfigHome, "nukri")
	configPath := filepath.Join(configDir, "config.toml")
	userThemes := filepath.Join(configDir, "themes")
	themeFlag := flag.String("theme", "", "theme name: a bundled theme, or <name>.toml in "+userThemes+
		" (default: the theme in "+configPath+", else "+defaultTheme+")")
	versionFlag := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println("nukri", versionString())
		return 0
	}

	settings, err := config.LoadSettings(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nukri:", err)
		return 1
	}
	// The flag wins, then the saved choice, then the default.
	themeName := firstNonEmpty(*themeFlag, settings.Theme, defaultTheme)
	selectedTheme, err := theme.Named(themeName, userThemes, configs.Themes)
	if err != nil && *themeFlag == "" && themeName != defaultTheme {
		// A saved theme that no longer exists shouldn't stop nukri starting.
		themeName = defaultTheme
		selectedTheme, err = theme.Named(themeName, userThemes, configs.Themes)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "nukri:", err)
		return 1
	}
	icons.InitIcon(true, selectedTheme.DirectoryIconColor)
	model := ui.New(selectedTheme).WithThemes(ui.ThemeCatalog{
		Current: themeName,
		Names:   func() []string { return theme.Names(userThemes, configs.Themes) },
		Load:    func(name string) (theme.Theme, error) { return theme.Named(name, userThemes, configs.Themes) },
		Save:    func(name string) error { return config.SaveTheme(configPath, name) },
	}).WithPreview(settings.PreviewShown(), func(shown bool) error { return config.SavePreview(configPath, shown) })
	output := model.TerminalOutput(os.Stdout)
	model = model.WithTerminal(output)
	options := []tea.ProgramOption{tea.WithOutput(output), tea.WithAltScreen(), tea.WithMouseCellMotion()}
	var input *kittydnd.Input
	if kittydnd.Allowed() {
		input, err = kittydnd.OpenInput(output.Protocol())
		if err == nil {
			defer input.Close()
			options = append(options, tea.WithInput(input))
			model = model.WithDragOutput(output, input)
			_, _ = os.Stdout.WriteString(kittydnd.EnableSequence(kittydnd.MachineID()))
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
