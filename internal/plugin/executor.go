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
type executor struct {
	// slots bounds concurrent plugin processes; nil means the executor runs
	// without a quota (used by tests that exercise a single run).
	slots chan struct{}
}

func newExecutor(maxConcurrent int) *executor {
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrentPlugins
	}
	return &executor{slots: make(chan struct{}, maxConcurrent)}
}

// acquire reserves an execution slot, honouring ctx cancellation so a saturated
// server rejects plugin work instead of queueing it without bound.
func (e *executor) acquire(ctx context.Context) error {
	if e.slots == nil {
		return nil
	}
	select {
	case e.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *executor) release() {
	if e.slots == nil {
		return
	}
	select {
	case <-e.slots:
	default:
	}
}

// maxPluginOutputBytes caps captured plugin stdout/stderr per stream so a
// misbehaving plugin cannot OOM the server by emitting unbounded output.
// Output beyond the cap is truncated and flagged via truncatedOutputNote.
const maxPluginOutputBytes = 2 << 20 // 2 MiB per stream

// maxPluginTimeoutSecs is the server-enforced ceiling on plugin runtimes.
// A manifest may request less, never more: an untrusted manifest asking for a
// multi-hour run must not pin a worker (and its process tree) indefinitely.
const maxPluginTimeoutSecs = 300

// defaultMaxConcurrentPlugins bounds how many plugin processes may run at the
// same time. Hooks fire on beacon-driven paths, so without a cap a burst of
// events (or several concurrent reports) can fan out an unbounded number of
// interpreters.
const defaultMaxConcurrentPlugins = 4

// pluginWaitDelay bounds how long Wait blocks after the context fires when a
// grandchild inherited the stdout/stderr pipes. Without it, cmd.Wait hangs
// past the timeout and the guard never runs.
const pluginWaitDelay = 5 * time.Second

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

// clampPluginTimeout applies the server-enforced ceiling to a requested
// runtime. Non-positive values fall back to the manifest default.
func clampPluginTimeout(requested, manifestDefault int) int {
	t := requested
	if t <= 0 {
		t = manifestDefault
	}
	if t > maxPluginTimeoutSecs {
		return maxPluginTimeoutSecs
	}
	return t
}

// run executes the plugin's entry script with the supplied input.
func (e *executor) run(ctx context.Context, pluginDir string, m *Manifest, input map[string]interface{}, timeoutSecs int) (*execResult, error) {
	requested := timeoutSecs
	if requested <= 0 {
		requested = m.DefaultTimeout()
	}
	if requested > maxPluginTimeoutSecs {
		slog.Warn("plugin timeout clamped to server maximum", "plugin", m.Name, "requested_s", requested, "max_s", maxPluginTimeoutSecs)
	}
	timeout := clampPluginTimeout(timeoutSecs, m.DefaultTimeout())

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Concurrency quota: cap simultaneous interpreters so event bursts cannot
	// fan out an unbounded number of processes.
	if err := e.acquire(ctx); err != nil {
		return nil, fmt.Errorf("plugin %q not started: execution queue full or cancelled: %w", m.Name, err)
	}
	defer e.release()

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
	// A grandchild that inherited the stdout/stderr pipes keeps Wait blocked
	// after the context fires; WaitDelay caps that and then force-closes them.
	cmd.WaitDelay = pluginWaitDelay

	guard, guardErr := attachProcessGuard(cmd.Process)
	if guardErr != nil {
		// Fail closed: without tree-kill the plugin (and anything it spawns)
		// can outlive its timeout. Kill what we started and refuse the run.
		slog.Error("plugin process guard unavailable, refusing run", "plugin", m.Name, "error", guardErr)
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("plugin %q refused: process isolation unavailable: %w", m.Name, guardErr)
	}

	// Release the guard the moment the context is done instead of waiting for
	// Wait to return: timeout enforcement must not depend on a well-behaved
	// process tree.
	guardDone := make(chan struct{})
	defer close(guardDone)
	go func() {
		select {
		case <-ctx.Done():
			guard.release()
		case <-guardDone:
		}
	}()

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
