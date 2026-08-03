package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Service) renderVideo(ctx context.Context, path string) (*VideoPreview, error) {
	if s.tools.FFProbe == "" || s.tools.FFmpeg == "" {
		return nil, fmt.Errorf("video preview: %w", ErrToolUnavailable)
	}
	output, err := runCommand(ctx, 1<<20, s.tools.FFProbe,
		"-v", "error", "-show_format", "-show_streams", "-of", "json", path,
	)
	if err != nil {
		return nil, fmt.Errorf("inspect video: %w", err)
	}
	var probe ffprobeResult
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("decode video metadata: %w", err)
	}
	videoStream, audioStreams, ok := selectVideoStreams(probe.Streams)
	if !ok {
		return nil, fmt.Errorf("inspect video: no video stream")
	}
	video := videoMetadata(probe.Format, videoStream, audioStreams)
	image, err := s.renderVideoFrame(ctx, path, video.Duration)
	if err != nil {
		return nil, err
	}
	fields := videoFields(video)
	return &VideoPreview{Video: video, Frame: image, Metadata: fields}, nil
}

func selectVideoStreams(streams []ffprobeStream) (ffprobeStream, []ffprobeStream, bool) {
	var video ffprobeStream
	found := false
	var audio []ffprobeStream
	for _, stream := range streams {
		if stream.CodecType == "video" && stream.Disposition.AttachedPicture == 0 && !found {
			video = stream
			found = true
		}
		if stream.CodecType == "audio" {
			audio = append(audio, stream)
		}
	}
	return video, audio, found
}

func videoMetadata(format ffprobeFormat, stream ffprobeStream, audioStreams []ffprobeStream) Video {
	duration := parseFloat(format.Duration)
	if duration == 0 {
		duration = parseFloat(stream.Duration)
	}
	codec := stream.CodecLongName
	if codec == "" {
		codec = stream.CodecName
	}
	container := format.FormatLongName
	if container == "" {
		container = format.FormatName
	}
	audioCodec := ""
	if len(audioStreams) != 0 {
		audioCodec = audioStreams[0].CodecLongName
		if audioCodec == "" {
			audioCodec = audioStreams[0].CodecName
		}
	}
	return Video{
		Title: tagValue(format.Tags, "title"), Codec: safeText(codec), Container: safeText(container),
		PixelFormat: safeText(stream.PixelFormat), Duration: duration, BitRate: parseInt(format.BitRate),
		Width: stream.Width, Height: stream.Height, FrameRate: parseFrameRate(stream.AverageRate),
		AudioCodec: safeText(audioCodec), AudioTracks: len(audioStreams),
	}
}

func (s *Service) renderVideoFrame(ctx context.Context, path string, duration float64) (Image, error) {
	directory, err := os.MkdirTemp("", "nukri-video-*")
	if err != nil {
		return Image{}, fmt.Errorf("create video preview directory: %w", err)
	}
	defer os.RemoveAll(directory)
	timestamp := duration * 0.1
	if timestamp > 30 {
		timestamp = 30
	}
	if timestamp < 0 {
		timestamp = 0
	}
	output := filepath.Join(directory, "frame.png")
	_, err = runCommand(ctx, 64<<10, s.tools.FFmpeg,
		"-v", "error", "-nostdin", "-y", "-ss", strconv.FormatFloat(timestamp, 'f', 3, 64),
		"-i", path, "-map", "0:v:0", "-frames:v", "1", "-an",
		"-vf", "scale=min(1600\\,iw):min(1600\\,ih):force_original_aspect_ratio=decrease", output,
	)
	if err != nil {
		return Image{}, fmt.Errorf("render video frame: %w", err)
	}
	image, err := readPNG(output, s.maxImageBytes, s.maxImageDimension)
	if err != nil {
		return Image{}, fmt.Errorf("read video frame: %w", err)
	}
	return image, nil
}

func videoFields(video Video) []Field {
	fields := make([]Field, 0, 10)
	appendField := func(name, value string) {
		if value != "" {
			fields = append(fields, Field{Name: name, Value: value})
		}
	}
	appendField("Title", video.Title)
	appendField("Duration", formatDuration(video.Duration))
	appendField("Codec", video.Codec)
	appendField("Container", video.Container)
	if video.Width != 0 && video.Height != 0 {
		appendField("Dimensions", fmt.Sprintf("%dx%d", video.Width, video.Height))
	}
	if video.FrameRate != 0 {
		appendField("Frame rate", fmt.Sprintf("%.3g fps", video.FrameRate))
	}
	appendField("Pixel format", video.PixelFormat)
	if video.BitRate != 0 {
		appendField("Bit rate", fmt.Sprintf("%d kb/s", video.BitRate/1000))
	}
	appendField("Audio codec", video.AudioCodec)
	if video.AudioTracks != 0 {
		appendField("Audio tracks", strconv.Itoa(video.AudioTracks))
	}
	return fields
}

func parseFrameRate(value string) float64 {
	numerator, denominator, ok := strings.Cut(value, "/")
	if !ok {
		return parseFloat(value)
	}
	bottom := parseFloat(denominator)
	if bottom == 0 {
		return 0
	}
	return parseFloat(numerator) / bottom
}

func tagValue(tags map[string]string, name string) string {
	for key, value := range tags {
		if strings.EqualFold(key, name) {
			return safeText(strings.TrimSpace(value))
		}
	}
	return ""
}
