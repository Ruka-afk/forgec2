//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/json"
	"net/url"
	"runtime"
	"strings"
	"testing"
)

func TestBuildAgentSignalURL(t *testing.T) {
	got, err := buildAgentSignalURL("https://c2.example:8443/base", "/ws/rd/agent", "sess 1", "tok+en", "agent-1")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.HasPrefix(got, "wss://c2.example:8443/ws/rd/agent?") {
		t.Fatalf("unexpected URL: %s", got)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Query().Get("session") != "sess 1" || parsed.Query().Get("token") != "tok+en" || parsed.Query().Get("agent") != "agent-1" {
		t.Fatalf("query lost values: %s", got)
	}

	cleartext, err := buildAgentSignalURL("http://127.0.0.1:8000", "", "s", "t", "")
	if err != nil {
		t.Fatalf("cleartext: %v", err)
	}
	if !strings.HasPrefix(cleartext, "ws://127.0.0.1:8000/ws/rd/agent?") {
		t.Fatalf("unexpected cleartext URL: %s", cleartext)
	}
	if _, err := buildAgentSignalURL("ftp://host", "", "s", "t", ""); err == nil {
		t.Fatal("unsupported scheme must fail")
	}
}

func TestBuildScreenFFmpegArgs(t *testing.T) {
	settings := webRTCSettings{Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 4500, Encoder: "libx264"}
	args, err := buildScreenFFmpegArgs(settings, 40000)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "rtp://127.0.0.1:40000") {
		t.Fatalf("RTP output missing: %s", joined)
	}
	if !strings.Contains(joined, "-c:v libx264") || !strings.Contains(joined, "-profile:v baseline") {
		t.Fatalf("baseline H.264 missing: %s", joined)
	}
	if !strings.Contains(joined, "-g 60") || !strings.Contains(joined, "-pkt_size 1200") {
		t.Fatalf("keyframe/RTP sizing missing: %s", joined)
	}
	if !strings.Contains(joined, "-vf scale=1920:1080") {
		t.Fatalf("scale filter missing: %s", joined)
	}
	if !strings.Contains(joined, "-level:v 4.0") {
		t.Fatalf("1080p level must be 4.0: %s", joined)
	}

	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(joined, "-f gdigrab") || !strings.Contains(joined, "-i desktop") {
			t.Fatalf("windows capture missing: %s", joined)
		}
	case "linux":
		if !strings.Contains(joined, "-f x11grab") {
			t.Fatalf("linux capture missing: %s", joined)
		}
	case "darwin":
		if !strings.Contains(joined, "-f avfoundation") {
			t.Fatalf("darwin capture missing: %s", joined)
		}
	}

	if _, err := buildScreenFFmpegArgs(settings, 0); err == nil {
		t.Fatal("invalid port must fail")
	}
}

func TestEncoderArgsDoNotPassX264OptionsToHardware(t *testing.T) {
	software := strings.Join(encoderFFmpegArgs("libx264", "3.1"), " ")
	if !strings.Contains(software, "-sc_threshold 0") || !strings.Contains(software, "repeat-headers=1") {
		t.Fatalf("x264 options missing: %s", software)
	}
	for _, encoder := range []string{"h264_nvenc", "h264_qsv", "h264_amf", "h264_videotoolbox"} {
		args := strings.Join(encoderFFmpegArgs(encoder, "3.1"), " ")
		if strings.Contains(args, "-sc_threshold") || strings.Contains(args, "x264-params") {
			t.Fatalf("%s received x264-only options: %s", encoder, args)
		}
	}
}

func TestNormalizeAgentEncoder(t *testing.T) {
	if got := normalizeAgentEncoder("H264_QSV"); got != "h264_qsv" {
		t.Fatalf("qsv normalization: %q", got)
	}
	if got := normalizeAgentEncoder("vp9"); got != "libx264" {
		t.Fatalf("unknown encoder fallback: %q", got)
	}
}

func TestMapWebRTCInputScalesToNativeScreen(t *testing.T) {
	raw := []byte(`{"type":"click","x":640,"y":360,"key":""}`)
	got := mapWebRTCInput(raw, 0, 0, 1920, 1080, 1280, 720, true)
	var ev webRTCInputEvent
	if err := json.Unmarshal(got, &ev); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ev.X != 960 || ev.Y != 540 {
		t.Fatalf("1080p mapped to (%d,%d), want (960,540)", ev.X, ev.Y)
	}

	// 16:10 source is pillarboxed inside the 16:9 stream; clicks in the black
	// bar clamp to the screen edge rather than landing on the desktop.
	raw = []byte(`{"type":"move","x":0,"y":360}`)
	got = mapWebRTCInput(raw, 0, 0, 1920, 1200, 1280, 720, true)
	if err := json.Unmarshal(got, &ev); err != nil {
		t.Fatalf("decode pillarbox: %v", err)
	}
	if ev.X != 0 || ev.Y != 600 {
		t.Fatalf("pillarbox mapped to (%d,%d), want (0,600)", ev.X, ev.Y)
	}
	raw = []byte(`{"type":"move","x":1279,"y":0}`)
	got = mapWebRTCInput(raw, 0, 0, 1920, 1200, 1280, 720, true)
	if err := json.Unmarshal(got, &ev); err != nil {
		t.Fatalf("decode edge: %v", err)
	}
	if ev.X != 1919 || ev.Y != 0 {
		t.Fatalf("edge mapped to (%d,%d), want (1919,0)", ev.X, ev.Y)
	}
}

func TestMapWebRTCInputPassesThroughKeyAndUnknownScreen(t *testing.T) {
	key := []byte(`{"type":"key","x":0,"y":0,"key":"enter"}`)
	if got := mapWebRTCInput(key, 0, 0, 1920, 1080, 1280, 720, true); string(got) != string(key) {
		t.Fatalf("key event changed: %s", got)
	}
	raw := []byte(`{"type":"move","x":640,"y":360}`)
	if got := mapWebRTCInput(raw, 0, 0, 0, 0, 1280, 720, false); string(got) != string(raw) {
		t.Fatalf("input changed without screen size: %s", got)
	}
}

func TestCappedBuffer(t *testing.T) {
	buf := &cappedBuffer{max: 5}
	if n, err := buf.Write([]byte("abcdefgh")); n != 8 || err != nil {
		t.Fatalf("Write = %d, %v", n, err)
	}
	if got := buf.String(); got != "abcde" {
		t.Fatalf("capped = %q", got)
	}
}
