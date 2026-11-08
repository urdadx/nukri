package preview

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strings"
)

func (s *Service) renderTorrent(ctx context.Context, path string) (*TorrentPreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open torrent: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, s.maxTorrentBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read torrent metainfo: %w", err)
	}
	if int64(len(data)) > s.maxTorrentBytes {
		return nil, ErrOutputTooLarge
	}
	meta, err := parseTorrentMeta(bytes.TrimSpace(data))
	if err != nil {
		return nil, fmt.Errorf("parse torrent metainfo: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := parseTorrentInfo(meta.info)
	if err != nil {
		return nil, fmt.Errorf("parse torrent info: %w", err)
	}
	if info.pieceLength < 0 || info.length < 0 {
		return nil, fmt.Errorf("parse torrent info: negative length")
	}

	value := Torrent{
		Name: safeText(info.bestName()), Comment: safeText(meta.comment),
		CreatedBy: safeText(meta.createdBy), CreationDate: meta.creationDate,
		PieceLength: info.pieceLength, PieceCount: info.numPieces(),
		Private: info.private,
	}
	if info.hasV1() {
		sum := sha1.Sum(meta.info)
		value.InfoHashV1 = hex.EncodeToString(sum[:])
	}
	if info.hasV2() {
		sum := sha256.Sum256(meta.info)
		value.InfoHashV2 = hex.EncodeToString(sum[:])
	}
	value.Trackers = torrentTrackers(meta.announceTiers(), s.maxTorrentTrackers)
	for _, seed := range meta.urlList {
		value.WebSeeds = append(value.WebSeeds, safeText(seed))
	}

	allFiles := info.files
	files := make([]TorrentFile, 0, min(len(allFiles), s.maxTorrentFiles))
	for index := range allFiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fileInfo := &allFiles[index]
		if fileInfo.length < 0 || value.TotalSize > math.MaxInt64-fileInfo.length {
			return nil, fmt.Errorf("parse torrent info: invalid total file size")
		}
		value.TotalSize += fileInfo.length
		if len(files) >= s.maxTorrentFiles {
			continue
		}
		filePath := value.Name
		if info.isDir() {
			filePath = safeText(strings.Join(fileInfo.path, "/"))
		}
		if filePath == "" {
			filePath = value.Name
		}
		files = append(files, TorrentFile{Path: filePath, Size: fileInfo.length})
	}
	if len(allFiles) == 0 && info.length != 0 {
		value.TotalSize = info.length
		files = append(files, TorrentFile{Path: value.Name, Size: info.length})
	}
	result := &TorrentPreview{Torrent: value, Files: files, Truncated: len(allFiles) > len(files)}
	result.Metadata = torrentFields(value, max(len(allFiles), len(files)))
	return result, nil
}

// torrentMeta is the outer metainfo dictionary (BEP 3, 12, 19).
type torrentMeta struct {
	info         []byte // raw bytes of the info dictionary, hashed for the info hash
	announce     string
	announceList [][]string
	creationDate int64
	comment      string
	createdBy    string
	urlList      []string
}

// torrentInfo is the info dictionary (BEP 3, 27, 52).
type torrentInfo struct {
	name, nameUTF8 string
	pieceLength    int64
	piecesBytes    int
	length         int64
	metaVersion    int64
	private        bool
	hasFileList    bool // the v1 "files" key is present
	v1Files        int
	treeIsDir      bool
	treePieceCount int
	files          []torrentFileInfo
}

type torrentFileInfo struct {
	length int64
	path   []string
}

