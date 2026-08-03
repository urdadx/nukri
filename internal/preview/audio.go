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

type ffprobeResult struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecName     string            `json:"codec_name"`
	CodecLongName string            `json:"codec_long_name"`
	CodecType     string            `json:"codec_type"`
	SampleRate    string            `json:"sample_rate"`
	Channels      int               `json:"channels"`
	ChannelLayout string            `json:"channel_layout"`
	BitsPerSample int               `json:"bits_per_sample"`
	BitRate       string            `json:"bit_rate"`
	Duration      string            `json:"duration"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	PixelFormat   string            `json:"pix_fmt"`
	AverageRate   string            `json:"avg_frame_rate"`
	Tags          map[string]string `json:"tags"`
	Disposition   struct {
		AttachedPicture int `json:"attached_pic"`
	} `json:"disposition"`
}

type ffprobeFormat struct {
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	Duration       string            `json:"duration"`
	BitRate        string            `json:"bit_rate"`
	Tags           map[string]string `json:"tags"`
}

func (s *Service) renderAudio(ctx context.Context, path string) (*AudioPreview, error) {
	if s.tools.FFProbe == "" || s.tools.FFmpeg == "" {
		return nil, fmt.Errorf("audio preview: %w", ErrToolUnavailable)
	}
	output, err := runCommand(ctx, 1<<20, s.tools.FFProbe,
		"-v", "error", "-show_format", "-show_streams", "-of", "json", path,
	)
	if err != nil {
		return nil, fmt.Errorf("inspect audio: %w", err)
	}
	var probe ffprobeResult
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("decode audio metadata: %w", err)
	}
	audioStream, hasCover, ok := selectAudioStreams(probe.Streams)
	if !ok {
		return nil, fmt.Errorf("inspect audio: no audio stream")
	}
	audio := audioMetadata(probe.Format, audioStream)
	image, visual, err := s.renderAudioVisual(ctx, path, hasCover)
	if err != nil {
		return nil, err
	}
	audio.Visual = visual
	fields := audioFields(audio)
	return &AudioPreview{Audio: audio, Visual: image, Metadata: fields}, nil
}

func selectAudioStreams(streams []ffprobeStream) (ffprobeStream, bool, bool) {
	var audio ffprobeStream
	found := false
	hasCover := false
	for _, stream := range streams {
		if stream.CodecType == "audio" && !found {
			audio = stream
			found = true
		}
		if stream.CodecType == "video" && stream.Disposition.AttachedPicture != 0 {
			hasCover = true
		}
	}
	return audio, hasCover, found
}

func audioMetadata(format ffprobeFormat, stream ffprobeStream) Audio {
	tags := make(map[string]string, len(format.Tags)+len(stream.Tags))
	for key, value := range format.Tags {
		tags[strings.ToLower(key)] = safeText(strings.TrimSpace(value))
	}
	for key, value := range stream.Tags {
		tags[strings.ToLower(key)] = safeText(strings.TrimSpace(value))
	}
	duration := parseFloat(format.Duration)
	if duration == 0 {
		duration = parseFloat(stream.Duration)
	}
	bitRate := parseInt(format.BitRate)
	if bitRate == 0 {
		bitRate = parseInt(stream.BitRate)
	}
	codec := stream.CodecLongName
	if codec == "" {
		codec = stream.CodecName
	}
	container := format.FormatLongName
	if container == "" {
		container = format.FormatName
	}
	return Audio{
		Title: tags["title"], Artist: tags["artist"], Album: tags["album"],
		Date: tags["date"], Genre: tags["genre"], Codec: safeText(codec),
		Container: safeText(container), ChannelLayout: safeText(stream.ChannelLayout),
		Duration: duration, BitRate: bitRate, SampleRate: int(parseInt(stream.SampleRate)),
		Channels: stream.Channels, BitDepth: stream.BitsPerSample,
	}
}

func (s *Service) renderAudioVisual(ctx context.Context, path string, hasCover bool) (Image, PreviewVisualKind, error) {
	directory, err := os.MkdirTemp("", "nukri-audio-*")
	if err != nil {
		return Image{}, Waveform, fmt.Errorf("create audio preview directory: %w", err)
	}
	defer os.RemoveAll(directory)
	if hasCover {
		cover := filepath.Join(directory, "cover.png")
		_, commandErr := runCommand(ctx, 64<<10, s.tools.FFmpeg,
			"-v", "error", "-nostdin", "-y", "-i", path,
			"-map", "0:v:0", "-frames:v", "1",
			"-vf", "scale=min(1600\\,iw):min(1600\\,ih):force_original_aspect_ratio=decrease", cover,
		)
		if commandErr == nil {
			if image, readErr := readPNG(cover, s.maxImageBytes, s.maxImageDimension); readErr == nil {
				return image, Cover, nil
			}
		}
	}
	waveform := filepath.Join(directory, "waveform.png")
	_, err = runCommand(ctx, 64<<10, s.tools.FFmpeg,
		"-v", "error", "-nostdin", "-y", "-t", "300", "-i", path,
		"-filter_complex", "showwavespic=s=1200x300:colors=0x1CB0F6", "-frames:v", "1", waveform,
	)
	if err != nil {
		return Image{}, Waveform, fmt.Errorf("render audio waveform: %w", err)
	}
	image, err := readPNG(waveform, s.maxImageBytes, s.maxImageDimension)
	if err != nil {
		return Image{}, Waveform, fmt.Errorf("read audio waveform: %w", err)
	}
	return image, Waveform, nil
}

func audioFields(audio Audio) []Field {
	fields := make([]Field, 0, 12)
	appendField := func(name, value string) {
		if value != "" {
			fields = append(fields, Field{Name: name, Value: value})
		}
	}
	appendField("Title", audio.Title)
	appendField("Artist", audio.Artist)
	appendField("Album", audio.Album)
	appendField("Date", audio.Date)
	appendField("Genre", audio.Genre)
	appendField("Duration", formatDuration(audio.Duration))
	appendField("Codec", audio.Codec)
	appendField("Container", audio.Container)
	if audio.SampleRate != 0 {
		appendField("Sample rate", fmt.Sprintf("%d Hz", audio.SampleRate))
	}
	if audio.BitDepth != 0 {
		appendField("Bit depth", fmt.Sprintf("%d-bit", audio.BitDepth))
	}
	if audio.Channels != 0 {
		value := fmt.Sprint(audio.Channels)
		if audio.ChannelLayout != "" {
			value += " (" + audio.ChannelLayout + ")"
		}
		appendField("Channels", value)
	}
	if audio.BitRate != 0 {
		appendField("Bit rate", fmt.Sprintf("%d kb/s", audio.BitRate/1000))
	}
	return fields
}

func parseFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func parseInt(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func formatDuration(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	total := int64(seconds + 0.5)
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}
