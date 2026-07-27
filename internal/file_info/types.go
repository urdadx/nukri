package fileinfo

import (
	"fmt"

	"github.com/urdadx/nukri/internal/core"
)

type PreviewKind int

const (
	Markdown PreviewKind = iota
	Source
	PlainText
	Iso
	Torrent
	Sqlite
	SqliteCandidate
	Csv
)

type DocumentFormat int

const (
	Doc DocumentFormat = iota
	Docx
	Pdf
	Html
	Odt
	Xls
	Xlsx
	Pptx
	Pptm
	Xlsm
	Azw3
	Pages
	Ods
	Odp
	Document
	Epub
	Mobi
)

func (d DocumentFormat) DetailLabel() string {
	switch d {
	case Doc:
		return "Microsoft Word Document"
	case Docx:
		return "Microsoft Word Open XML Document"
	case Pdf:
		return "PDF Document"
	case Html:
		return "HTML Document"
	case Odt:
		return "OpenDocument Text Document"
	case Xls:
		return "Microsoft Excel Spreadsheet"
	case Xlsx:
		return "Microsoft Excel Open XML Spreadsheet"
	case Pptx:
		return "Microsoft PowerPoint Presentation"
	case Pptm:
		return "Microsoft PowerPoint Macro-Enabled Presentation"
	case Xlsm:
		return "Microsoft Excel Macro-Enabled Spreadsheet"
	case Azw3:
		return "Amazon Kindle eBook"
	case Pages:
		return "Apple Pages Document"
	case Ods:
		return "OpenDocument Spreadsheet"
	case Odp:
		return "OpenDocument Presentation"
	case Document:
		return "Document File"
	case Epub:
		return "EPUB eBook"
	case Mobi:
		return "Mobipocket eBook"
	default:
		return "Unknown Document Format"
	}
}

type CodeBackend int

const (
	Plain CodeBackend = iota
	Chroma
	Custom
)

type CustomCodeKind int

const (
	DirectiveConf CustomCodeKind = iota
	Ini
	DesktopEntry
	Json
	Jsonc
	Toml
	Yaml
	Log
)

type StructuredFormat string

const (
	StructuredJSON   StructuredFormat = "json"
	StructuredJSONC  StructuredFormat = "jsonc"
	StructuredJSON5  StructuredFormat = "json5"
	StructuredTOML   StructuredFormat = "toml"
	StructuredYAML   StructuredFormat = "yaml"
	StructuredDotenv StructuredFormat = "dotenv"
	StructuredLog    StructuredFormat = "log"
)

func (f StructuredFormat) DetailLabel() string {
	switch f {
	case StructuredJSON:
		return "JSON"
	case StructuredJSONC:
		return "JSONC"
	case StructuredJSON5:
		return "JSON5"
	case StructuredTOML:
		return "TOML"
	case StructuredYAML:
		return "YAML"
	case StructuredDotenv:
		return ".env"
	case StructuredLog:
		return "Log"
	default:
		return "Unknown"
	}
}

type CompressionKind string

const (
	Gzip  CompressionKind = "gzip"
	Xz    CompressionKind = "xz"
	Bzip2 CompressionKind = "bzip2"
	Zstd  CompressionKind = "zstd"
)

type DiskImageKind string

const (
	RawImage  DiskImageKind = "raw"
	DiskImage DiskImageKind = "img"
	IsoImage  DiskImageKind = "iso"
	Qcow2     DiskImageKind = "qcow2"
	Vmdk      DiskImageKind = "vmdk"
	Vdi       DiskImageKind = "vdi"
	Vhd       DiskImageKind = "vhd"
	Vhdx      DiskImageKind = "vhdx"
)

func (k DiskImageKind) DetailLabel() string {
	switch k {
	case RawImage:
		return "Raw disk image"
	case DiskImage:
		return "Disk image"
	case IsoImage:
		return "ISO disk image"
	case Qcow2:
		return "QCOW2 disk image"
	case Vmdk:
		return "VMDK disk image"
	case Vdi:
		return "VDI disk image"
	case Vhd:
		return "VHD disk image"
	case Vhdx:
		return "VHDX disk image"
	default:
		return "Unknown disk image"
	}
}

type CompoundArchiveKind string

const (
	TarGzip  CompoundArchiveKind = "tar.gz"
	TarXz    CompoundArchiveKind = "tar.xz"
	TarBzip2 CompoundArchiveKind = "tar.bz2"
	TarZstd  CompoundArchiveKind = "tar.zst"

	CompressedDiskImage CompoundArchiveKind = "compressed_disk_image"
)

type CompoundArchive struct {
	Kind        CompoundArchiveKind
	Image       DiskImageKind
	Compression CompressionKind
}

