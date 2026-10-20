//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// file_thumb is the bounded preview half of file intelligence. It reads at
// most 12 operator-selected image/video paths, downscales a single frame to a
// small JPEG, and returns base64 only for the server-side cache writer. The
// task result itself never becomes the media browser's image source, so a
// thumbnail cannot be replayed as a task-result blob after the cache expires.
const (
	fileThumbDefaultMaxItems = 12
	fileThumbDefaultMaxEdge  = 320
	fileThumbHardMaxEdge     = 512
	fileThumbDefaultMaxBytes = 256 * 1024
	fileThumbHardMaxBytes    = 512 * 1024
	fileThumbSourceMaxBytes  = 64 * 1024 * 1024
	fileThumbDecodeMaxPixels = 40 * 1000 * 1000
	fileThumbResultBudget    = 900 * 1024
	fileThumbFFmpegTimeout   = 12 * time.Second
)

var fileThumbImageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
}

var fileThumbVideoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".wmv": true,
	".webm": true, ".m4v": true, ".flv": true, ".mpg": true, ".mpeg": true,
	".ts": true,
}

type fileThumbOptions struct {
	Paths    []string `json:"paths"`
	MaxEdge  int      `json:"max_edge"`
	MaxBytes int      `json:"max_bytes"`
}

type fileThumbItem struct {
	Path       string `json:"path"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
	MIME       string `json:"mime,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	SourceSize int64  `json:"source_size,omitempty"`
	Modified   string `json:"modified,omitempty"`
	Data       string `json:"data,omitempty"`
}

type fileThumbResult struct {
	Items []fileThumbItem `json:"items"`
}

func handleFileThumb(task Task, res *TaskResult) {
	opts, err := parseFileThumbOptions(task)
	if err != nil {
		res.Error = err.Error()
		return
	}
	ctx := currentExecCtx()
	if ctx == nil {
		ctx = context.Background()
	}

	result := fileThumbResult{Items: make([]fileThumbItem, 0, len(opts.Paths))}
	used := 0
	for i, path := range opts.Paths {
		if used >= fileThumbResultBudget {
			for _, remaining := range opts.Paths[i:] {
				result.Items = append(result.Items, fileThumbItem{
					Path:  remaining,
					Error: "thumbnail result budget exceeded",
				})
			}
			break
		}
		item := makeFileThumb(ctx, path, opts)
		if item.Data != "" {
			encodedLen := len(item.Data)
			if used+encodedLen > fileThumbResultBudget {
				item = fileThumbItem{Path: path, Error: "thumbnail result budget exceeded"}
			} else {
				used += encodedLen
			}
		}
		result.Items = append(result.Items, item)
	}
	out, err := json.Marshal(result)
	if err != nil {
		res.Error = "file_thumb: encode result: " + err.Error()
		return
	}
	res.Output = string(out)
	if len(opts.Paths) > 0 {
		res.Path = opts.Paths[0]
	}
}

func parseFileThumbOptions(task Task) (fileThumbOptions, error) {
	opts := fileThumbOptions{
		MaxEdge:  fileThumbDefaultMaxEdge,
		MaxBytes: fileThumbDefaultMaxBytes,
	}
	if raw := strings.TrimSpace(task.Data); raw != "" {
		if err := json.Unmarshal([]byte(raw), &opts); err != nil {
			return opts, fmt.Errorf("file_thumb: invalid options: %w", err)
		}
	}
	if len(opts.Paths) == 0 {
		legacy := strings.TrimSpace(task.Path)
		if legacy == "" {
			legacy = strings.TrimSpace(task.Command)
		}
		if legacy != "" {
			opts.Paths = []string{legacy}
		}
	}
	if opts.MaxEdge <= 0 {
		opts.MaxEdge = fileThumbDefaultMaxEdge
	}
	if opts.MaxEdge < 64 {
		opts.MaxEdge = 64
	}
	if opts.MaxEdge > fileThumbHardMaxEdge {
		opts.MaxEdge = fileThumbHardMaxEdge
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = fileThumbDefaultMaxBytes
	}
	if opts.MaxBytes < 32*1024 {
		opts.MaxBytes = 32 * 1024
	}
	if opts.MaxBytes > fileThumbHardMaxBytes {
		opts.MaxBytes = fileThumbHardMaxBytes
	}

	seen := map[string]bool{}
	paths := make([]string, 0, len(opts.Paths))
	for _, path := range opts.Paths {
		path = strings.TrimSpace(path)
		if path == "" || len(path) > 4096 || strings.ContainsRune(path, '\x00') {
			continue
		}
		key := strings.ToLower(path)
		if seen[key] {
			continue
		}
		seen[key] = true
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return opts, fmt.Errorf("file_thumb: at least one path required")
	}
	if len(paths) > fileThumbDefaultMaxItems {
		return opts, fmt.Errorf("file_thumb: too many paths (max %d)", fileThumbDefaultMaxItems)
	}
	opts.Paths = paths
	return opts, nil
}

