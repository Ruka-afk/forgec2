//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writeThumbTestImage(t *testing.T, dir, name string, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatalf("encode image: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close image: %v", err)
	}
	return p
}

func decodeThumbResult(t *testing.T, raw string) fileThumbResult {
	t.Helper()
	var result fileThumbResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode thumb result: %v; raw=%s", err, raw)
	}
	return result
}

func TestFileThumbGeneratesBoundedJPEG(t *testing.T) {
	dir := t.TempDir()
	src := writeThumbTestImage(t, dir, "large.png", 900, 600)
	options, _ := json.Marshal(fileThumbOptions{
		Paths:    []string{src},
		MaxEdge:  64,
		MaxBytes: 64 * 1024,
	})
	res := executeTask(Task{Type: "file_thumb", Data: string(options)})
	if res.Error != "" {
		t.Fatalf("file_thumb failed: %s", res.Error)
	}
	result := decodeThumbResult(t, res.Output)
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if !item.OK || item.Error != "" {
		t.Fatalf("item = %#v", item)
	}
	if item.MIME != "image/jpeg" || item.Width <= 0 || item.Height <= 0 {
		t.Fatalf("item metadata = %#v", item)
	}
	if item.Width > 64 || item.Height > 64 {
		t.Fatalf("thumbnail bounds = %dx%d, want <= 64", item.Width, item.Height)
	}
	data, err := base64.StdEncoding.DecodeString(item.Data)
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	if !bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		t.Fatalf("thumbnail is not JPEG")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("jpeg config: %v", err)
	}
	if cfg.Width != item.Width || cfg.Height != item.Height {
		t.Fatalf("decoded bounds = %dx%d, metadata = %dx%d", cfg.Width, cfg.Height, item.Width, item.Height)
	}
}

func TestFileThumbReportsPerPathErrors(t *testing.T) {
	dir := t.TempDir()
	textPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(textPath, []byte("not media"), 0600); err != nil {
		t.Fatalf("write text: %v", err)
	}
	options, _ := json.Marshal(fileThumbOptions{
		Paths: []string{filepath.Join(dir, "missing.jpg"), textPath},
	})
	res := executeTask(Task{Type: "file_thumb", Data: string(options)})
	if res.Error != "" {
		t.Fatalf("file_thumb failed: %s", res.Error)
	}
	result := decodeThumbResult(t, res.Output)
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Items))
	}
	for _, item := range result.Items {
		if item.OK || item.Error == "" || item.Data != "" {
			t.Fatalf("expected per-path error, got %#v", item)
		}
	}
}

func TestFileThumbAcceptsLegacyTaskPath(t *testing.T) {
	dir := t.TempDir()
	src := writeThumbTestImage(t, dir, "small.png", 32, 32)
	res := executeTask(Task{Type: "file_thumb", Path: src})
	if res.Error != "" {
		t.Fatalf("file_thumb failed: %s", res.Error)
	}
	result := decodeThumbResult(t, res.Output)
	if len(result.Items) != 1 || !result.Items[0].OK {
		t.Fatalf("items = %#v", result.Items)
	}
}

func TestFileThumbRejectsTooManyPaths(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, fileThumbDefaultMaxItems+1)
	for i := range paths {
		paths[i] = filepath.Join(dir, fmt.Sprintf("x%d.jpg", i))
	}
	options, _ := json.Marshal(fileThumbOptions{Paths: paths})
	res := executeTask(Task{Type: "file_thumb", Data: string(options)})
	if res.Error == "" {
		t.Fatal("expected too-many-paths error")
	}
}
