// classifies files based on their extension.
// Given an extension such as go, pdf, or png, it produces FileFacts describing:
// The broad class: code, document, image, archive, config, etc.
// A specific label: "Go source file", "PDF document", etc.
// The preview strategy: syntax-highlighted code, Markdown, CSV, SQLite, document rendering, or plain text.

package fileinfo

import (
	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/preview/code/registry"
)

func previewForExtension(ext string) PreviewSpec {
	language, ok := registry.LanguageForExtension(ext)
	if !ok {
		panic("extension registry entry should exist for code preview: " + ext)
	}
	return previewForLanguage(language)
}

func facts(class core.FileClass, label string, preview PreviewSpec) FileFacts {
	var specificTypeLabel *string
	if label != "" {
		specificTypeLabel = stringPtr(label)
	}
	return FileFacts{BuiltinClass: class, SpecificTypeLabel: specificTypeLabel, Preview: preview}
}

func codeFacts(ext, label string, class core.FileClass) FileFacts {
	return facts(class, label, previewForExtension(ext))
}

func inspectExtension(ext string) FileFacts {
	switch ext {
	case "md", "markdown", "mdown", "mkd", "mdx", "qmd":
		return facts(core.FileClassDocument, "", MarkdownPreview())
	case "iso":
		return facts(core.FileClassArchive, "ISO disk image", IsoPreview())
	case "torrent":
		return facts(core.FileClassData, "BitTorrent file", TorrentPreview())
	case "json":
		return codeFacts(ext, "JSON file", core.FileClassConfig)
	case "jsonc":
		return codeFacts(ext, "JSON with comments", core.FileClassConfig)
	case "json5":
		return codeFacts(ext, "JSON5 file", core.FileClassConfig)
	case "toml":
		return codeFacts(ext, "TOML file", core.FileClassConfig)
	case "yaml", "yml":
		return codeFacts(ext, "YAML file", core.FileClassConfig)
	case "html", "htm", "xhtml":
		return codeFacts(ext, "HTML document", core.FileClassCode)
	case "xml", "xsd", "xsl", "xslt":
		return codeFacts(ext, "XML document", core.FileClassCode)
	case "css":
		return codeFacts(ext, "Stylesheet", core.FileClassCode)
	case "scss":
		return codeFacts(ext, "SCSS stylesheet", core.FileClassCode)
	case "sass":
		return codeFacts(ext, "Sass stylesheet", core.FileClassCode)
	case "less":
		return codeFacts(ext, "Less stylesheet", core.FileClassCode)
	case "qml":
		return codeFacts(ext, "QML source file", core.FileClassCode)
	case "ts", "mts", "cts":
		return codeFacts(ext, "TypeScript source file", core.FileClassCode)
	case "tsx":
		return codeFacts(ext, "TSX source file", core.FileClassCode)
	case "jsx":
		return codeFacts(ext, "JSX source file", core.FileClassCode)
	case "mjs", "cjs":
		return codeFacts(ext, "JavaScript module", core.FileClassCode)
	case "js":
		return codeFacts(ext, "JavaScript source file", core.FileClassCode)
	case "sql":
		return codeFacts(ext, "SQL script", core.FileClassCode)
	case "diff":
		return codeFacts(ext, "Diff file", core.FileClassCode)
	case "patch":
		return codeFacts(ext, "Patch file", core.FileClassCode)
	case "tex", "ltx":
		return codeFacts(ext, "LaTeX document", core.FileClassDocument)
	case "bib":
		return codeFacts(ext, "BibTeX bibliography", core.FileClassDocument)
	case "sty":
		return codeFacts(ext, "TeX/LaTeX style file", core.FileClassDocument)
	case "cls":
		return codeFacts(ext, "TeX/LaTeX class file", core.FileClassDocument)
	case "nix":
		return codeFacts(ext, "Nix expression", core.FileClassConfig)
	case "hcl":
		return codeFacts(ext, "HCL config", core.FileClassConfig)
	case "tf":
		return codeFacts(ext, "Terraform module", core.FileClassConfig)
	case "tfvars":
		return codeFacts(ext, "Terraform variables", core.FileClassConfig)
	case "tfbackend":
		return codeFacts(ext, "Terraform backend config", core.FileClassConfig)
	case "cmake":
		return codeFacts(ext, "CMake script", core.FileClassConfig)
	case "lock":
		return codeFacts(ext, "Lockfile", core.FileClassData)
	case "ini":
		return codeFacts(ext, "INI config file", core.FileClassConfig)
	case "keys":
		return codeFacts(ext, "Keys file", core.FileClassConfig)
	case "conf", "cfg":
		return codeFacts(ext, "Config file", core.FileClassConfig)
	case "env":
		return codeFacts(ext, "Environment file", core.FileClassConfig)
	case "desktop":
		return codeFacts(ext, "Desktop Entry", core.FileClassConfig)
	case "svg":
		return codeFacts(ext, "SVG image", core.FileClassImage)
	case "raw":
		return DiskImageFileFacts(RawImage)
	case "img":
		return DiskImageFileFacts(DiskImage)
	case "qcow2":
		return DiskImageFileFacts(Qcow2)
	case "vmdk":
		return DiskImageFileFacts(Vmdk)
	case "vdi":
		return DiskImageFileFacts(Vdi)
	case "vhd":
		return DiskImageFileFacts(Vhd)
	case "vhdx":
		return DiskImageFileFacts(Vhdx)
	case "log":
		return codeFacts(ext, "Log file", core.FileClassDocument)
	}

	if result, ok := inspectProgrammingExtension(ext); ok {
		return result
	}
	if result, ok := inspectDataOrDocumentExtension(ext); ok {
		return result
	}
	if class, label, ok := plainExtension(ext); ok {
		return facts(class, label, PlainTextPreview())
	}
	return plain(core.FileClassFile, nil)
}

