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

/*
renderVideo shows a frame from 10% into the video (at most 30s in) plus its
metadata. Each external tool costs a few hundred milliseconds just to start, so:

  - the result is cached on disk by file identity and pane size, so a revisit
    runs no tools;
  - the frame for long videos is grabbed while ffprobe is still running. Any
    video of five minutes or more uses the 30s cap, so that frame is started
    speculatively and kept when the duration confirms it.
*/
func (s *Service) renderVideo(ctx context.Context, path string, cellWidth int) (*VideoPreview, error) {
	if s.tools.FFProbe == "" || s.tools.FFmpeg == "" {
		return nil, fmt.Errorf("video preview: %w", ToolUnavailable("ffprobe/ffmpeg"))
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat video: %w", err)
	}
	targetSize := s.imageTargetSize(cellWidth)
	cacheKey := s.imageCache.Key(path, info.Size(), info.ModTime(), targetSize)
	if cached, ok := s.cachedVideo(cacheKey); ok {
		return cached, nil
	}

	speculative, cancelSpeculative := context.WithCancel(ctx)
	defer cancelSpeculative()
	type frameResult struct {
		image Image
		err   error
	}
	guess := make(chan frameResult, 1)
	go func() {
		image, err := s.renderVideoFrame(speculative, path, videoFrameCap, targetSize)
		guess <- frameResult{image, err}
	}()

	video, err := s.probeVideo(ctx, path)
	if err != nil {
		return nil, err
	}
	var image Image
	if timestamp := videoFrameTime(video.Duration); timestamp == videoFrameCap {
		result := <-guess
		image, err = result.image, result.err
	} else {
		cancelSpeculative()
		image, err = s.renderVideoFrame(ctx, path, timestamp, targetSize)
	}
	if err != nil {
		return nil, err
	}
	if encoded, err := json.Marshal(video); err == nil {
		s.imageCache.Put(cacheKey, image.Data, image.Width, image.Height, "video", video.Width, video.Height, info.Size(), encoded)
	}
	return &VideoPreview{Video: video, Frame: image, Metadata: videoFields(video)}, nil
}

func (s *Service) cachedVideo(cacheKey string) (*VideoPreview, bool) {
	cached, ok := s.imageCache.Get(cacheKey)
	if !ok {
		return nil, false
	}
	var video Video
	if err := json.Unmarshal(cached.Metadata, &video); err != nil {
		return nil, false
	}
	frame := Image{MediaType: "image/png", Data: cached.Data, Width: cached.Width, Height: cached.Height}
	return &VideoPreview{Video: video, Frame: frame, Metadata: videoFields(video)}, true
}

func (s *Service) probeVideo(ctx context.Context, path string) (Video, error) {
	output, err := runCommand(ctx, 1<<20, s.tools.FFProbe,
		"-v", "error", "-show_format", "-show_streams", "-of", "json", path,
	)
	if err != nil {
		return Video{}, fmt.Errorf("inspect video: %w", err)
	}
	var probe ffprobeResult
	if err := json.Unmarshal(output, &probe); err != nil {
		return Video{}, fmt.Errorf("decode video metadata: %w", err)
	}
	videoStream, audioStreams, ok := selectVideoStreams(probe.Streams)
	if !ok {
		return Video{}, fmt.Errorf("inspect video: no video stream")
	}
	return videoMetadata(probe.Format, videoStream, audioStreams), nil
}

// videoFrameCap is the latest point a preview frame is taken from, in seconds.
const videoFrameCap = 30.0

// videoFrameTime picks the preview frame: 10% in, skipping intros and fades,
// but no later than videoFrameCap.
func videoFrameTime(duration float64) float64 {
	return min(max(duration*0.1, 0), videoFrameCap)
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

func (s *Service) renderVideoFrame(ctx context.Context, path string, timestamp float64, targetSize int) (Image, error) {
	seek := []string{"-ss", strconv.FormatFloat(timestamp, 'f', 3, 64)}
	image, err := s.ffmpegStill(ctx, path, targetSize, true, seek...)
	if err != nil && ctx.Err() == nil {
		// Some streams have no keyframe near the seek point; decode exactly.
		image, err = s.ffmpegStill(ctx, path, targetSize, false, seek...)
	}
	if err != nil {
		return Image{}, fmt.Errorf("render video frame: %w", err)
	}
	return image, nil
}

/*
ffmpegStill decodes one frame of path (after inputArguments, such as a seek)
into a PNG no larger than targetSize on either side. It is tuned for a single
frame: one decoder thread avoids frame-threading delay, and the PNG is piped
back with light compression rather than written to a temporary file. With
keyframe set, a seek lands on the nearest keyframe and only keyframes are
decoded, instead of every frame from the keyframe up to the exact time.
*/
func (s *Service) ffmpegStill(ctx context.Context, path string, targetSize int, keyframe bool, inputArguments ...string) (Image, error) {
	arguments := []string{"-v", "error", "-nostdin", "-threads", "1"}
	if keyframe {
		arguments = append(arguments, "-noaccurate_seek")
	}
	arguments = append(arguments, inputArguments...)
	if keyframe {
		arguments = append(arguments, "-skip_frame", "nokey")
	}
	arguments = append(arguments,
		"-i", path, "-map", "0:v:0", "-frames:v", "1", "-an",
		"-vf", fmt.Sprintf("scale=min(%d\\,iw):min(%d\\,ih):force_original_aspect_ratio=decrease", targetSize, targetSize),
		"-f", "image2pipe", "-c:v", "png", "-compression_level", "1", "-",
	)
	output, err := runCommand(ctx, s.maxImageBytes, s.tools.FFmpeg, arguments...)
	if err != nil {
		return Image{}, err
	}
	config, err := png.DecodeConfig(bytes.NewReader(output))
	if err != nil {
		return Image{}, fmt.Errorf("no frame decoded: %w", err)
	}
	if config.Width > s.maxImageDimension || config.Height > s.maxImageDimension {
		return Image{}, ErrOutputTooLarge
	}
	return Image{MediaType: "image/png", Data: output, Width: config.Width, Height: config.Height}, nil
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
