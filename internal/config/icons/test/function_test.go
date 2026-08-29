package test

import (
	"testing"

	"github.com/urdadx/nukri/internal/config/icons"
)

func TestInitIconRestoresNerdFontIcons(t *testing.T) {
	icons.InitIcon(false, "")
	if icons.Home != "" || icons.Desktop != "" || icons.Trash != "" || icons.LinkedPlace != "" || icons.RemovableDevice != "" {
		t.Fatal("sidebar Nerd Font icons were not disabled")
	}
	if icons.CheckboxEmpty != "[ ]" || icons.CheckboxChecked != "[x]" {
		t.Fatal("ASCII checkbox fallbacks were not configured")
	}

	icons.InitIcon(true, "")
	if icons.Home == "" || icons.Desktop == "" || icons.Trash == "" || icons.LinkedPlace == "" || icons.RemovableDevice == "" {
		t.Fatal("sidebar Nerd Font icons were not restored")
	}
	if icons.Folders["folder"].Icon != icons.Directory {
		t.Fatal("generic folder icon does not follow the selected icon mode")
	}
}

func TestInitIconDisablesGenericFolderIcon(t *testing.T) {
	icons.InitIcon(false, "#ffffff")
	t.Cleanup(func() { icons.InitIcon(true, "") })

	folder := icons.Folders["folder"]
	if folder.Icon != "" {
		t.Fatalf("generic folder icon = %q, want empty ASCII fallback", folder.Icon)
	}
	if folder.Color != "#ffffff" {
		t.Fatalf("generic folder color = %q, want #ffffff", folder.Color)
	}
}
