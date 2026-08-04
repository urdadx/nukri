package preview

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/anacrolix/torrent/metainfo"
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
	meta, err := metainfo.Load(bytes.NewReader(bytes.TrimSpace(data)))
	if err != nil {
		return nil, fmt.Errorf("parse torrent metainfo: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := meta.UnmarshalInfo()
	if err != nil {
		return nil, fmt.Errorf("parse torrent info: %w", err)
	}
	if info.PieceLength < 0 || info.Length < 0 {
		return nil, fmt.Errorf("parse torrent info: negative length")
	}

	value := Torrent{
		Name: safeText(info.BestName()), Comment: safeText(meta.Comment),
		CreatedBy: safeText(meta.CreatedBy), CreationDate: meta.CreationDate,
		PieceLength: info.PieceLength, PieceCount: info.NumPieces(),
	}
	if info.Private != nil {
		value.Private = *info.Private
	}
	if info.HasV1() {
		value.InfoHashV1 = meta.HashInfoBytes().HexString()
	}
	if info.HasV2() {
		magnet, err := meta.MagnetV2()
		if err == nil && magnet.V2InfoHash.Ok {
			value.InfoHashV2 = magnet.V2InfoHash.Value.HexString()
		}
	}
	value.Trackers = torrentTrackers(meta.UpvertedAnnounceList(), s.maxTorrentTrackers)
	for _, seed := range meta.UrlList {
		value.WebSeeds = append(value.WebSeeds, safeText(seed))
	}

	allFiles := info.UpvertedFiles()
	files := make([]TorrentFile, 0, min(len(allFiles), s.maxTorrentFiles))
	for index := range allFiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fileInfo := &allFiles[index]
		if fileInfo.Length < 0 || value.TotalSize > math.MaxInt64-fileInfo.Length {
			return nil, fmt.Errorf("parse torrent info: invalid total file size")
		}
		value.TotalSize += fileInfo.Length
		if len(files) >= s.maxTorrentFiles {
			continue
		}
		filePath := safeText(fileInfo.DisplayPath(&info))
		if filePath == "" {
			filePath = value.Name
		}
		files = append(files, TorrentFile{Path: filePath, Size: fileInfo.Length})
	}
	if len(allFiles) == 0 && info.Length != 0 {
		value.TotalSize = info.Length
		files = append(files, TorrentFile{Path: value.Name, Size: info.Length})
	}
	result := &TorrentPreview{Torrent: value, Files: files, Truncated: len(allFiles) > len(files)}
	result.Metadata = torrentFields(value, max(len(allFiles), len(files)))
	return result, nil
}

func torrentTrackers(tiers metainfo.AnnounceList, maximum int) []string {
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
