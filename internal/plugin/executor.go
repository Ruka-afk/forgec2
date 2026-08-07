package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// execResult contains the raw output of a script execution.
type execResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	TimedOut bool
}

// executor runs plugin scripts with JSON input on stdin.
type executor struct{}

// run executes the plugin's entry script with the supplied input.
func (e *executor) run(ctx context.Context, pluginDir string, m *Manifest, input map[string]interface{}, timeoutSecs int) (*execResult, error) {
	timeout := timeoutSecs
	if timeout <= 0 {
		timeout = m.DefaultTimeout()
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	args := interpreterArgs(m.Interpreter, m.Entry)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = pluginDir

	// Plugin scripts run with a minimal environment instead of inheriting the
	// full server environment: secrets, tokens and machine context in env vars
	// must not be readable by (potentially untrusted) plugins. HOME/TMP* are
	// pinned to the OS temp directory so plugins cannot touch operator files.
	// Go toolchain variables (GOCACHE/GOPATH/GOROOT) are injected explicitly so
	// "go run" plugins still compile without leaking anything sensitive.
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"LANG=C.UTF-8",
		"HOME=" + os.TempDir(),
		"TMPDIR=" + os.TempDir(),
		"TMP=" + os.TempDir(),
		"TEMP=" + os.TempDir(),
		"GOCACHE=" + filepath.Join(os.TempDir(), "forgec2-plugin-gocache"),
		"GOPATH=" + filepath.Join(os.TempDir(), "forgec2-plugin-gopath"),
		"PLUGIN_NAME=" + m.Name,
		"PLUGIN_VERSION=" + m.Version,
	}
	if goroot := os.Getenv("GOROOT"); goroot != "" {
		env = append(env, "GOROOT="+goroot)
	}
	cmd.Env = env

	stdinData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plugin input: %w", err)
	}
	cmd.Stdin = bytes.NewReader(stdinData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	res := &execResult{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}

	if ctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		return res, fmt.Errorf("plugin %q timed out after %ds", m.Name, timeout)
	}
	if runErr != nil {
		stderrText := strings.TrimSpace(string(res.Stderr))
		if stderrText == "" {
			stderrText = runErr.Error()
		}
		return res, fmt.Errorf("plugin %q exited with code %d: %s", m.Name, res.ExitCode, stderrText)
	}
	return res, nil
}

// parseResult parses the executor's stdout as a plugin Result.
func parseResult(stdout []byte) (*Result, error) {
	stdout = bytes.TrimSpace(stdout)
	if len(stdout) == 0 {
		return nil, errors.New("plugin produced empty output")
	}
	var r Result
	if err := json.Unmarshal(stdout, &r); err != nil {
		return nil, fmt.Errorf("failed to parse plugin result: %w", err)
	}
	return &r, nil
}

// parseReport parses the executor's stdout as a plugin Report.
func parseReport(stdout []byte) (*Report, error) {
	stdout = bytes.TrimSpace(stdout)
	if len(stdout) == 0 {
		return nil, errors.New("plugin produced empty output")
	}
	var r Report
	if err := json.Unmarshal(stdout, &r); err != nil {
		return nil, fmt.Errorf("failed to parse plugin report: %w", err)
	}
	return &r, nil
}

// interpreterArgs builds the command-line arguments for the configured interpreter.
func interpreterArgs(interpreter, entry string) []string {
	switch interpreter {
	case "go":
		return []string{"go", "run", entry}
	case "powershell", "pwsh":
		return []string{interpreter, "-File", entry}
	default:
		return []string{interpreter, entry}
	}
}
