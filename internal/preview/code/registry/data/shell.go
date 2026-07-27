package data

import (
	fileinfo "github.com/urdadx/nukri/internal/file_info"
	registry "github.com/urdadx/nukri/internal/preview/code/registry/model"
)

var Shell = []registry.RegistryEntry{
	registry.Entry(
		registry.Language("sh", "Shell", fileinfo.Chroma, nil),
		[]string{"sh"},
		[]string{".profile", ".xprofile", ".xsessionrc", ".envrc"},
		[]string{"sh"},
		[]string{"sh", "shell"},
		[]string{"sh", "shell"},
	),
	registry.Entry(
		registry.Language("bash", "Bash", fileinfo.Chroma, nil),
		[]string{"bash"},
		[]string{
			".bashrc",
			".bash_profile",
			".bash_login",
			".bash_logout",
			".bash_aliases",
			"pkgbuild",
		},
		[]string{"bash"},
		[]string{"bash"},
		[]string{"bash"},
	),
	registry.Entry(
		registry.Language("zsh", "Zsh", fileinfo.Chroma, nil),
		[]string{"zsh"},
		[]string{".zshrc", ".zprofile", ".zshenv", ".zlogin", ".zlogout"},
		[]string{"zsh"},
		[]string{"zsh"},
		[]string{"zsh"},
	),
	registry.Entry(
		registry.Language("ksh", "KornShell", fileinfo.Chroma, nil),
		[]string{"ksh"},
		[]string{".kshrc", ".mkshrc"},
		[]string{"ksh"},
		[]string{"ksh"},
		[]string{"ksh"},
	),
	registry.Entry(
		registry.Language("fish", "Fish", fileinfo.Chroma, nil),
		[]string{"fish"},
		nil,
		[]string{"fish"},
		[]string{"fish"},
		[]string{"fish"},
	),
	registry.Entry(
		registry.Language("powershell", "PowerShell", fileinfo.Chroma, nil),
		[]string{"ps1", "psm1", "psd1"},
		nil,
		[]string{"pwsh", "powershell"},
		[]string{"powershell", "pwsh", "ps1"},
		[]string{"powershell", "pwsh", "ps1"},
	),
}
