package fileinfo

// This file classifies filesystem entries by name, extension, and file content,
// selects an appropriate preview format, and caches results for unchanged files.

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/urdadx/nukri/internal/core"
)

const ConfigSniffByteLimit = 16 * 1024
const ConfigSniffLineLimit = 80
const ConfigHintLineLimit = 10
const FileFactsCacheLimit = 4096
const StrongIniThreshold uint8 = 4
const StrongShellThreshold uint8 = 4
const ScoreMargin uint8 = 2

type CacheKey struct {
	Path           string
	DisplayName    string
	HasDisplayName bool
	IsDir          bool
	Size           uint64
	MtimeSec       int64
	MtimeNsec      uint32
	HasMtime       bool
}

type FactsCache struct {
	mu    sync.Mutex // it locks access to the data while map is being read or written
	Facts map[CacheKey]FileFacts
	Order []CacheKey
}

var fileFactsCache = FactsCache{Facts: make(map[CacheKey]FileFacts)}

func InspectPath(path string, kind core.EntryKind) FileFacts {
	return inspectPathWithName(path, "", false, kind)
}

func InspectEntryFast(entry *core.Entry) FileFacts {
	return inspectPathWithNameFast(entry.Path, entry.Name, true, entry.Kind)
}

func inspectPathWithName(path, displayName string, hasDisplayName bool, kind core.EntryKind) FileFacts {
	_, name, ext, facts := inspectPathWithNameBase(path, displayName, hasDisplayName, kind)
	switch ext {
	case "":
		if sniffed, ok := sniffExtensionlessFileType(path); ok {
			facts = sniffed
		}
	case "conf", "cfg":
		if sniffed, ok := sniffConfigFileType(path); ok {
			facts = sniffed
		}
	}
	if sniffed, ok := sniffLicenseFileType(path, name, ext, facts); ok {
		return sniffed
	}
	return facts
}

func inspectPathWithNameFast(path, displayName string, hasDisplayName bool, kind core.EntryKind) FileFacts {
	_, name, ext, facts := inspectPathWithNameBase(path, displayName, hasDisplayName, kind)
	if sniffed, ok := sniffBrowserLicenseFileType(name, ext, facts); ok {
		return sniffed
	}
	return facts
}

func inspectPathWithNameBase(path, displayName string, hasDisplayName bool, kind core.EntryKind) (string, string, string, FileFacts) {
	if kind == core.Directory {
		return "", "", "", FileFacts{BuiltinClass: core.FileClassDirectory, Preview: PlainTextPreview()}
	}

	nameForType := displayName
	if !hasDisplayName {
		nameForType = filepath.Base(path)
		if nameForType == "." || nameForType == string(filepath.Separator) {
			nameForType = ""
		}
	}
	name := normalizeKey(nameForType)
	if facts, ok := inspectExactName(name); ok {
		return nameForType, name, "", facts
	}
	if facts, ok := inspectArchiveName(name); ok {
		return nameForType, name, "", facts
	}

	ext := pathExtension(nameForType)
	return nameForType, name, ext, inspectExtension(ext)
}

func pathExtension(path string) string {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") && !strings.Contains(base[1:], ".") {
		return ""
	}
	return normalizeKey(strings.TrimPrefix(filepath.Ext(base), "."))
}

func InspectPathCached(path string, kind core.EntryKind, size uint64, modified *time.Time) FileFacts {
	return inspectPathWithNameCached(path, "", false, kind, size, modified)
}

func InspectEntryCached(entry *core.Entry) FileFacts {
	size := uint64(0)
	if entry.Size > 0 {
		size = uint64(entry.Size)
	}
	var modified *time.Time
	if !entry.Modified.IsZero() {
		modified = &entry.Modified
	}
	return inspectPathWithNameCached(entry.Path, entry.Name, true, entry.Kind, size, modified)
}