func makeFileThumb(ctx context.Context, path string, opts fileThumbOptions) fileThumbItem {
	item := fileThumbItem{Path: path}
	info, err := os.Stat(path)
	if err != nil {
		item.Error = "stat: " + err.Error()
		return item
	}
	if !info.Mode().IsRegular() {
		item.Error = "not a regular file"
		return item
	}
	item.SourceSize = info.Size()
	item.Modified = info.ModTime().UTC().Format(time.RFC3339)

	ext := strings.ToLower(filepath.Ext(path))
	var data []byte
	var width, height int
	switch {
	case fileThumbImageExts[ext]:
		if info.Size() > fileThumbSourceMaxBytes {
			item.Error = fmt.Sprintf("image exceeds source cap (%d bytes)", fileThumbSourceMaxBytes)
			return item
		}
		data, width, height, err = makeImageThumb(path, opts)
	case fileThumbVideoExts[ext]:
		data, width, height, err = makeVideoThumb(ctx, path, opts)
	default:
		err = fmt.Errorf("unsupported media type %q", ext)
	}
	if err != nil {
		item.Error = err.Error()
		return item
	}
	if len(data) == 0 {
		item.Error = "empty thumbnail"
		return item
	}
	item.OK = true
	item.MIME = "image/jpeg"
	item.Width = width
	item.Height = height
	item.Data = base64.StdEncoding.EncodeToString(data)
	return item
}

func makeImageThumb(path string, opts fileThumbOptions) ([]byte, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, 0, 0, fmt.Errorf("invalid image dimensions")
	}
	if int64(cfg.Width)*int64(cfg.Height) > fileThumbDecodeMaxPixels {
		return nil, 0, 0, fmt.Errorf("image dimensions exceed decode cap")
	}
	switch format {
	case "jpeg", "png", "gif":
	default:
		return nil, 0, 0, fmt.Errorf("unsupported image format %q", format)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, 0, 0, fmt.Errorf("rewind: %w", err)
	}
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode: %w", err)
	}
	dst := downscaleImage(src, opts.MaxEdge)
	encoded, err := encodeJPEGWithin(dst, opts.MaxBytes)
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := dst.Bounds()
	return encoded, bounds.Dx(), bounds.Dy(), nil
}

func makeVideoThumb(ctx context.Context, path string, opts fileThumbOptions) ([]byte, int, int, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, 0, 0, fmt.Errorf("ffmpeg not available on target")
	}
	filter := fmt.Sprintf("scale='min(%d,iw)':'min(%d,ih)':force_original_aspect_ratio=decrease",
		opts.MaxEdge, opts.MaxEdge)
	for _, seek := range []string{"1", "0"} {
		data, err := runFFmpegThumb(ctx, ffmpeg, filter, path, seek, opts.MaxBytes)
		if err == nil && len(data) > 0 {
			return data, 0, 0, nil
		}
		if ctx.Err() != nil {
			return nil, 0, 0, ctx.Err()
		}
	}
	return nil, 0, 0, fmt.Errorf("ffmpeg could not extract a frame")
}

func runFFmpegThumb(ctx context.Context, ffmpeg, filter, path, seek string, maxBytes int) ([]byte, error) {
	runCtx, cancel := context.WithTimeout(ctx, fileThumbFFmpegTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, ffmpeg,
		"-v", "error",
		"-nostdin",
		"-ss", seek,
		"-i", path,
		"-frames:v", "1",
		"-vf", filter,
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	)
	applyHideWindow(cmd)
	var out bytes.Buffer
	lw := &limitWriter{w: &out, limit: maxBytes + 1}
	cmd.Stdout = lw
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if lw.truncated || out.Len() > maxBytes {
		return nil, fmt.Errorf("ffmpeg thumbnail exceeds size cap")
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("ffmpeg: %s", truncateThumbText(msg, 200))
		}
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}
	data := out.Bytes()
	if !bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return nil, fmt.Errorf("ffmpeg returned a non-JPEG frame")
	}
	return data, nil
}

func downscaleImage(src image.Image, maxEdge int) image.Image {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return src
	}
	if width <= maxEdge && height <= maxEdge {
		return src
	}
	scale := float64(maxEdge) / float64(width)
	if height > width {
		scale = float64(maxEdge) / float64(height)
	}
	dstW := int(float64(width) * scale)
	dstH := int(float64(height) * scale)
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		sy := bounds.Min.Y + (y*height)/dstH
		for x := 0; x < dstW; x++ {
			sx := bounds.Min.X + (x*width)/dstW
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func truncateThumbText(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func encodeJPEGWithin(img image.Image, maxBytes int) ([]byte, error) {
	for _, quality := range []int{82, 72, 62, 52, 42, 32, 22} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, fmt.Errorf("encode jpeg: %w", err)
		}
		if buf.Len() <= maxBytes {
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("thumbnail exceeds max_bytes")
}
