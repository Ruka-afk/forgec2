package server

import (
	"log/slog"
	"strings"
)

// logLevelVar is the single level gate for the process-wide slog handler.
// main.go builds its handler against this var, so SetLogLevel takes effect
// on config hot-reload without rebuilding loggers or restarting.
var logLevelVar = new(slog.LevelVar)

// LogLevelVar exposes the shared gate for handler construction at startup.
func LogLevelVar() *slog.LevelVar {
	return logLevelVar
}

// SetLogLevel parses a config log level and applies it live. Unknown values
// keep the previous level and report false.
func SetLogLevel(level string) bool {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		logLevelVar.Set(slog.LevelDebug)
	case "info", "":
		logLevelVar.Set(slog.LevelInfo)
	case "warn", "warning":
		logLevelVar.Set(slog.LevelWarn)
	case "error":
		logLevelVar.Set(slog.LevelError)
	default:
		return false
	}
	return true
}
