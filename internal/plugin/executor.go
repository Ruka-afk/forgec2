package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

// maxPluginOutputBytes caps captured plugin stdout/stderr per stream so a
// misbehaving plugin cannot OOM the server by emitting unbounded output.
// Output beyond the cap is truncated and flagged via truncatedOutputNote.
const maxPluginOutputBytes = 2 << 20 // 2 MiB per stream

const truncatedOutputNote = "\n[forgec2: output truncated at 2 MiB]"

// cappedBuffer is a bytes.Buffer that discards writes beyond max bytes.
type cappedBuffer struct {
	buf     bytes.Buffer
	max     int
	dropped int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	remaining := b.max - b.buf.Len()
	if remaining <= 0 {
		b.dropped += len(p)
		return len(p), nil
	}
	if len(p) > remaining {
		b.buf.Write(p[:remaining])
		b.dropped += len(p) - remaining
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *cappedBuffer) Bytes() []byte { return b.buf.Bytes() }

// sanitizePluginPATH strips PATH entries a plugin could abuse for binary
// hijacking: empty entries (imply CWD on Unix), "." and relative entries
// (resolved against the plugin dir / CWD at exec time), and duplicates.
// Absolute entries are kept verbatim so interpreters keep resolving. When
// nothing survives, a locked-down system default is returned instead of an
// empty PATH (which would imply CWD everywhere).
func sanitizePluginPATH(raw string) string {
	seen := make(map[string]bool)
	var kept []string
	for _, e := range strings.Split(raw, string(os.PathListSeparator)) {
		e = strings.TrimSpace(e)
		if e == "" || e == "." {
			continue
		}
		if !filepath.IsAbs(e) {
			continue
		}
		key := e
		if os.PathListSeparator == ';' {
			key = strings.ToLower(e)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, e)
	}
	if len(kept) > 0 {
		return strings.Join(kept, string(os.PathListSeparator))
	}
	if os.PathListSeparator == ';' {
		if sysroot := os.Getenv("SystemRoot"); sysroot != "" {
			return filepath.Join(sysroot, "System32") + ";" + sysroot
		}
		return `C:\Windows\System32;C:\Windows`
	}
	return "/usr/local/bin:/usr/bin:/bin"
}

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
	// New process group on Unix so timeout/guard can kill the whole tree.
	isolateProcessGroup(cmd)

	// Plugin scripts run with a minimal environment instead of inheriting the
	// full server environment: secrets, tokens and machine context in env vars
	// must not be readable by (potentially untrusted) plugins. HOME/TMP* are
	// pinned to the OS temp directory so plugins cannot touch operator files.
	// Go toolchain variables (GOCACHE/GOPATH/GOROOT) are injected explicitly so
	// "go run" plugins still compile without leaking anything sensitive.
	env := []string{
		"PATH=" + sanitizePluginPATH(os.Getenv("PATH")),
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

	var stdout, stderr cappedBuffer
	stdout.max = maxPluginOutputBytes
	stderr.max = maxPluginOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the process explicitly so a process-tree guard can be attached
	// (Windows Job Object with KILL_ON_JOB_CLOSE; Unix process-group kill)
	// before it does any work. On timeout the guard release below also
	// terminates surviving children, not just the main process.
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start plugin %q: %w", m.Name, err)
	}
	guard, guardErr := attachProcessGuard(cmd.Process)
	if guardErr != nil {
		slog.Warn("plugin process guard unavailable", "plugin", m.Name, "error", guardErr)
	}

	runErr := cmd.Wait()
	guard.release()

	res := &execResult{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}
	if stdout.dropped > 0 {
		res.Stdout = append(res.Stdout, truncatedOutputNote...)
		slog.Warn("plugin stdout truncated", "plugin", m.Name, "dropped_bytes", stdout.dropped)
	}
	if stderr.dropped > 0 {
		res.Stderr = append(res.Stderr, truncatedOutputNote...)
		slog.Warn("plugin stderr truncated", "plugin", m.Name, "dropped_bytes", stderr.dropped)
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