func inspectProgrammingExtension(ext string) (FileFacts, bool) {
	labels := map[string]string{
		"c": "C source file", "h": "C header", "cpp": "C++ source file", "cc": "C++ source file", "cxx": "C++ source file",
		"hpp": "C++ header", "hh": "C++ header", "hxx": "C++ header", "mk": "Makefile", "mak": "Makefile",
		"sh": "Shell script", "bash": "Bash script", "zsh": "Zsh script", "ksh": "KornShell script", "fish": "Fish script",
		"ps1": "PowerShell script", "psm1": "PowerShell module", "psd1": "PowerShell data file",
		"py": "Python source file", "pyi": "Python stub file", "pyw": "Python script (no console)", "pyx": "Cython source file",
		"rs": "Rust source file", "go": "Go source file", "java": "Java source file", "php": "PHP script", "swift": "Swift source file",
		"kt": "Kotlin source file", "kts": "Kotlin script", "cs": "C# source file", "csx": "C# script", "dart": "Dart source file",
		"f": "Fortran source file", "for": "Fortran source file", "f90": "Fortran source file", "f95": "Fortran source file", "f03": "Fortran source file", "f08": "Fortran source file", "fpp": "Fortran preprocessor source file",
		"cbl": "COBOL source file", "cob": "COBOL source file", "cobol": "COBOL source file", "cpy": "COBOL copybook", "zig": "Zig source file",
		"groovy": "Groovy source file", "gvy": "Groovy source file", "gradle": "Gradle build script", "scala": "Scala source file", "sbt": "sbt build definition",
		"pl": "Perl script", "pm": "Perl module", "pod": "Perl POD file", "t": "Perl test script", "hs": "Haskell source file", "lhs": "Literate Haskell source file",
		"jl": "Julia source file", "r": "R script", "ex": "Elixir source file", "exs": "Elixir script", "clj": "Clojure source file", "cljs": "ClojureScript source file", "cljc": "Portable Clojure source file",
		"edn": "EDN file", "rb": "Ruby script", "lua": "Lua script",
	}
	label, ok := labels[ext]
	if !ok {
		return FileFacts{}, false
	}
	class := core.FileClassCode
	if ext == "mk" || ext == "mak" || ext == "psd1" || ext == "gradle" || ext == "sbt" || ext == "edn" {
		class = core.FileClassConfig
	}
	return codeFacts(ext, label, class), true
}

func inspectDataOrDocumentExtension(ext string) (FileFacts, bool) {
	switch ext {
	case "ron":
		return SourceOnly(core.FileClassConfig, nil, nil), true
	case "csv":
		return facts(core.FileClassData, "CSV file", CsvPreview()), true
	case "tsv":
		return facts(core.FileClassData, "TSV file", CsvPreview()), true
	case "sqlite", "sqlite3", "db3":
		return facts(core.FileClassData, "SQLite database", SqlitePreview()), true
	case "db":
		return facts(core.FileClassData, "Database file", SqliteCandidatePreview()), true
	case "sqlite-wal", "db-wal":
		return plain(core.FileClassData, stringPtr("SQLite WAL")), true
	case "sqlite-shm", "db-shm":
		return plain(core.FileClassData, stringPtr("SQLite shared memory")), true
	case "sqlite-journal", "db-journal":
		return plain(core.FileClassData, stringPtr("SQLite rollback journal")), true
	case "parquet":
		return SourceOnly(core.FileClassData, stringPtr("Parquet file"), nil), true
	}
	formats := map[string]struct {
		format DocumentFormat
		label  string
	}{
		"doc": {Doc, "DOC document"}, "docx": {Docx, "DOCX document"}, "docm": {Docm, "DOCM document"},
		"odt": {Odt, "ODT document"}, "ods": {Ods, "ODS spreadsheet"}, "odp": {Odp, "ODP presentation"},
		"pptx": {Pptx, "PPTX presentation"}, "pptm": {Pptm, "PPTM presentation"}, "xlsx": {Xlsx, "XLSX spreadsheet"},
		"xlsm": {Xlsm, "XLSM spreadsheet"}, "pages": {Pages, "Pages document"}, "epub": {Epub, "EPUB ebook"},
		"mobi": {Mobi, "MOBI ebook"}, "azw3": {Azw3, "AZW3 ebook"}, "pdf": {Pdf, "PDF document"},
	}
	if document, ok := formats[ext]; ok {
		return facts(core.FileClassDocument, document.label, DocumentPreview(document.format)), true
	}
	return FileFacts{}, false
}

