package server

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// SetupLogRotation configures lumberjack-based log rotation and returns
// a writer that sends output to both stdout and the rotating log file.
func SetupLogRotation(logDir string) io.Writer {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		slog.Error("Failed to create log directory", "dir", logDir, "err", err)
		return os.Stdout
	}

	lumberjackLogger := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "forgec2.log"),
		MaxSize:    100, // MB
		MaxBackups: 5,
		MaxAge:     30, // days
		Compress:   true,
	}

	return io.MultiWriter(os.Stdout, lumberjackLogger)
}
