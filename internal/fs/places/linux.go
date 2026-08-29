/**
 * Discovers mounted Linux devices from /proc/mounts, filters system and hidden
 * mounts, resolves device labels and removability, and converts visible mounts
 * into sorted sidebar items without accessing potentially blocking mount paths.
 */

package places

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/urdadx/nukri/internal/config/icons"
	"github.com/urdadx/nukri/internal/core"
)

type LinuxMount struct {
	source     string
	mountPoint string
	fsType     string
}

func mountedDeviceItems(home string, pinnedPaths map[string]struct{}) []core.SidebarItem {
	content, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}
	mounts := parseLinuxMounts(string(content))
	return linuxDeviceItemsFromMounts(
		mounts,
		home,
		linuxDeviceLabels(),
		linuxRemovableDevices(mounts),
		pinnedPaths,
	)
}

func parseLinuxMounts(content string) []LinuxMount {
	var mounts []LinuxMount
	for line := range strings.SplitSeq(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mounts = append(mounts, LinuxMount{
			source:     unmangleProcMountField(fields[0]),
			mountPoint: unmangleProcMountField(fields[1]),
			fsType:     unmangleProcMountField(fields[2]),
		})
	}
	return mounts
}

func linuxDeviceItemsFromMounts(mounts []LinuxMount, home string, labels map[string]string, removable map[string]bool, pinnedPaths map[string]struct{}) []core.SidebarItem {
	seen := make(map[string]struct{})
	var items []core.SidebarItem
	for _, mount := range mounts {
		isRemovable := linuxMountRemovable(mount, removable)
		if !linuxMountShouldAppear(mount, home, pinnedPaths, isRemovable) {
			continue
		}
		if _, exists := seen[mount.mountPoint]; exists {
			continue
		}
		seen[mount.mountPoint] = struct{}{}

		// Never canonicalize mount points here. Statting an autofs trigger or dead
		// network share can block the main event loop until the mount responds.
		icon := icons.FixedDevice
		if isRemovable {
			icon = icons.RemovableDevice
		}
		item := core.NewSidebarItem(core.Device, linuxMountTitle(mount, labels), icon, mount.mountPoint, filepath.Clean(mount.mountPoint))
		item.Removable = isRemovable
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		left, right := strings.ToLower(items[i].Title), strings.ToLower(items[j].Title)
		if left == right {
			return items[i].Path < items[j].Path
		}
		return left < right
	})
	return items
}

func linuxMountShouldAppear(mount LinuxMount, home string, pinnedPaths map[string]struct{}, removable bool) bool {
	// Keep these filters free of path syscalls to avoid triggering or blocking mounts.
	if mount.mountPoint == "/" || linuxSystemMountType(mount.fsType) || linuxHiddenMountPath(mount.mountPoint) {
		return false
	}
	if _, pinned := pinnedPaths[filepath.Clean(mount.mountPoint)]; pinned {
		return false
	}
	return linuxUserVisibleMountPath(mount.mountPoint, home) || linuxTopLevelUserMountPath(mount.mountPoint) || removable
}

func linuxSystemMountType(fsType string) bool {
	systemTypes := map[string]struct{}{
		"autofs": {}, "aufs": {}, "binfmt_misc": {}, "bpf": {}, "cgroup": {},
		"cgroup2": {}, "configfs": {}, "debugfs": {}, "devpts": {}, "devtmpfs": {},
		"efivarfs": {}, "fuse.gvfsd-fuse": {}, "fuse.portal": {}, "fusectl": {},
		"hugetlbfs": {}, "mqueue": {}, "nsfs": {}, "overlay": {}, "proc": {},
		"pstore": {}, "ramfs": {}, "rpc_pipefs": {}, "securityfs": {}, "squashfs": {},
		"sysfs": {}, "tmpfs": {}, "tracefs": {},
	}
	_, found := systemTypes[fsType]
	return found
}