func inspectPathWithNameCached(path, displayName string, hasDisplayName bool, kind core.EntryKind, size uint64, modified *time.Time) FileFacts {
	key := CacheKey{Path: path, HasDisplayName: hasDisplayName, IsDir: kind == core.Directory, Size: size}
	if hasDisplayName {
		key.DisplayName = displayName
	}
	if modified != nil && !modified.Before(time.Unix(0, 0)) {
		key.MtimeSec = modified.Unix()
		key.MtimeNsec = uint32(modified.Nanosecond())
		key.HasMtime = true
	}

	if facts, ok := fileFactsCache.get(key); ok {
		return facts
	}
	facts := inspectPathWithName(path, displayName, hasDisplayName, kind)
	fileFactsCache.insert(key, facts)
	return facts
}

func (c *FactsCache) get(key CacheKey) (FileFacts, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	facts, ok := c.Facts[key]
	return facts, ok
}

func (c *FactsCache) insert(key CacheKey, facts FileFacts) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Facts[key] = facts
	for i, cached := range c.Order {
		if cached == key {
			c.Order = append(c.Order[:i], c.Order[i+1:]...)
			break
		}
	}
	c.Order = append(c.Order, key)
	for len(c.Order) > FileFactsCacheLimit {
		stale := c.Order[0]
		c.Order = c.Order[1:]
		delete(c.Facts, stale)
	}
}

