package preview

import (
	"errors"
	"strings"
	"unicode"

	fileinfo "github.com/urdadx/nukri/internal/file_info"
)

var (
	ErrUnsupported     = errors.New("preview format is unsupported")
	ErrToolUnavailable = errors.New("preview tool is unavailable")
	ErrOutputTooLarge  = errors.New("preview output exceeds limit")
)

const (
	DefaultMaxImageDimension   = 1600
	DefaultMaxImageBytes       = 16 << 20
	DefaultMaxArchiveEntries   = 10_000
	DefaultMaxToolOutput       = 8 << 20
	DefaultMaxEPUBBytes        = 64 << 20
	DefaultMaxEPUBEntryBytes   = 16 << 20
	DefaultMaxEPUBTotalBytes   = 256 << 20
	DefaultMaxSVGBytes         = 8 << 20
	DefaultMaxMarkdownBytes    = 2 << 20
	DefaultMarkdownWidth       = 80
	DefaultMaxFontBytes        = 32 << 20
	DefaultMaxDirectoryEntries = 10_000
	DefaultMaxCSVBytes         = 2 << 20
	DefaultMaxCSVRows          = 100
	DefaultMaxCSVColumns       = 50
	DefaultMaxCSVCellRunes     = 1_000
	DefaultMaxTorrentBytes     = 8 << 20
	DefaultMaxTorrentFiles     = 10_000
	DefaultMaxTorrentTrackers  = 100
)

type Request struct {
	Path  string
	Facts fileinfo.FileFacts
	Width int
}

type Preview interface {
	isPreview()
}

type PDFPreview struct {
	Page     Image
	Metadata []Field
}

type SVGPreview struct {
	Image Image
}

type OfficePreview struct {
	Format   fileinfo.DocumentFormat
	Page     Image
	Metadata []Field
}

type EbookPreview struct {
	Format   fileinfo.DocumentFormat
	Page     Image
	Metadata []Field
}

type MarkdownPreview struct {
	Text string
}

type DirectoryPreview struct {
	Entries     []DirectoryEntry
	TotalItems  int
	FolderCount int
	FileCount   int
	Truncated   bool
}

type ArchivePreview struct {
	Archive Archive
}

type EPUBPreview struct {
	Book     EPUB
	Metadata []Field
}

type FontPreview struct {
	Font     Font
	Specimen Image
	Metadata []Field
}

type AudioPreview struct {
	Audio    Audio
	Visual   Image
	Metadata []Field
}

type VideoPreview struct {
	Video    Video
	Frame    Image
	Metadata []Field
}

type CSVPreview struct {
	Rows             [][]string
	Metadata         CSVMetadata
	RowsTruncated    bool
	ColumnsTruncated bool
	CellsTruncated   bool
}

type CSVMetadata struct {
	RowCount    int
	ColumnCount int
	Delimiter   rune
}

type TorrentPreview struct {
	Torrent   Torrent
	Files     []TorrentFile
	Metadata  []Field
	Truncated bool
}

type Torrent struct {
	Name         string
	InfoHashV1   string
	InfoHashV2   string
	Comment      string
	CreatedBy    string
	CreationDate int64
	Private      bool
	PieceLength  int64
	PieceCount   int
	TotalSize    int64
	Trackers     []string
	WebSeeds     []string
}

type TorrentFile struct {
	Path string
	Size int64
}

type ISOPreview struct {
	ISO       ISO
	Entries   []ISOEntry
	Metadata  []Field
	Truncated bool
}

type ISO struct {
	SystemID      string
	VolumeID      string
	VolumeSetID   string
	PublisherID   string
	DataPreparer  string
	ApplicationID string
	BlockSize     int64
	VolumeBlocks  int64
	FileSize      int64
	RockRidge     bool
	Joliet        bool
	Bootable      bool
}

type ISOEntry struct {
	Path string
}

func (*PDFPreview) isPreview()       {}
func (*SVGPreview) isPreview()       {}
func (*OfficePreview) isPreview()    {}
func (*EbookPreview) isPreview()     {}
func (*MarkdownPreview) isPreview()  {}
func (*DirectoryPreview) isPreview() {}
func (*ArchivePreview) isPreview()   {}
func (*EPUBPreview) isPreview()      {}
func (*FontPreview) isPreview()      {}
func (*AudioPreview) isPreview()     {}
func (*VideoPreview) isPreview()     {}
func (*CSVPreview) isPreview()       {}
func (*TorrentPreview) isPreview()   {}
func (*ISOPreview) isPreview()       {}

type Image struct {
	MediaType string
	Data      []byte
	Width     int
	Height    int
}

type Field struct {
	Name  string
	Value string
}

type Archive struct {
	Entries   []ArchiveEntry
	Truncated bool
}

type ArchiveEntry struct {
	Path       string
	Size       int64
	PackedSize int64
	Modified   string
	Attributes string
	Directory  bool
}

type EPUB struct {
	Identifier   string
	Titles       []string
	Creators     []string
	Languages    []string
	Publishers   []string
	Subjects     []string
	Descriptions []string
	Dates        []string
}

type Font struct {
	Family     string
	Subfamily  string
	FullName   string
	Version    string
	Format     string
	Glyphs     int
	UnitsPerEm uint16
}

type DirectoryEntry struct {
	Name      string
	Size      int64
	Mode      string
	Modified  string
	Directory bool
	Symlink   bool
}

type Audio struct {
	Title         string
	Artist        string
	Album         string
	Date          string
	Genre         string
	Codec         string
	Container     string
	ChannelLayout string
	Duration      float64
	BitRate       int64
	SampleRate    int
	Channels      int
	BitDepth      int
	Visual        PreviewVisualKind
}

type Video struct {
	Title       string
	Codec       string
	Container   string
	PixelFormat string
	Duration    float64
	BitRate     int64
	Width       int
	Height      int
	FrameRate   float64
	AudioCodec  string
	AudioTracks int
}

type Tools struct {
	PDFInfo      string
	PDFToCairo   string
	SevenZip     string
	LibreOffice  string
	EbookConvert string
	FFProbe      string
	FFmpeg       string
	ISOInfo      string
}

type Capabilities struct {
	PDF       bool
	SVG       bool
	Archives  bool
	Documents bool
	EPUB      bool
	Ebooks    bool
	Fonts     bool
	Audio     bool
	Video     bool
	ISO       bool
}

func safeText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}

type PreviewVisualKind int

const (
	Cover PreviewVisualKind = iota
	PageImage
	Waveform
)
