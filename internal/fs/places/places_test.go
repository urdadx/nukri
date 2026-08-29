package places

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urdadx/nukri/internal/config"
	"github.com/urdadx/nukri/internal/config/icons"
	"github.com/urdadx/nukri/internal/core"
)

func TestLinuxDeviceItemsFilterAndSortVisibleMounts(t *testing.T) {
	mounts := parseLinuxMounts(
		"proc /proc proc rw 0 0\n" +
			"tmpfs /run tmpfs rw 0 0\n" +
			"/dev/sda1 /boot ext4 rw 0 0\n" +
			"/dev/sdb1 /run/media/alex/My\\040USB exfat rw 0 0\n" +
			"/dev/sdc1 /home/alex/mnt/photos ext4 rw 0 0\n" +
			"server:/share /run/user/1000/gvfs fuse.gvfsd-fuse rw 0 0\n",
	)

	items := linuxDeviceItemsFromMounts(
		mounts,
		"/home/alex",
		map[string]string{"/dev/sdb1": "Vacation"},
		map[string]bool{"sdb": true, "sdc": false},
		map[string]struct{}{filepath.Clean("/home/alex"): {}, "/": {}},
	)

	if len(items) != 2 {
		t.Fatalf("got %d device items, want 2: %#v", len(items), items)
	}
	if items[0].Title != "photos" || items[0].Path != "/home/alex/mnt/photos" || items[0].Removable {
		t.Fatalf("unexpected fixed device: %#v", items[0])
	}
	if items[0].Icon != icons.FixedDevice {
		t.Fatalf("fixed device icon = %q, want %q", items[0].Icon, icons.FixedDevice)
	}
	if items[1].Title != "Vacation" || items[1].Path != "/run/media/alex/My USB" || !items[1].Removable {
		t.Fatalf("unexpected removable device: %#v", items[1])
	}
	if items[1].Icon != icons.RemovableDevice {
		t.Fatalf("removable device icon = %q, want %q", items[1].Icon, icons.RemovableDevice)
	}
}

func TestLinuxDeviceItemsKeepCustomTopLevelMountOnly(t *testing.T) {
	mounts := parseLinuxMounts(
		"/dev/sda2 /home ext4 rw 0 0\n" +
			"/dev/sda3 /var ext4 rw 0 0\n" +
			"/dev/sdb1 /data ext4 rw 0 0\n" +
			"/dev/loop0 /snap/core squashfs ro 0 0\n",
	)
	items := linuxDeviceItemsFromMounts(mounts, "/home/alex", nil, map[string]bool{}, map[string]struct{}{"/": {}})

	if len(items) != 1 || items[0].Title != "data" || items[0].Path != "/data" {
		t.Fatalf("unexpected top-level device items: %#v", items)
	}
}

func TestParseLinuxMountsAndLabelEscapes(t *testing.T) {
	mounts := parseLinuxMounts("/dev/sdb1 /media/My\\040Disk ext4 rw 0 0\nmalformed\n")
	if len(mounts) != 1 {
		t.Fatalf("got %d mounts, want 1", len(mounts))
	}
	if mounts[0].mountPoint != "/media/My Disk" {
		t.Fatalf("mount point = %q, want %q", mounts[0].mountPoint, "/media/My Disk")
	}
	if got := decodeLinuxLabelName(`New\x20vol\x23A`); got != "New vol#A" {
		t.Fatalf("decoded label = %q, want %q", got, "New vol#A")
	}
}

func TestLinuxBaseBlockDeviceName(t *testing.T) {
	tests := map[string]string{
		"sdb1": "sdb", "vda2": "vda", "nvme0n1p2": "nvme0n1",
		"mmcblk0p1": "mmcblk0", "loop7": "loop7",
	}
	for input, want := range tests {
		got, ok := linuxBaseBlockDeviceName(input)
		if !ok || got != want {
			t.Errorf("linuxBaseBlockDeviceName(%q) = %q, %v; want %q, true", input, got, ok, want)
		}
	}
	if _, ok := linuxBaseBlockDeviceName("zfs"); ok {
		t.Fatal("unsupported source was recognized as a block device")
	}
}

func TestBuildPinnedSidebarItemsResolvesAndDeduplicates(t *testing.T) {
	home := t.TempDir()
	downloads := filepath.Join(home, "Downloads")
	if err := os.Mkdir(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	context := &PlaceResolutionContext{Home: home, Downloads: &downloads}
	place := config.Downloads
	places := config.PlacesConfig{Entries: []config.PlaceEntrySpec{
		{Builtin: &place},
		{Title: "Duplicate", Path: downloads},
	}}

	items := buildPinnedSidebarItems(places, context)
	if len(items) != 1 {
		t.Fatalf("got %d pinned items, want 1: %#v", len(items), items)
	}
	if items[0].Kind != core.Downloads || items[0].Title != "Downloads" || items[0].Path != downloads {
		t.Fatalf("unexpected Downloads item: %#v", items[0])
	}
}

func TestPlaceIconHandlesOverridesAndSymlinks(t *testing.T) {
	directory := t.TempDir()
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(directory, link); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(t.TempDir(), "broken")
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), broken); err != nil {
		t.Fatal(err)
	}

	if got := placeIcon(directory, "custom", "default"); got != "custom" {
		t.Fatalf("override icon = %q, want custom", got)
	}
	if got := placeIcon(directory, "", "default"); got != "default" {
		t.Fatalf("regular directory icon = %q, want default", got)
	}
	if got := placeIcon(link, "", "default"); got != icons.LinkedPlace {
		t.Fatalf("linked directory icon = %q, want %q", got, icons.LinkedPlace)
	}
	if got := placeIcon(broken, "", "default"); got != icons.BrokenPlace {
		t.Fatalf("broken link icon = %q, want %q", got, icons.BrokenPlace)
	}
}

func TestPathIdentityKeyResolvesSymlinks(t *testing.T) {
	directory := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(directory, link); err != nil {
		t.Fatal(err)
	}
	if got := pathIdentityKey(link); got != directory {
		t.Fatalf("identity path = %q, want %q", got, directory)
	}
}

func TestExistingDirRejectsMissingAndRegularFiles(t *testing.T) {
	directory := t.TempDir()
	if got := existingDir(directory); got == nil || *got != directory {
		t.Fatalf("existingDir(%q) = %#v", directory, got)
	}
	file := filepath.Join(directory, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := existingDir(file); got != nil {
		t.Fatalf("regular file accepted as directory: %q", *got)
	}
	if got := existingDir(filepath.Join(directory, "missing")); got != nil {
		t.Fatalf("missing directory accepted: %q", *got)
	}
}