func normalizeKey(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func sniffExtensionlessFileType(path string) (FileFacts, bool) {
	if !isRegularFile(path) {
		return FileFacts{}, false
	}
	file, err := os.Open(path)
	if err != nil {
		return FileFacts{}, false
	}
	defer file.Close()
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return FileFacts{}, false
	}
	prefix := buffer[:n]
	if facts, ok := sniffImageType(prefix); ok {
		return facts, true
	}
	return sniffShebangScriptType(prefix)
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func sniffImageType(buffer []byte) (FileFacts, bool) {
	switch {
	case bytes.HasPrefix(buffer, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return imageFacts("PNG image"), true
	case bytes.HasPrefix(buffer, []byte{0xff, 0xd8, 0xff}):
		return imageFacts("JPEG image"), true
	case bytes.HasPrefix(buffer, []byte("GIF87a")), bytes.HasPrefix(buffer, []byte("GIF89a")):
		return imageFacts("GIF image"), true
	case len(buffer) >= 12 && bytes.Equal(buffer[:4], []byte("RIFF")) && bytes.Equal(buffer[8:12], []byte("WEBP")):
		return imageFacts("WebP image"), true
	}
	if !utf8.Valid(buffer) {
		return FileFacts{}, false
	}
	text := strings.TrimLeftFunc(string(buffer), func(r rune) bool { return r == '\ufeff' || r <= unicode.MaxASCII && unicode.IsSpace(r) })
	if strings.HasPrefix(text, "<svg") || strings.HasPrefix(text, "<?xml") && strings.Contains(text, "<svg") {
		return imageFacts("SVG image"), true
	}
	return FileFacts{}, false
}

func imageFacts(label string) FileFacts {
	return FileFacts{BuiltinClass: core.FileClassImage, SpecificTypeLabel: stringPtr(label), Preview: PlainTextPreview()}
}

func sniffShebangScriptType(buffer []byte) (FileFacts, bool) {
	if !utf8.Valid(buffer) {
		return FileFacts{}, false
	}
	firstLine, _, _ := strings.Cut(string(buffer), "\n")
	interpreter, ok := shebangInterpreterName(strings.TrimPrefix(firstLine, "\ufeff"))
	if !ok {
		return FileFacts{}, false
	}
	language, ok := languageForShebang(interpreter)
	if !ok {
		return FileFacts{}, false
	}
	labels := map[string]string{
		"bash": "Bash script", "zsh": "Zsh script", "ksh": "KornShell script", "sh": "Shell script",
		"fish": "Fish script", "elixir": "Elixir script", "groovy": "Groovy script", "perl": "Perl script",
		"haskell": "Haskell script", "julia": "Julia script", "r": "R script", "powershell": "PowerShell script",
		"clojure": "Clojure script",
	}
	label, ok := labels[language.canonicalID]
	if !ok {
		return FileFacts{}, false
	}
	return FileFacts{BuiltinClass: core.FileClassCode, SpecificTypeLabel: stringPtr(label), Preview: language.previewSpec()}, true
}

func shebangInterpreterName(firstLine string) (string, bool) {
	command, ok := strings.CutPrefix(firstLine, "#!")
	if !ok {
		return "", false
	}
	tokens := strings.Fields(command)
	if len(tokens) == 0 {
		return "", false
	}
	program := filepath.Base(tokens[0])
	if program != "env" {
		return program, program != "."
	}
	for _, token := range tokens[1:] {
		if !strings.HasPrefix(token, "-") {
			program = filepath.Base(token)
			return program, program != "."
		}
	}
	return "", false
}

func sniffConfigFileType(path string) (FileFacts, bool) {
	prefix, ok := readTextPrefix(path)
	if !ok {
		return FileFacts{}, false
	}
	if hint, ok := detectConfigHint(prefix); ok {
		return hint, true
	}
	iniScore, shellScore := scoreConfigPrefix(prefix)
	if iniScore >= StrongIniThreshold && iniScore >= saturatingAdd(shellScore, ScoreMargin) {
		language, ok := languageForCodeSyntax("ini")
		return configFileFacts(language), ok
	}
	if shellScore >= StrongShellThreshold && shellScore >= saturatingAdd(iniScore, ScoreMargin) {
		language, ok := languageForCodeSyntax("sh")
		return configFileFacts(language), ok
	}
	language, ok := languageForCodeSyntax("config")
	return configFileFacts(language), ok
}

func readTextPrefix(path string) (string, bool) {
	if !isRegularFile(path) {
		return "", false
	}
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()
	buffer := make([]byte, ConfigSniffByteLimit)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", false
	}
	return strings.ToValidUTF8(string(buffer[:n]), "\ufffd"), true
}

func detectConfigHint(prefix string) (FileFacts, bool) {
	for i, line := range strings.Split(prefix, "\n") {
		if i >= ConfigHintLineLimit {
			break
		}
		if hint, ok := extractModeHint(strings.TrimSuffix(line, "\r")); ok {
			if facts, ok := configFactsFromHint(hint); ok {
				return facts, true
			}
		}
	}
	return FileFacts{}, false
}

func extractModeHint(line string) (string, bool) {
	if hint, ok := extractEmacsModeHint(line); ok {
		return hint, true
	}
	return extractVimModeHint(line)
}

func extractEmacsModeHint(line string) (string, bool) {
	_, after, ok := strings.Cut(line, "-*-")
	if !ok {
		return "", false
	}
	rest := after
	before, _, ok0 := strings.Cut(rest, "-*-")
	if !ok0 {
		return "", false
	}
	payload := strings.TrimSpace(before)
	if payload == "" {
		return "", false
	}
	for _, part := range strings.Split(payload, ";") {
		trimmed := strings.TrimSpace(part)
		if strings.HasPrefix(trimmed, "mode:") || strings.HasPrefix(trimmed, "Mode:") {
			return strings.TrimSpace(trimmed[5:]), true
		}
	}
	token := strings.Fields(payload)[0]
	return token, !strings.Contains(token, ":")
}

func extractVimModeHint(line string) (string, bool) {
	lower := strings.ToLower(line)
	for _, needle := range []string{"filetype=", "syntax=", "ft="} {
		if index := strings.Index(lower, needle); index >= 0 {
			value := line[index+len(needle):]
			end := strings.IndexFunc(value, func(r rune) bool {
				return !isASCIIAlphaNumeric(r) && r != '_' && r != '-' && r != '+'
			})
			if end >= 0 {
				value = value[:end]
			}
			value = strings.TrimSpace(value)
			return value, value != ""
		}
	}
	return "", false
}

func configFactsFromHint(token string) (FileFacts, bool) {
	language, ok := languageForModeline(token)
	return configFileFacts(language), ok
}

func configFileFacts(language registeredLanguage) FileFacts {
	return FileFacts{BuiltinClass: core.FileClassConfig, Preview: language.previewSpec()}
}

func scoreConfigPrefix(prefix string) (uint8, uint8) {
	var iniSections, iniAssignments, iniSemicolonComments uint8
	var shellExpansions, shellControls, shellAssignments uint8
	for i, line := range strings.Split(prefix, "\n") {
		if i >= ConfigSniffLineLimit {
			break
		}
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, ";") {
			iniSemicolonComments = saturatingAdd(iniSemicolonComments, 1)
			continue
		}
		if looksLikeINISection(trimmed) {
			iniSections = saturatingAdd(iniSections, 1)
			continue
		}
		if looksLikeINIAssignment(trimmed) {
			iniAssignments = saturatingAdd(iniAssignments, 1)
		}
		if looksLikeShellExpansion(trimmed) {
			shellExpansions = saturatingAdd(shellExpansions, 1)
		}
		if looksLikeShellControl(trimmed) {
			shellControls = saturatingAdd(shellControls, 1)
		}
		if looksLikeShellAssignment(trimmed) {
			shellAssignments = saturatingAdd(shellAssignments, 1)
		}
	}
	iniScore := 4*min(iniSections, 1) + min(iniAssignments, 2) + min(iniSemicolonComments, 2)
	shellScore := 3*min(shellExpansions, 1) + 3*min(shellControls, 1) + min(shellAssignments, 2)
	return iniScore, shellScore
}

