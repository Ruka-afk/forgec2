package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
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
