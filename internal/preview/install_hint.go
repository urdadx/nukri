package preview

import (
	"os"
	"os/exec"
	"strings"
	"sync"
)

// toolPackages names the package that provides each preview tool, by package
// manager. Tool names are those ToolUnavailable reports. install.sh carries the
// same table for its optional tools prompt.
var toolPackages = map[string]map[string]string{
	"ffprobe/ffmpeg":   {"apt": "ffmpeg", "pacman": "ffmpeg", "dnf": "ffmpeg-free", "zypper": "ffmpeg", "apk": "ffmpeg"},
	"rsvg-convert":     {"apt": "librsvg2-bin", "pacman": "librsvg", "dnf": "librsvg2-tools", "zypper": "rsvg-convert", "apk": "rsvg-convert"},
	"pdftoppm/pdfinfo": {"apt": "poppler-utils", "pacman": "poppler", "dnf": "poppler-utils", "zypper": "poppler-tools", "apk": "poppler-utils"},
	"imagemagick":      {"apt": "imagemagick", "pacman": "imagemagick", "dnf": "ImageMagick", "zypper": "ImageMagick", "apk": "imagemagick"},
	"7zz":              {"apt": "7zip", "pacman": "7zip", "dnf": "7zip", "zypper": "7zip", "apk": "7zip"},
	"xz":               {"apt": "xz-utils", "pacman": "xz", "dnf": "xz", "zypper": "xz", "apk": "xz"},
	"zstd":             {"apt": "zstd", "pacman": "zstd", "dnf": "zstd", "zypper": "zstd", "apk": "zstd"},
	"isoinfo":          {"apt": "genisoimage", "pacman": "cdrtools", "dnf": "genisoimage", "zypper": "genisoimage", "apk": "cdrkit"},
	"libreoffice":      {"apt": "libreoffice", "pacman": "libreoffice-fresh", "dnf": "libreoffice", "zypper": "libreoffice", "apk": "libreoffice"},
	"ebook-convert":    {"apt": "calibre", "pacman": "calibre", "dnf": "calibre", "zypper": "calibre", "apk": "calibre"},
}

// packageManagers pairs each supported package manager with its install
// command, in the order they are looked for.
var packageManagers = []struct{ name, binary, install string }{
	{"apt", "apt-get", "apt install"},
	{"pacman", "pacman", "pacman -S"},
	{"dnf", "dnf", "dnf install"},
	{"zypper", "zypper", "zypper install"},
	{"apk", "apk", "apk add"},
}

// detectedPackageManager finds this system's package manager once.
var detectedPackageManager = sync.OnceValues(func() (string, string) {
	for _, manager := range packageManagers {
		if _, err := exec.LookPath(manager.binary); err == nil {
			return manager.name, manager.install
		}
	}
	return "", ""
})

/*
InstallCommand returns the command that installs tool with this system's
package manager, such as "sudo apt install librsvg2-bin", or "" when the
package manager or the tool's package is unknown. Nothing is run: the preview
only shows it.
*/
func InstallCommand(tool string) string {
	manager, _ := detectedPackageManager()
	return InstallCommandFor(tool, manager, os.Geteuid() == 0)
}

// InstallCommandFor is InstallCommand for a given package manager ("apt",
// "pacman", "dnf", "zypper" or "apk"), run as root or through sudo.
func InstallCommandFor(tool, manager string, root bool) string {
	var install string
	for _, candidate := range packageManagers {
		if candidate.name == manager {
			install = candidate.install
		}
	}
	pkg := toolPackages[tool][manager]
	if pkg == "" || install == "" {
		return ""
	}
	command := install + " " + pkg
	if !root {
		command = "sudo " + command
	}
	return strings.TrimSpace(command)
}
