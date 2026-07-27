//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
)

// deleteFileOrDir removes file or directory (recursive)
func deleteFileOrDir(path string) error {
	if path == "" {
		return fmt.Errorf("path required")
	}
	return os.RemoveAll(path)
}

// readFileContent returns raw bytes of a file (for "read" task)
func readFileContent(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("path required")
	}
	return os.ReadFile(path)
}

// downloadFileChunk reads a chunk from file
func downloadFileChunk(path string, offset, size int64) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("path required")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer f.Close()
	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			return nil, err
		}
	}
	if size == 0 {
		size = 1024 * 1024 // default 1MB
	}
	buf := make([]byte, size)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read chunk failed: %w", err)
	}
	return buf[:n], nil
}

// uploadFileChunk writes base64 chunk at offset
func uploadFileChunk(path string, offset int64, b64Content string) error {
	data, err := base64.StdEncoding.DecodeString(b64Content)
	if err != nil {
		return fmt.Errorf("base64 decode failed: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open file for write failed: %w", err)
	}
	defer f.Close()
	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			return err
		}
	}
	_, err = f.Write(data)
	return err
}

// downloadFromURL downloads a file from HTTP URL to dest path on disk
func downloadFromURL(urlStr, destPath string) error {
	if destPath == "" {
		return fmt.Errorf("destination path required")
	}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("http status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}
