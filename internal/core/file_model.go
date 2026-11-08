package core

import "time"

type EntryKind int

const (
	Directory EntryKind = iota
	File
)

type FileClass int

const (
	FileClassDirectory FileClass = iota
	FileClassSymlinkDirectory
	FileClassBrokenSymlink
	FileClassCode
	FileClassConfig
	FileClassDocument
	FileClassLicense
	FileClassImage
	FileClassAudio
	FileClassVideo
	FileClassArchive
	FileClassFont
	FileClassData
	FileClassFile
)

func (c FileClass) String() string {
	switch c {
	case FileClassDirectory:
		return "Directory"
	case FileClassSymlinkDirectory:
		return "Symlink directory"
	case FileClassBrokenSymlink:
		return "Broken symlink"
	case FileClassCode:
		return "Code"
	case FileClassConfig:
		return "Config"
	case FileClassDocument:
		return "Document"
	case FileClassLicense:
		return "License"
	case FileClassImage:
		return "Image"
	case FileClassAudio:
		return "Audio"
	case FileClassVideo:
		return "Video"
	case FileClassArchive:
		return "Archive"
	case FileClassFont:
		return "Font"
	case FileClassData:
		return "Data"
	case FileClassFile:
		return "File"
	default:
		return "Unknown"
	}
}

type Entry struct {
	Name     string
	Path     string
	Kind     EntryKind
	Size     int64
	Modified time.Time
	NameKey  string
}

func (e *Entry) IsDirectory() bool {
	return e.Kind == Directory
}