func compressedDiskImageLabel(c CompressionKind, d DiskImageKind) string {
	compression := map[CompressionKind]string{
		Gzip:  "Gzip",
		Xz:    "XZ",
		Bzip2: "Bzip2",
		Zstd:  "Zstandard",
	}[c]

	return fmt.Sprintf("%s-compressed %s", compression, d.DetailLabel())
}

func (c CompoundArchive) DetailLabel() string {
	switch c.Kind {
	case TarGzip:
		return "TAR.GZ archive"
	case TarXz:
		return "TAR.XZ archive"
	case TarBzip2:
		return "TAR.BZ2 archive"
	case TarZstd:
		return "TAR.ZST archive"

	case CompressedDiskImage:
		return compressedDiskImageLabel(c.Compression, c.Image)

	default:
		return "Unknown archive"
	}
}

type PreviewSpec struct {
	Kind             PreviewKind
	LanguageHint     *string
	CodeSyntax       *string
	CodeBackend      CodeBackend
	StructuredFormat *StructuredFormat
	DocumentFormat   *DocumentFormat
}

type FileFacts struct {
	BuiltinClass      core.FileClass
	SpecificTypeLabel *string
	Preview           PreviewSpec
}

func PlainTextPreview() PreviewSpec {
	return PreviewSpec{
		Kind:             PlainText,
		LanguageHint:     nil,
		CodeSyntax:       nil,
		CodeBackend:      Plain,
		StructuredFormat: nil,
		DocumentFormat:   nil,
	}
}

func MarkdownPreview() PreviewSpec {
	return PreviewSpec{
		Kind:             Markdown,
		LanguageHint:     nil,
		CodeSyntax:       nil,
		CodeBackend:      Plain,
		StructuredFormat: nil,
		DocumentFormat:   nil,
	}
}

func IsoPreview() PreviewSpec {
	return PreviewSpec{
		Kind:             Iso,
		LanguageHint:     nil,
		CodeSyntax:       nil,
		CodeBackend:      Plain,
		StructuredFormat: nil,
		DocumentFormat:   nil,
	}
}

func TorrentPreview() PreviewSpec {
	return PreviewSpec{
		Kind:             Torrent,
		LanguageHint:     nil,
		CodeSyntax:       nil,
		CodeBackend:      Plain,
		StructuredFormat: nil,
		DocumentFormat:   nil,
	}
}

func SqlitePreview() PreviewSpec {
	return PreviewSpec{
		Kind:             Sqlite,
		LanguageHint:     nil,
		CodeSyntax:       nil,
		CodeBackend:      Plain,
		StructuredFormat: nil,
		DocumentFormat:   nil,
	}
}

func SqliteCandidatePreview() PreviewSpec {
	return PreviewSpec{
		Kind:             SqliteCandidate,
		LanguageHint:     nil,
		CodeSyntax:       nil,
		CodeBackend:      Plain,
		StructuredFormat: nil,
		DocumentFormat:   nil,
	}
}

func CsvPreview() PreviewSpec {
	return PreviewSpec{
		Kind:             Csv,
		LanguageHint:     nil,
		CodeSyntax:       nil,
		CodeBackend:      Plain,
		StructuredFormat: nil,
		DocumentFormat:   nil,
	}
}

func SourcePreview(languageHint *string) PreviewSpec {
	return PreviewSpec{
		Kind:             Source,
		LanguageHint:     languageHint,
		CodeSyntax:       languageHint,
		CodeBackend:      Plain,
		StructuredFormat: nil,
		DocumentFormat:   nil,
	}
}

func CodePreview(codeSyntax string, codeBackend CodeBackend, structuredFormat *StructuredFormat) PreviewSpec {
	s := codeSyntax
	return PreviewSpec{
		Kind:             Source,
		LanguageHint:     &s,
		CodeSyntax:       &s,
		CodeBackend:      codeBackend,
		StructuredFormat: structuredFormat,
		DocumentFormat:   nil,
	}
}

func DocumentPreview(documentFormat DocumentFormat) PreviewSpec {
	df := documentFormat
	return PreviewSpec{
		Kind:             PlainText,
		LanguageHint:     nil,
		CodeSyntax:       nil,
		CodeBackend:      Plain,
		StructuredFormat: nil,
		DocumentFormat:   &df,
	}
}

func plain(class core.FileClass, specificTypeLabel *string) FileFacts {
	return FileFacts{
		BuiltinClass:      class,
		SpecificTypeLabel: specificTypeLabel,
		Preview:           PlainTextPreview(),
	}
}

func SourceOnly(
	class core.FileClass,
	specificTypeLabel *string,
	languageHint *string,
) FileFacts {
	return FileFacts{
		BuiltinClass:      class,
		SpecificTypeLabel: specificTypeLabel,
		Preview:           SourcePreview(languageHint),
	}
}

func DiskImageFileFacts(kind DiskImageKind) FileFacts {
	label := kind.DetailLabel()
	return plain(core.FileClassFile, &label)
}
