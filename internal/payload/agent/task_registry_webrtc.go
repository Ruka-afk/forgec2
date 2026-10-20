//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "webrtc" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module, so
// Pion and ffmpeg helpers are not linked into agents that only need JPEG screen
// streaming.

func init() {
	taskHandlers["screen_webrtc_start"] = handleScreenWebRTCStart
	taskHandlers["screen_webrtc_stop"] = handleScreenWebRTCStop
}