func linuxHiddenMountPath(path string) bool {
	if pathHasPrefix(path, "/run/media") {
		return false
	}
	for _, prefix := range []string{"/proc", "/sys", "/dev", "/run", "/snap", "/var/lib"} {
		if pathHasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func linuxUserVisibleMountPath(path, home string) bool {
	for _, prefix := range []string{home, "/media", "/run/media", "/mnt", "/Volumes"} {
		if prefix != "" && pathHasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func pathHasPrefix(path, prefix string) bool {
	relative, err := filepath.Rel(prefix, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func linuxTopLevelUserMountPath(path string) bool {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) || clean == "/" || strings.Count(strings.TrimPrefix(clean, "/"), "/") != 0 {
		return false
	}
	name := filepath.Base(clean)
	systemRoots := map[string]struct{}{
		"bin": {}, "boot": {}, "dev": {}, "etc": {}, "home": {}, "lib": {}, "lib32": {},
		"lib64": {}, "lost+found": {}, "nix": {}, "opt": {}, "proc": {}, "root": {},
		"run": {}, "sbin": {}, "snap": {}, "srv": {}, "sys": {}, "tmp": {}, "usr": {}, "var": {},
	}
	_, system := systemRoots[name]
	return !system
}

func linuxMountTitle(mount LinuxMount, labels map[string]string) string {
	for _, key := range linuxDeviceLookupKeys(mount.source) {
		if label := labels[key]; label != "" {
			return label
		}
	}
	if title := filepath.Base(mount.mountPoint); title != "." && title != "/" && title != "" {
		return title
	}
	if title := filepath.Base(mount.source); title != "." && title != "/" && title != "" {
		return title
	}
	return mount.mountPoint
}

func linuxDeviceLookupKeys(source string) []string {
	keys := make([]string, 0, 2)
	if strings.HasPrefix(source, "/dev/") {
		if canonical, err := filepath.EvalSymlinks(source); err == nil {
			keys = append(keys, canonical)
		}
	}
	return append(keys, source)
}

func linuxDeviceLabels() map[string]string {
	labels := make(map[string]string)
	entries, err := os.ReadDir("/dev/disk/by-label")
	if err != nil {
		return labels
	}
	for _, entry := range entries {
		label := decodeLinuxLabelName(entry.Name())
		if label == "" {
			continue
		}
		target, err := filepath.EvalSymlinks(filepath.Join("/dev/disk/by-label", entry.Name()))
		if err == nil {
			if _, exists := labels[target]; !exists {
				labels[target] = label
			}
		}
	}
	return labels
}

func decodeLinuxLabelName(label string) string {
	decoded := make([]byte, 0, len(label))
	for index := 0; index < len(label); index++ {
		if label[index] == '\\' && index+3 < len(label) && label[index+1] == 'x' {
			high, highOK := hexValue(label[index+2])
			low, lowOK := hexValue(label[index+3])
			if highOK && lowOK {
				decoded = append(decoded, high<<4|low)
				index += 3
				continue
			}
		}
		decoded = append(decoded, label[index])
	}
	return strings.ToValidUTF8(string(decoded), "�")
}

func hexValue(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}

func linuxRemovableDevices(mounts []LinuxMount) map[string]bool {
	removable := make(map[string]bool)
	for _, mount := range mounts {
		deviceName, ok := linuxSourceDeviceName(mount.source)
		if !ok {
			continue
		}
		baseName, ok := linuxBaseBlockDeviceName(deviceName)
		if !ok {
			continue
		}
		if _, exists := removable[baseName]; !exists {
			removable[baseName] = linuxBlockDeviceIsRemovable(baseName)
		}
	}
	return removable
}

func linuxMountRemovable(mount LinuxMount, removable map[string]bool) bool {
	deviceName, ok := linuxSourceDeviceName(mount.source)
	if !ok {
		return false
	}
	baseName, ok := linuxBaseBlockDeviceName(deviceName)
	return ok && removable[baseName]
}

func linuxSourceDeviceName(source string) (string, bool) {
	if !strings.HasPrefix(source, "/dev/") {
		return "", false
	}
	name := filepath.Base(source)
	return name, name != "." && name != "/" && name != ""
}

func linuxBaseBlockDeviceName(deviceName string) (string, bool) {
	if len(deviceName) < 3 {
		return "", false
	}
	for _, prefix := range []string{"sd", "hd", "vd"} {
		if strings.HasPrefix(deviceName, prefix) {
			return deviceName[:3], true
		}
	}
	if strings.HasPrefix(deviceName, "nvme") || strings.HasPrefix(deviceName, "mmcblk") {
		if before, _, ok := strings.Cut(deviceName, "p"); ok {
			return before, true
		}
		return deviceName, true
	}
	if strings.HasPrefix(deviceName, "loop") {
		return deviceName, true
	}
	return "", false
}

func linuxBlockDeviceIsRemovable(baseName string) bool {
	content, err := os.ReadFile(filepath.Join("/sys/block", baseName, "removable"))
	return err == nil && strings.TrimSpace(string(content)) == "1"
}

func unmangleProcMountField(value string) string {
	for _, replacement := range [][2]string{
		{`\011`, "\t"}, {`\012`, "\n"}, {`\040`, " "}, {`\043`, "#"}, {`\134`, `\`},
	} {
		value = strings.ReplaceAll(value, replacement[0], replacement[1])
	}
	return value
}
