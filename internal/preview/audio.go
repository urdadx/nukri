package preview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
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

/*
renderAudio shows the cover art (or a waveform when there is none) plus the
audio metadata. Decoding audio for the waveform dominates the cost, so:

  - the result is cached on disk by file identity and pane size, so a revisit
    runs no tools;
  - ffprobe, cover extraction and the waveform run at the same time. Whether a
    cover exists is only known after probing, so both visuals start at once and
    the unneeded one is cancelled.
*/
func (s *Service) renderAudio(ctx context.Context, path string, cellWidth int) (*AudioPreview, error) {
	if s.tools.FFProbe == "" || s.tools.FFmpeg == "" {
		return nil, fmt.Errorf("audio preview: %w", ToolUnavailable("ffprobe/ffmpeg"))
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat audio: %w", err)
	}
	targetSize := s.imageTargetSize(cellWidth)
	cacheKey := s.imageCache.Key(path, info.Size(), info.ModTime(), targetSize)
	if cached, ok := s.cachedAudio(cacheKey); ok {
		return cached, nil
	}

	type visualResult struct {
		image Image
		err   error
	}
	coverContext, cancelCover := context.WithCancel(ctx)
	defer cancelCover()
	waveContext, cancelWave := context.WithCancel(ctx)
	defer cancelWave()
	cover := make(chan visualResult, 1)
	wave := make(chan visualResult, 1)
	go func() {
		image, err := s.ffmpegStill(coverContext, path, targetSize, false)
		cover <- visualResult{image, err}
	}()
	go func() {
		image, err := s.renderWaveform(waveContext, path, targetSize)
		wave <- visualResult{image, err}
	}()

	audio, hasCover, err := s.probeAudio(ctx, path)
	if err != nil {
		return nil, err
	}
	var visual visualResult
	audio.Visual = Waveform
	if hasCover {
		if visual = <-cover; visual.err == nil {
			audio.Visual = Cover
			cancelWave()
		}
	} else {
		cancelCover()
	}
	if audio.Visual == Waveform {
		if visual = <-wave; visual.err != nil {
			return nil, fmt.Errorf("render audio waveform: %w", visual.err)
		}
	}
	if encoded, err := json.Marshal(audio); err == nil {
		s.imageCache.Put(cacheKey, visual.image.Data, visual.image.Width, visual.image.Height, "audio", 0, 0, info.Size(), encoded)
	}
	return &AudioPreview{Audio: audio, Visual: visual.image, Metadata: audioFields(audio)}, nil
}

func (s *Service) cachedAudio(cacheKey string) (*AudioPreview, bool) {
	cached, ok := s.imageCache.Get(cacheKey)
	if !ok {
		return nil, false
	}
	var audio Audio
	if err := json.Unmarshal(cached.Metadata, &audio); err != nil {
		return nil, false
	}
	visual := Image{MediaType: "image/png", Data: cached.Data, Width: cached.Width, Height: cached.Height}
	return &AudioPreview{Audio: audio, Visual: visual, Metadata: audioFields(audio)}, true
}

// probeAudio reads the metadata and whether the file embeds cover art.
func (s *Service) probeAudio(ctx context.Context, path string) (Audio, bool, error) {
	output, err := runCommand(ctx, 1<<20, s.tools.FFProbe,
		"-v", "error", "-show_format", "-show_streams", "-of", "json", path,
	)
	if err != nil {
		return Audio{}, false, fmt.Errorf("inspect audio: %w", err)
	}
	var probe ffprobeResult
	if err := json.Unmarshal(output, &probe); err != nil {
		return Audio{}, false, fmt.Errorf("decode audio metadata: %w", err)
	}
	audioStream, hasCover, ok := selectAudioStreams(probe.Streams)
	if !ok {
		return Audio{}, false, fmt.Errorf("inspect audio: no audio stream")
	}
	return audioMetadata(probe.Format, audioStream), hasCover, nil
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

// waveformSeconds caps how much audio the waveform decodes. Decoding is the
// slow part of an audio preview and grows with length, so the waveform shows
// the opening minute rather than the whole track.
const waveformSeconds = 60

// renderWaveform draws the opening waveformSeconds of path at pane size: a
// strip as wide as targetSize and a quarter as tall.
func (s *Service) renderWaveform(ctx context.Context, path string, targetSize int) (Image, error) {
	width := max(targetSize, 64)
	height := max(width/4, 16)
	output, err := runCommand(ctx, s.maxImageBytes, s.tools.FFmpeg,
		"-v", "error", "-nostdin", "-t", strconv.Itoa(waveformSeconds), "-i", path,
		"-filter_complex", fmt.Sprintf("showwavespic=s=%dx%d:colors=0x1CB0F6", width, height),
		"-frames:v", "1", "-f", "image2pipe", "-c:v", "png", "-compression_level", "1", "-",
	)
	if err != nil {
		return Image{}, err
	}
	config, err := png.DecodeConfig(bytes.NewReader(output))
	if err != nil {
		return Image{}, fmt.Errorf("no waveform drawn: %w", err)
	}
	return Image{MediaType: "image/png", Data: output, Width: config.Width, Height: config.Height}, nil
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
