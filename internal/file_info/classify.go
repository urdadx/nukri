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
	"github.com/urdadx/nukri/internal/preview/code/registry"
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
	return SniffLicenseFileType(path, name, ext, facts)
}

func inspectPathWithNameFast(path, displayName string, hasDisplayName bool, kind core.EntryKind) FileFacts {
	_, name, ext, facts := inspectPathWithNameBase(path, displayName, hasDisplayName, kind)
	return SniffBrowserLicenseFileType(path, name, ext, facts)
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
	language, ok := registry.LanguageForShebangInterpreter(interpreter)
	if !ok {
		return FileFacts{}, false
	}
	label := language.DisplayLabel + " script"
	return FileFacts{BuiltinClass: core.FileClassCode, SpecificTypeLabel: stringPtr(label), Preview: previewForLanguage(language)}, true
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
		language, ok := registry.LanguageForCodeSyntax("ini")
		return configFileFacts(language), ok
	}
	if shellScore >= StrongShellThreshold && shellScore >= saturatingAdd(iniScore, ScoreMargin) {
		language, ok := registry.LanguageForCodeSyntax("sh")
		return configFileFacts(language), ok
	}
	language, ok := registry.LanguageForCodeSyntax("config")
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
	language, ok := registry.LanguageForModeline(token)
	return configFileFacts(language), ok
}

func configFileFacts(language registry.RegisteredLanguage) FileFacts {
	return FileFacts{BuiltinClass: core.FileClassConfig, Preview: previewForLanguage(language)}
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

func stringPtr(value string) *string {
	return &value
}
