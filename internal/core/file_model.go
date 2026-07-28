package core

import "time"

type SortMode int

const (
	Name SortMode = iota
	Modified
	Size
)

func (s SortMode) Cycle() SortMode {
	switch s {
	case Name:
		return Modified
	case Modified:
		return Size
	case Size:
		return Name
	default:
		return Name
	}
}

func (s SortMode) Label() string {
	switch s {
	case Name:
		return "Name"
	case Modified:
		return "Modified"
	case Size:
		return "Size"
	default:
		return "Unknown"
	}
}

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

type SymlinkInfo struct {
	Target     *string
	TargetKind *EntryKind
}

func (s *SymlinkInfo) IsBroken() bool {
	return s.TargetKind == nil
}

type Entry struct {
	Name        string
	Path        string
	Kind        EntryKind
	Size        int64
	Modified    time.Time
	SymlinkInfo *SymlinkInfo
	ReadOnly    bool
	NameKey     string
}

func (e *Entry) Default() {
	e.Path = ""
	e.Name = ""
	e.NameKey = ""
	e.Kind = File
	e.SymlinkInfo = nil
	e.Size = 0
	e.Modified = time.Time{}
	e.ReadOnly = false
}

func (e *Entry) IsDirectory() bool {
	return e.Kind == Directory
}

func (e *Entry) IsSymlink() bool {
	return e.SymlinkInfo != nil
}

func (e *Entry) IsBrokenSymlink() bool {
	return e.IsSymlink() && e.SymlinkInfo.IsBroken()
}
