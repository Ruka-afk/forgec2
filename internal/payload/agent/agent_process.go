//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// suspendProcess / resumeProcess allow pausing (freezing) processes e.g. games.
// target can be PID (e.g. "1234") or executable name (e.g. "game.exe").
// Useful for "pause game" scenarios.
func suspendProcess(target string) (string, error) {
	if runtime.GOOS == "windows" {
		return suspendProcessWindows(target)
	}
	// Linux
	cmd := exec.Command("kill", "-STOP", target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kill -STOP failed: %w: %s", err, string(out))
	}
	return "process suspended: " + target, nil
}

func resumeProcess(target string) (string, error) {
	if runtime.GOOS == "windows" {
		return resumeProcessWindows(target)
	}
	// Linux
	cmd := exec.Command("kill", "-CONT", target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kill -CONT failed: %w: %s", err, string(out))
	}
	return "process resumed: " + target, nil
}

// killProcess, clipboard*, findFiles, reg* are platform implemented
func killProcess(target string) (string, error) {
	if runtime.GOOS == "windows" {
		return killProcessWindows(target)
	}
	cmd := exec.Command("kill", "-9", target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kill failed: %w: %s", err, string(out))
	}
	return "killed: " + target, nil
}

func clipboardGet() (string, error) {
	return clipboardGetWindows()
}

func clipboardSet(data string) error {
	return clipboardSetWindows(data)
}

func findFiles(path, pattern string) (string, error) {
	if path == "" {
		path = "."
	}
	var results []string
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if pattern != "" {
			matched, _ := filepath.Match(pattern, filepath.Base(p))
			if !matched {
				return nil
			}
		}
		results = append(results, fmt.Sprintf("%s\t%d\t%s", p, info.Size(), info.ModTime().Format("2006-01-02 15:04")))
		return nil
	})
	if err != nil {
		return "", err
	}
	return strings.Join(results, "\n"), nil
}

func regGet(key string) (string, error) {
	if runtime.GOOS == "windows" {
		return regGetWindows(key)
	}
	return "", fmt.Errorf("registry only on Windows")
}

func regSet(path, data string) error {
	if runtime.GOOS == "windows" {
		return regSetWindows(path, data)
	}
	return fmt.Errorf("registry only on Windows")
}

func regDelete(key string) error {
	if runtime.GOOS == "windows" {
		return regDeleteWindows(key)
	}
	return fmt.Errorf("registry only on Windows")
}