func looksLikeINISection(line string) bool {
	return len(line) > 2 && strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") && !strings.Contains(line, "\n")
}

func looksLikeINIAssignment(line string) bool {
	left, _, ok := strings.Cut(line, "=")
	if !ok {
		return false
	}
	key := strings.TrimSpace(left)
	return key != "" && strings.IndexFunc(key, func(r rune) bool {
		return !isASCIIAlphaNumeric(r) && r != '_' && r != '-' && r != '.'
	}) < 0
}

func looksLikeShellExpansion(line string) bool {
	for _, token := range []string{"${", "$(", "$((", "`", "&&", "||", "[[", "]]"} {
		if strings.Contains(line, token) {
			return true
		}
	}
	return false
}

func looksLikeShellControl(line string) bool {
	for _, prefix := range []string{"export ", "if ", "for ", "while ", "case "} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	switch line {
	case "then", "do", "done", "fi", "esac":
		return true
	}
	return strings.Contains(line, "; then") || strings.Contains(line, "; do")
}

func looksLikeShellAssignment(line string) bool {
	left, _, ok := strings.Cut(line, "=")
	if !ok || strings.TrimSpace(left) == "" || strings.IndexFunc(left, unicode.IsSpace) >= 0 {
		return false
	}
	return strings.IndexFunc(left, func(r rune) bool { return !isASCIIAlphaNumeric(r) && r != '_' }) < 0
}

