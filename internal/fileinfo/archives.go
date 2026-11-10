package fileinfo

import (
	"strings"

	"github.com/urdadx/nukri/internal/core"
)

func inspectArchiveName(name string) (FileFacts, bool) {
	var detail string
	if kind, ok := inspectCompoundArchiveName(name); ok {
		detail = kind.DetailLabel()
	} else {
		archives := []struct {
			suffix string
			label  string
		}{
			{suffix: ".cbz", label: "Comic ZIP archive"},
			{suffix: ".cbr", label: "Comic RAR archive"},
			{suffix: ".rar", label: "RAR archive"},
			{suffix: ".zip", label: "ZIP archive"},
			{suffix: ".7z", label: "7z archive"},
			{suffix: ".jar", label: "Java archive"},
			{suffix: ".apk", label: "Android package"},
			{suffix: ".aab", label: "Android App Bundle"},
			{suffix: ".apkg", label: "Anki package"},
		}
		for _, archive := range archives {
			if strings.HasSuffix(name, archive.suffix) {
				detail = archive.label
				break
			}
		}
	}

	if detail == "" {
		return FileFacts{}, false
	}
	return plain(core.FileClassArchive, stringPtr(detail)), true
}

func inspectCompoundArchiveName(name string) (CompoundArchive, bool) {
	if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
		return CompoundArchive{Kind: TarGzip}, true
	}
	if strings.HasSuffix(name, ".tar.xz") || strings.HasSuffix(name, ".txz") {
		return CompoundArchive{Kind: TarXz}, true
	}
	if strings.HasSuffix(name, ".tar.bz2") || strings.HasSuffix(name, ".tbz2") || strings.HasSuffix(name, ".tbz") {
		return CompoundArchive{Kind: TarBzip2}, true
	}
	if strings.HasSuffix(name, ".tar.zst") || strings.HasSuffix(name, ".tzst") {
		return CompoundArchive{Kind: TarZstd}, true
	}

	return inspectCompressedDiskImageName(name)
}

func inspectCompressedDiskImageName(name string) (CompoundArchive, bool) {
	compressions := []struct {
		suffix string
		kind   CompressionKind
	}{
		{suffix: ".gz", kind: Gzip},
		{suffix: ".xz", kind: Xz},
		{suffix: ".bz2", kind: Bzip2},
		{suffix: ".zst", kind: Zstd},
	}
	for _, compression := range compressions {
		if archive, ok := CompressedDiskImageKind(name, compression.suffix, compression.kind); ok {
			return archive, true
		}
	}

	return CompoundArchive{}, false
}

// CompressedDiskImageKind identifies a disk image with the given compression suffix.
func CompressedDiskImageKind(name, compressionSuffix string, compression CompressionKind) (CompoundArchive, bool) {
	base, ok := strings.CutSuffix(name, compressionSuffix)
	if !ok {
		return CompoundArchive{}, false
	}
	image, ok := diskImageKindFromName(base)
	if !ok {
		return CompoundArchive{}, false
	}
	return CompoundArchive{Kind: CompressedDiskImage, Image: image, Compression: compression}, true
}

func diskImageKindFromName(name string) (DiskImageKind, bool) {
	images := []struct {
		suffix string
		kind   DiskImageKind
	}{
		{suffix: ".raw", kind: RawImage},
		{suffix: ".img", kind: DiskImage},
		{suffix: ".iso", kind: IsoImage},
		{suffix: ".qcow2", kind: Qcow2},
		{suffix: ".vmdk", kind: Vmdk},
		{suffix: ".vdi", kind: Vdi},
		{suffix: ".vhd", kind: Vhd},
		{suffix: ".vhdx", kind: Vhdx},
	}
	for _, image := range images {
		if strings.HasSuffix(name, image.suffix) {
			return image.kind, true
		}
	}
	return "", false
}
