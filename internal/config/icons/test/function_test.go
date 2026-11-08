package test

import (
	"sync"
	"testing"

	"github.com/urdadx/nukri/internal/config/icons"
)

func TestInitIconRestoresNerdFontIcons(t *testing.T) {
	icons.InitIcon(false, "")
	set := icons.Current()
	if set.Home != "" || set.Desktop != "" || set.Trash != "" || set.LinkedPlace != "" || set.RemovableDevice != "" {
		t.Fatal("sidebar Nerd Font icons were not disabled")
	}

	icons.InitIcon(true, "")
	set = icons.Current()
	if set.Home == "" || set.Desktop == "" || set.Trash == "" || set.LinkedPlace == "" || set.RemovableDevice == "" {
		t.Fatal("sidebar Nerd Font icons were not restored")
	}
	if set.Folder.Icon != set.Directory {
		t.Fatal("generic folder icon does not follow the selected icon mode")
	}
}

func TestInitIconDisablesGenericFolderIcon(t *testing.T) {
	icons.InitIcon(false, "#ffffff")
	t.Cleanup(func() { icons.InitIcon(true, "") })

	folder := icons.Current().Folder
	if folder.Icon != "" {
		t.Fatalf("generic folder icon = %q, want empty ASCII fallback", folder.Icon)
	}
	if folder.Color != "#ffffff" {
		t.Fatalf("generic folder color = %q, want #ffffff", folder.Color)
	}
}

// Theme changes call InitIcon while preview workers look up icons; run with
// -race to catch unsynchronized access.
func TestInitIconWhileIconsAreRead(t *testing.T) {
	t.Cleanup(func() { icons.InitIcon(true, "") })

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			icons.InitIcon(i%2 == 0, "#123456")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			set := icons.Current()
			if set.Folder.Icon != set.Directory {
				t.Error("generic folder icon and directory icon come from different modes")
				return
			}
			_ = icons.Folders[".git"]
		}
	}()
	wg.Wait()
}