func isASCIIAlphaNumeric(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

func saturatingAdd(a, b uint8) uint8 {
	if a > ^uint8(0)-b {
		return ^uint8(0)
	}
	return a + b
}

type registeredLanguage struct {
	canonicalID string
	codeSyntax  string
	backend     CodeBackend
	structured  *StructuredFormat
}

func (l registeredLanguage) previewSpec() PreviewSpec {
	return CodePreview(l.codeSyntax, l.backend, l.structured)
}

var languages = map[string]registeredLanguage{
	"bash":       {canonicalID: "bash", codeSyntax: "bash", backend: Chroma},
	"zsh":        {canonicalID: "zsh", codeSyntax: "zsh", backend: Chroma},
	"ksh":        {canonicalID: "ksh", codeSyntax: "bash", backend: Chroma},
	"sh":         {canonicalID: "sh", codeSyntax: "sh", backend: Chroma},
	"fish":       {canonicalID: "fish", codeSyntax: "fish", backend: Chroma},
	"elixir":     {canonicalID: "elixir", codeSyntax: "elixir", backend: Chroma},
	"groovy":     {canonicalID: "groovy", codeSyntax: "groovy", backend: Chroma},
	"perl":       {canonicalID: "perl", codeSyntax: "perl", backend: Chroma},
	"haskell":    {canonicalID: "haskell", codeSyntax: "haskell", backend: Chroma},
	"julia":      {canonicalID: "julia", codeSyntax: "julia", backend: Chroma},
	"r":          {canonicalID: "r", codeSyntax: "r", backend: Chroma},
	"powershell": {canonicalID: "powershell", codeSyntax: "powershell", backend: Chroma},
	"clojure":    {canonicalID: "clojure", codeSyntax: "clojure", backend: Chroma},
	"ini":        {canonicalID: "ini", codeSyntax: "ini", backend: Custom},
	"config":     {canonicalID: "config", codeSyntax: "config", backend: Custom},
}

func languageForShebang(interpreter string) (registeredLanguage, bool) {
	aliases := map[string]string{
		"dash": "sh", "ash": "sh", "elixir": "elixir", "groovy": "groovy", "perl": "perl",
		"runhaskell": "haskell", "julia": "julia", "Rscript": "r", "pwsh": "powershell",
		"powershell": "powershell", "clojure": "clojure",
	}
	key := interpreter
	if alias, ok := aliases[interpreter]; ok {
		key = alias
	}
	language, ok := languages[key]
	return language, ok
}

func languageForCodeSyntax(syntax string) (registeredLanguage, bool) {
	language, ok := languages[normalizeKey(syntax)]
	return language, ok
}

func languageForModeline(token string) (registeredLanguage, bool) {
	aliases := map[string]string{"shell": "sh", "shell-script": "sh", "conf": "config", "cfg": "config", "dosini": "ini"}
	key := normalizeKey(token)
	if alias, ok := aliases[key]; ok {
		key = alias
	}
	return languageForCodeSyntax(key)
}

func inspectExactName(name string) (FileFacts, bool) {
	codeNames := map[string]string{"makefile": "make", "gnumakefile": "make", "dockerfile": "docker", "jenkinsfile": "groovy"}
	if syntax, ok := codeNames[name]; ok {
		label := name + " file"
		return FileFacts{BuiltinClass: core.FileClassCode, SpecificTypeLabel: &label, Preview: CodePreview(syntax, Chroma, nil)}, true
	}
	return FileFacts{}, false
}

func inspectExtension(ext string) FileFacts {
	imageLabels := map[string]string{"png": "PNG image", "jpg": "JPEG image", "jpeg": "JPEG image", "gif": "GIF image", "webp": "WebP image", "svg": "SVG image"}
	if label, ok := imageLabels[ext]; ok {
		return imageFacts(label)
	}
	archiveLabels := map[string]string{"zip": "ZIP archive", "tar": "TAR archive", "gz": "Gzip archive", "xz": "XZ archive", "bz2": "Bzip2 archive", "zst": "Zstandard archive", "7z": "7-Zip archive", "rar": "RAR archive"}
	if label, ok := archiveLabels[ext]; ok {
		return plain(core.FileClassArchive, stringPtr(label))
	}
	codeSyntax := map[string]string{"go": "go", "rs": "rust", "py": "python", "js": "javascript", "ts": "typescript", "jsx": "jsx", "tsx": "tsx", "c": "c", "h": "c", "cpp": "cpp", "hpp": "cpp", "java": "java", "rb": "ruby", "php": "php", "pl": "perl", "ex": "elixir", "exs": "elixir", "sh": "sh", "bash": "bash", "zsh": "zsh", "fish": "fish", "sql": "sql"}
	if syntax, ok := codeSyntax[ext]; ok {
		return FileFacts{BuiltinClass: core.FileClassCode, Preview: CodePreview(syntax, Chroma, nil)}
	}
	if ext == "conf" || ext == "cfg" || ext == "ini" {
		return FileFacts{BuiltinClass: core.FileClassConfig, Preview: CodePreview(ext, Custom, nil)}
	}
	return plain(core.FileClassFile, nil)
}

func sniffBrowserLicenseFileType(name, ext string, facts FileFacts) (FileFacts, bool) {
	base := strings.TrimSuffix(name, "."+ext)
	if base == "license" || base == "licence" || base == "copying" || base == "copyright" {
		return FileFacts{BuiltinClass: core.FileClassLicense, SpecificTypeLabel: stringPtr("License file"), Preview: PlainTextPreview()}, true
	}
	return facts, false
}

func sniffLicenseFileType(path, name, ext string, facts FileFacts) (FileFacts, bool) {
	if sniffed, ok := sniffBrowserLicenseFileType(name, ext, facts); ok {
		return sniffed, true
	}
	return facts, false
}

func stringPtr(value string) *string {
	return &value
}