func plainExtension(ext string) (core.FileClass, string, bool) {
	values := map[string]struct {
		class core.FileClass
		label string
	}{
		"mp4": {core.FileClassVideo, "MP4 video"}, "mkv": {core.FileClassVideo, "Matroska video"}, "mov": {core.FileClassVideo, "QuickTime video"}, "webm": {core.FileClassVideo, "WebM video"}, "avi": {core.FileClassVideo, "AVI video"},
		"xcf": {core.FileClassImage, "GIMP image"}, "ico": {core.FileClassImage, "ICO image"}, "svg": {core.FileClassImage, "SVG image"}, "png": {core.FileClassImage, "PNG image"}, "jpg": {core.FileClassImage, "JPEG image"}, "jpeg": {core.FileClassImage, "JPEG image"}, "gif": {core.FileClassImage, "GIF image"}, "webp": {core.FileClassImage, "WebP image"}, "avif": {core.FileClassImage, "AVIF image"},
		"mp3": {core.FileClassAudio, "MP3 audio"}, "wav": {core.FileClassAudio, "WAV audio"}, "flac": {core.FileClassAudio, "FLAC audio"}, "ogg": {core.FileClassAudio, "Ogg audio"}, "m4a": {core.FileClassAudio, "M4A audio"},
		"rpm": {core.FileClassArchive, "RPM package"}, "deb": {core.FileClassArchive, "Debian package"}, "apk": {core.FileClassArchive, "Android package"}, "aab": {core.FileClassArchive, "Android App Bundle"}, "apkg": {core.FileClassArchive, "Anki package"}, "zst": {core.FileClassArchive, "Zstandard archive"}, "zest": {core.FileClassArchive, "Zest archive"}, "appimage": {core.FileClassArchive, "AppImage bundle"}, "jar": {core.FileClassArchive, "Java archive"}, "cbz": {core.FileClassArchive, "Comic ZIP archive"}, "cbr": {core.FileClassArchive, "Comic RAR archive"},
		"hash": {core.FileClassData, "Hash file"}, "sha1": {core.FileClassData, "SHA-1 checksum"}, "sha256": {core.FileClassData, "SHA-256 checksum"}, "sha512": {core.FileClassData, "SHA-512 checksum"}, "md5": {core.FileClassData, "MD5 checksum"},
		"srt": {core.FileClassDocument, "SubRip subtitles"}, "p12": {core.FileClassConfig, "PKCS#12 certificate"}, "pfx": {core.FileClassConfig, "PKCS#12 certificate"}, "pem": {core.FileClassConfig, "PEM certificate"}, "crt": {core.FileClassConfig, "Certificate"}, "cer": {core.FileClassConfig, "Certificate"}, "csr": {core.FileClassConfig, "Certificate signing request"}, "key": {core.FileClassConfig, "Private key"},
		"exe": {core.FileClassFile, "Windows executable"}, "dll": {core.FileClassFile, "Windows DLL"}, "sys": {core.FileClassFile, "Windows system driver"}, "msi": {core.FileClassFile, "Windows Installer package"}, "so": {core.FileClassFile, "Shared library"}, "dylib": {core.FileClassFile, "Dynamic library"}, "o": {core.FileClassFile, "Object file"}, "a": {core.FileClassFile, "Static library"}, "lib": {core.FileClassFile, "Library file"},
		"ttf": {core.FileClassFont, "TrueType font"}, "otf": {core.FileClassFont, "OpenType font"}, "woff": {core.FileClassFont, "WOFF font"}, "woff2": {core.FileClassFont, "WOFF2 font"},
	}
	if value, ok := values[ext]; ok {
		return value.class, value.label, true
	}
	if ext == "txt" || ext == "rst" {
		return core.FileClassDocument, "", true
	}
	if ext == "zip" || ext == "tar" || ext == "gz" || ext == "xz" || ext == "bz2" || ext == "7z" || ext == "rar" {
		return core.FileClassArchive, "", true
	}
	return 0, "", false
}
