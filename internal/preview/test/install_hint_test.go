package test

import (
	"testing"

	"github.com/urdadx/nukri/internal/preview"
)

func TestInstallCommandNamesThePackage(t *testing.T) {
	for _, test := range []struct {
		tool, manager string
		root          bool
		want          string
	}{
		{"rsvg-convert", "apt", false, "sudo apt install librsvg2-bin"},
		{"rsvg-convert", "pacman", true, "pacman -S librsvg"},
		{"pdftoppm/pdfinfo", "dnf", false, "sudo dnf install poppler-utils"},
		{"ffprobe/ffmpeg", "apk", true, "apk add ffmpeg"},
		{"zstd", "zypper", false, "sudo zypper install zstd"},
		{"rsvg-convert", "", false, ""},     // unknown package manager
		{"no-such-tool", "apt", false, ""},  // unknown tool
		{"rsvg-convert", "brew", false, ""}, // unsupported package manager
	} {
		if got := preview.InstallCommandFor(test.tool, test.manager, test.root); got != test.want {
			t.Errorf("InstallCommandFor(%q, %q, %v) = %q, want %q", test.tool, test.manager, test.root, got, test.want)
		}
	}
}

// Every tool a preview can ask for has a package on each supported manager.
func TestEveryPreviewToolHasAPackage(t *testing.T) {
	tools := []string{"ffprobe/ffmpeg", "rsvg-convert", "pdftoppm/pdfinfo", "imagemagick", "7zz", "xz", "zstd", "isoinfo", "libreoffice", "ebook-convert"}
	for _, tool := range tools {
		for _, manager := range []string{"apt", "pacman", "dnf", "zypper", "apk"} {
			if preview.InstallCommandFor(tool, manager, true) == "" {
				t.Errorf("no %s package for %s", manager, tool)
			}
		}
	}
}