func parseTorrentMeta(data []byte) (torrentMeta, error) {
	var meta torrentMeta
	if err := bencodeValidate(data); err != nil {
		return meta, err
	}
	values, _, err := bencodeDict(data)
	if err != nil {
		return meta, err
	}
	info, ok := values["info"]
	if !ok {
		return meta, fmt.Errorf("missing info dictionary")
	}
	meta.info = info
	if err := decodeOptional(values, "announce", bencodeString, &meta.announce); err != nil {
		return meta, err
	}
	if raw, ok := values["announce-list"]; ok {
		tiers, err := bencodeList(raw)
		if err != nil {
			return meta, err
		}
		for _, tier := range tiers {
			urls, err := bencodeStrings(tier)
			if err != nil {
				return meta, err
			}
			meta.announceList = append(meta.announceList, urls)
		}
	}
	// A malformed creation date is ignored rather than rejecting the file.
	if raw, ok := values["creation date"]; ok {
		meta.creationDate, _ = bencodeInt(raw)
	}
	if err := decodeOptional(values, "comment", bencodeString, &meta.comment); err != nil {
		return meta, err
	}
	if err := decodeOptional(values, "created by", bencodeString, &meta.createdBy); err != nil {
		return meta, err
	}
	// BEP 19 allows url-list to be a single string or a list of strings.
	if raw, ok := values["url-list"]; ok {
		if raw[0] == 'l' {
			meta.urlList, err = bencodeStrings(raw)
		} else {
			var url string
			url, err = bencodeString(raw)
			meta.urlList = []string{url}
		}
		if err != nil {
			return meta, err
		}
	}
	return meta, nil
}

func parseTorrentInfo(raw []byte) (torrentInfo, error) {
	var info torrentInfo
	values, _, err := bencodeDict(raw)
	if err != nil {
		return info, err
	}
	var pieces string
	var private int64
	for _, field := range []error{
		decodeOptional(values, "name", bencodeString, &info.name),
		decodeOptional(values, "name.utf-8", bencodeString, &info.nameUTF8),
		decodeOptional(values, "piece length", bencodeInt, &info.pieceLength),
		decodeOptional(values, "pieces", bencodeString, &pieces),
		decodeOptional(values, "length", bencodeInt, &info.length),
		decodeOptional(values, "meta version", bencodeInt, &info.metaVersion),
		decodeOptional(values, "private", bencodeInt, &private),
	} {
		if field != nil {
			return info, field
		}
	}
	info.piecesBytes = len(pieces)
	info.private = private != 0

	var v1Files []torrentFileInfo
	if raw, ok := values["files"]; ok {
		info.hasFileList = true
		v1Files, err = parseTorrentFileList(raw)
		if err != nil {
			return info, err
		}
		info.v1Files = len(v1Files)
	}
	if !info.hasV2() {
		if len(v1Files) == 0 {
			// Single-file torrent: the file is named by the torrent name.
			info.files = []torrentFileInfo{{length: info.length}}
		}
		info.files = append(info.files, v1Files...)
		return info, nil
	}
	if raw, ok := values["file tree"]; ok {
		root, keys, err := bencodeDict(raw)
		if err != nil {
			return info, err
		}
		info.treeIsDir = slices.ContainsFunc(keys, func(key string) bool { return key != "" })
		if err := info.walkFileTree(root, keys, nil, 0); err != nil {
			return info, err
		}
	}
	return info, nil
}

func parseTorrentFileList(raw []byte) ([]torrentFileInfo, error) {
	items, err := bencodeList(raw)
	if err != nil {
		return nil, err
	}
	files := make([]torrentFileInfo, 0, len(items))
	for _, item := range items {
		values, _, err := bencodeDict(item)
		if err != nil {
			return nil, err
		}
		var file torrentFileInfo
		var path, pathUTF8 []string
		for _, field := range []error{
			decodeOptional(values, "length", bencodeInt, &file.length),
			decodeOptional(values, "path", bencodeStrings, &path),
			decodeOptional(values, "path.utf-8", bencodeStrings, &pathUTF8),
		} {
			if field != nil {
				return nil, field
			}
		}
		file.path = path
		if len(pathUTF8) != 0 {
			file.path = pathUTF8
		}
		files = append(files, file)
	}
	return files, nil
}

// walkFileTree flattens a BEP 52 file tree in key order. A node whose only key
// is "" is a file; its properties live under that key.
func (info *torrentInfo) walkFileTree(node map[string][]byte, keys, path []string, depth int) error {
	if depth > bencodeMaxDepth {
		return fmt.Errorf("file tree nesting too deep")
	}
	var length int64
	if raw, ok := node[""]; ok {
		properties, _, err := bencodeDict(raw)
		if err != nil {
			return err
		}
		if err := decodeOptional(properties, "length", bencodeInt, &length); err != nil {
			return err
		}
		if info.pieceLength > 0 {
			info.treePieceCount += int((length + info.pieceLength - 1) / info.pieceLength)
		}
	}
	children := slices.DeleteFunc(slices.Clone(keys), func(key string) bool { return key == "" })
	if len(children) == 0 {
		info.files = append(info.files, torrentFileInfo{length: length, path: slices.Clone(path)})
		return nil
	}
	slices.Sort(children)
	for _, key := range children {
		child, childKeys, err := bencodeDict(node[key])
		if err != nil {
			return err
		}
		if err := info.walkFileTree(child, childKeys, append(path, key), depth+1); err != nil {
			return err
		}
	}
	return nil
}

func decodeOptional[T any](values map[string][]byte, key string, decode func([]byte) (T, error), target *T) error {
	raw, ok := values[key]
	if !ok {
		return nil
	}
	value, err := decode(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	*target = value
	return nil
}

func (info *torrentInfo) bestName() string {
	if info.nameUTF8 != "" {
		return info.nameUTF8
	}
	return info.name
}

func (info *torrentInfo) hasV2() bool { return info.metaVersion == 2 }

// hasV1 follows the BEP 52 upgrade path: hybrid torrents carry both versions.
func (info *torrentInfo) hasV1() bool {
	return info.metaVersion == 0 || info.metaVersion == 1 || info.hasFileList || info.length != 0 || info.piecesBytes != 0
}

func (info *torrentInfo) isDir() bool {
	if info.hasV2() {
		return info.treeIsDir
	}
	return info.v1Files != 0
}

func (info *torrentInfo) numPieces() int {
	if info.hasV2() {
		return info.treePieceCount
	}
	return info.piecesBytes / sha1.Size
}

// announceTiers returns announce-list, falling back to the single announce URL.
func (meta *torrentMeta) announceTiers() [][]string {
	for _, tier := range meta.announceList {
		for _, url := range tier {
			if url != "" || meta.announce == "" {
				return meta.announceList
			}
		}
	}
	if meta.announce != "" {
		return [][]string{{meta.announce}}
	}
	return nil
}

func torrentTrackers(tiers [][]string, maximum int) []string {
	seen := make(map[string]struct{})
	trackers := make([]string, 0, min(len(tiers), maximum))
	for _, tier := range tiers {
		for _, tracker := range tier {
			tracker = safeText(strings.TrimSpace(tracker))
			if tracker == "" {
				continue
			}
			if _, ok := seen[tracker]; ok {
				continue
			}
			seen[tracker] = struct{}{}
			trackers = append(trackers, tracker)
			if len(trackers) >= maximum {
				return trackers
			}
		}
	}
	return trackers
}

func torrentFields(value Torrent, fileCount int) []Field {
	fields := make([]Field, 0, 12)
	appendField := func(name, fieldValue string) {
		if fieldValue != "" {
			fields = append(fields, Field{Name: name, Value: fieldValue})
		}
	}
	appendField("Name", value.Name)
	appendField("Size", formatByteSize(value.TotalSize))
	appendField("Files", fmt.Sprint(fileCount))
	if value.PieceLength > 0 {
		appendField("Pieces", fmt.Sprintf("%d x %s", value.PieceCount, formatByteSize(value.PieceLength)))
	}
	appendField("Info hash v1", value.InfoHashV1)
	appendField("Info hash v2", value.InfoHashV2)
	appendField("Created by", value.CreatedBy)
	appendField("Comment", value.Comment)
	if value.Private {
		appendField("Private", "yes")
	}
	appendField("Trackers", fmt.Sprint(len(value.Trackers)))
	appendField("Web seeds", fmt.Sprint(len(value.WebSeeds)))
	return fields
}

func formatByteSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	value := float64(size)
	for _, unit := range units {
		value /= 1024
		if value < 1024 || unit == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%d B", size)
}
