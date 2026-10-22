package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Hot-update markers.
//
// The server replaces its own binary, so the work is split in two: the Go
// process prepares everything (download, verify, snapshot) and a tiny
// generated restarter performs the swap once the old process is gone.
//
// The restarter and the new server talk through three files in a ".update"
// directory next to the binary:
//
//	pending.json  what the running server is about to install (written by Go)
//	ok            "the new server booted and bound its port" (written by the
//	              new server once its listener is up)
//	last          the outcome of the attempt (written by the restarter)
//
// The restarter rolls back to the previous binary when "ok" does not appear
// in time, which is what turns a broken release into a short outage instead
// of a dead service.

const (
	updatePendingFile = "pending.json"
	updateOKFile      = "ok"
	updateLastFile    = "last"
	updateLockFile    = "lock"
	updatePrevFile    = "prev.bin"
	updateNewFile     = "new.bin"
	updateScriptUnix  = "restart.sh"
	updateScriptWin   = "restart.ps1"
)

// updatePending describes an in-flight hot update.
type updatePending struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Exe       string    `json:"exe"`
	New       string    `json:"new"`
	Prev      string    `json:"prev"`
	WorkDir   string    `json:"work_dir"`
	Args      []string  `json:"args"`
	DBPath    string    `json:"db_path,omitempty"`
	DBBackup  string    `json:"db_backup,omitempty"`
	Script    string    `json:"script,omitempty"`
}

// updateStateDir is the directory that holds the hot-update markers.
func updateStateDir(exePath string) string {
	return filepath.Join(filepath.Dir(exePath), ".update")
}

// writeUpdatePending records what is about to be installed.
func writeUpdatePending(path string, p *updatePending) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("encode pending: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write pending: %w", err)
	}
	return nil
}

// readUpdatePending loads the pending marker, if any.
func readUpdatePending(path string) (*updatePending, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p updatePending
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse pending: %w", err)
	}
	return &p, nil
}

// updateLastResult is the outcome the restarter recorded.
type updateLastResult struct {
	Status  string    `json:"status"`
	Version string    `json:"version"`
	At      time.Time `json:"at"`
	Error   string    `json:"error,omitempty"`
}

// Update result statuses.
const (
	updateStatusSuccess     = "success"
	updateStatusFailed      = "failed"
	updateStatusRolledBack  = "rolled_back"
	updateStatusInterrupted = "interrupted"
)

// writeUpdateLast records the outcome of an update attempt.
func writeUpdateLast(path, status, version, errMsg string) error {
	var b strings.Builder
	b.WriteString("status=" + status + "\n")
	b.WriteString("version=" + version + "\n")
	b.WriteString("at=" + time.Now().UTC().Format(time.RFC3339) + "\n")
	if errMsg != "" {
		b.WriteString("error=" + strings.ReplaceAll(errMsg, "\n", " ") + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write last: %w", err)
	}
	return nil
}

// readUpdateLast loads the recorded outcome of the last update attempt.
func readUpdateLast(path string) (*updateLastResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	res := &updateLastResult{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "status":
			res.Status = value
		case "version":
			res.Version = value
		case "error":
			res.Error = value
		case "at":
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				res.At = t
			}
		}
	}
	if res.Status == "" {
		return nil, fmt.Errorf("last marker has no status")
	}
	return res, nil
}

// markUpdateBootOK tells the restarter that the new server is up. It is a
// no-op unless this process is the binary the restarter was asked to install,
// so a stale marker can never vouch for the wrong build.
func markUpdateBootOK(dir string, version string) (bool, error) {
	pending, err := readUpdatePending(filepath.Join(dir, updatePendingFile))
	if err != nil {
		return false, nil // nothing pending: not an update boot
	}
	if pending.To != version {
		return false, nil
	}
	okPath := filepath.Join(dir, updateOKFile)
	if err := os.WriteFile(okPath, []byte(version+"\n"), 0o600); err != nil {
		return false, fmt.Errorf("write ok marker: %w", err)
	}
	return true, nil
}

// reconcileUpdateResult records what happened to the last update attempt.
//
// The restarter owns the outcome: it writes "last" once it has seen the boot
// marker (or once it gave up). The server only reports what it finds there.
//
// The one case the server has to conclude itself is an update that never
// finished: a pending marker without an outcome and without a live restarter
// means the machine was restarted in the middle of the swap.
func (s *Server) reconcileUpdateResult(dir string) {
	pending, pendingErr := readUpdatePending(filepath.Join(dir, updatePendingFile))
	lastPath := filepath.Join(dir, updateLastFile)

	last, lastErr := readUpdateLast(lastPath)
	if lastErr != nil && pendingErr == nil {
		if pending.To == ServerVersion {
			// This process IS the pending build: the restarter is still
			// running and will report the outcome. Concluding "interrupted"
			// here would contradict the restarter a second later.
			return
		}
		// A pending update for another build and no outcome: the process
		// died (or the host rebooted) between the swap and the report.
		last = &updateLastResult{
			Status:  updateStatusInterrupted,
			Version: pending.To,
			At:      time.Now(),
			Error:   "the server restarted before the update could be confirmed",
		}
		_ = writeUpdateLast(lastPath, last.Status, last.Version, last.Error)
	}
	if last == nil {
		return
	}
	s.auditUpdateResult(last)
}

// auditUpdateResult records the outcome of an update attempt in the audit
// trail.
func (s *Server) auditUpdateResult(last *updateLastResult) {
	details := fmt.Sprintf("hot update to %s: %s", last.Version, last.Status)
	if last.Error != "" {
		details += " (" + last.Error + ")"
	}
	slog.Info("Hot update result", "status", last.Status, "version", last.Version, "error", last.Error)
	s.LogAuditRecord(nil, "hot_update_result", "server", "self", details, last.Status == updateStatusSuccess, nil)
}

// awaitUpdateResult waits for the restarter's outcome and records it. The
// new build writes the boot marker before the restarter has decided whether
// the update succeeded, so the outcome only becomes known a moment later.
func (s *Server) awaitUpdateResult(dir string) {
	s.wg.Add(1)
	defer s.wg.Done()
	deadline := time.Now().Add(updateResultWaitTimeout)
	for time.Now().Before(deadline) {
		res, err := readUpdateLast(filepath.Join(dir, updateLastFile))
		if err == nil && res.Status != "" {
			s.auditUpdateResult(res)
			return
		}
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// markUpdateBootOK writes the boot marker when this process is the binary a
// pending hot update installed.
func (s *Server) markUpdateBootOK() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	if exePath, err = filepath.Abs(exePath); err != nil {
		return
	}
	dir := updateStateDir(exePath)
	wrote, err := markUpdateBootOK(dir, ServerVersion)
	if err != nil {
		slog.Warn("Hot update: could not write the boot marker", "err", err)
		return
	}
	if wrote {
		slog.Info("Hot update: reported a healthy boot", "version", ServerVersion)
		go s.awaitUpdateResult(dir)
		return
	}
	s.reconcileUpdateResult(dir)
}

// restartPlan describes the swap the generated restarter performs.
type restartPlan struct {
	Exe      string
	New      string
	Prev     string
	Pending  string
	OK       string
	Last     string
	Lock     string
	WorkDir  string
	Args     []string
	OldPID   int
	Target   string
	Script   string
	DBPath   string
	DBBackup string
	// Deadline bounds how long the restarter waits for the old process to
	// exit and for the new build to report a healthy boot. Zero means the
	// production default.
	Deadline int
}

// deadline returns the effective wait budget of the restarter.
func (p restartPlan) deadline() int {
	if p.Deadline > 0 {
		return p.Deadline
	}
	return updateRestartDeadlineSeconds
}

// renderRestartScript generates the restarter for the given platform.
//
// The script is deliberately dumb: it waits for the old process to exit,
// backs up the current binary, installs the new one, starts it with the
// original command line and rolls back when the new server does not report a
// healthy boot. Keeping the logic in the script (and not in the Go process,
// which is about to exit) is what makes the rollback possible.
func renderRestartScript(goos string, p restartPlan) (string, error) {
	switch goos {
	case "windows":
		return renderWindowsRestartScript(p), nil
	case "linux", "darwin", "freebsd", "netbsd", "openbsd":
		return renderUnixRestartScript(p), nil
	default:
		return "", fmt.Errorf("hot update is not supported on %s", goos)
	}
}

// renderUnixRestartScript generates the POSIX restarter.
func renderUnixRestartScript(p restartPlan) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# ForgeC2 hot-update restarter (generated; safe to delete).\n")
	b.WriteString("set -u\n\n")
	fmt.Fprintf(&b, "EXE=%s\n", quoteSh(p.Exe))
	fmt.Fprintf(&b, "NEW=%s\n", quoteSh(p.New))
	fmt.Fprintf(&b, "PREV=%s\n", quoteSh(p.Prev))
	fmt.Fprintf(&b, "PENDING=%s\n", quoteSh(p.Pending))
	fmt.Fprintf(&b, "OKFILE=%s\n", quoteSh(p.OK))
	fmt.Fprintf(&b, "LASTFILE=%s\n", quoteSh(p.Last))
	fmt.Fprintf(&b, "LOCKFILE=%s\n", quoteSh(p.Lock))
	fmt.Fprintf(&b, "TARGET=%s\n", quoteSh(p.Target))
	fmt.Fprintf(&b, "DBPATH=%s\n", quoteSh(p.DBPath))
	fmt.Fprintf(&b, "DBBACKUP=%s\n", quoteSh(p.DBBackup))
	fmt.Fprintf(&b, "OLD_PID=%d\n", p.OldPID)
	fmt.Fprintf(&b, "WORKDIR=%s\n", quoteSh(p.WorkDir))
	fmt.Fprintf(&b, "DEADLINE=%d\n", p.deadline())
	b.WriteString("\nwrite_last() {\n")
	b.WriteString("  {\n")
	b.WriteString("    echo \"status=$1\"\n")
	b.WriteString("    echo \"version=$TARGET\"\n")
	b.WriteString("    echo \"at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')\"\n")
	b.WriteString("    if [ -n \"${2:-}\" ]; then echo \"error=$2\"; fi\n")
	b.WriteString("  } >\"$LASTFILE\" 2>/dev/null\n")
	b.WriteString("}\n\n")
	b.WriteString("cleanup() {\n")
	b.WriteString("  rm -f \"$NEW\" \"$PENDING\" \"$OKFILE\" \"$LOCKFILE\" \"$DBBACKUP\" 2>/dev/null\n")
	b.WriteString("}\n\n")
	b.WriteString("cd \"$WORKDIR\" 2>/dev/null || true\n\n")
	b.WriteString("# 1. wait for the old process to exit\n")
	b.WriteString("i=0\n")
	b.WriteString("while kill -0 \"$OLD_PID\" 2>/dev/null; do\n")
	b.WriteString("  i=$((i + 1))\n")
	b.WriteString("  if [ \"$i\" -ge \"$DEADLINE\" ]; then\n")
	b.WriteString("    write_last failed \"the previous server process did not exit\"\n")
	b.WriteString("    rm -f \"$LOCKFILE\" 2>/dev/null\n")
	b.WriteString("    exit 1\n")
	b.WriteString("  fi\n")
	b.WriteString("  sleep 1\n")
	b.WriteString("done\n\n")
	b.WriteString("# 2. back up the current binary so a failed boot can be undone\n")
	b.WriteString("if ! cp -p \"$EXE\" \"$PREV\" 2>/dev/null; then\n")
	b.WriteString("  write_last failed \"could not back up the current binary\"\n")
	b.WriteString("  rm -f \"$LOCKFILE\" 2>/dev/null\n")
	b.WriteString("  exit 1\n")
	b.WriteString("fi\n\n")
	b.WriteString("# 3. install the new binary\n")
	b.WriteString("if ! mv -f \"$NEW\" \"$EXE\" 2>/dev/null; then\n")
	b.WriteString("  write_last failed \"could not install the new binary\"\n")
	b.WriteString("  rm -f \"$PREV\" 2>/dev/null\n")
	b.WriteString("  rm -f \"$LOCKFILE\" 2>/dev/null\n")
	b.WriteString("  exit 1\n")
	b.WriteString("fi\n")
	b.WriteString("chmod 0755 \"$EXE\" 2>/dev/null\n\n")
	b.WriteString("# 4. start the new server with the original command line\n")
	fmt.Fprintf(&b, "\"$EXE\"%s &\n", unixArgSuffix(p.Args))
	b.WriteString("NEW_PID=$!\n\n")
	b.WriteString("# 5. wait for the new server to report a healthy boot\n")
	b.WriteString("i=0\n")
	b.WriteString("while [ \"$i\" -lt \"$DEADLINE\" ]; do\n")
	b.WriteString("  if [ -f \"$OKFILE\" ]; then\n")
	b.WriteString("    write_last success \"\"\n")
	b.WriteString("    cleanup\n")
	b.WriteString("    rm -f \"$PREV\" 2>/dev/null\n")
	b.WriteString("    rm -f \"$0\" 2>/dev/null\n")
	b.WriteString("    exit 0\n")
	b.WriteString("  fi\n")
	b.WriteString("  if ! kill -0 \"$NEW_PID\" 2>/dev/null; then break; fi\n")
	b.WriteString("  i=$((i + 1))\n")
	b.WriteString("  sleep 1\n")
	b.WriteString("done\n\n")
	b.WriteString("# 6. the new server never came up: roll back to the backup\n")
	b.WriteString("kill \"$NEW_PID\" 2>/dev/null\n")
	b.WriteString("if [ -f \"$PREV\" ]; then\n")
	b.WriteString("  mv -f \"$PREV\" \"$EXE\" 2>/dev/null\n")
	b.WriteString("  chmod 0755 \"$EXE\" 2>/dev/null\n")
	b.WriteString("fi\n")
	b.WriteString("# 6b. restore the database snapshot: the failed build may have migrated it\n")
	b.WriteString("if [ -n \"$DBPATH\" ] && [ -f \"$DBBACKUP\" ]; then\n")
	b.WriteString("  cp -f \"$DBBACKUP\" \"$DBPATH\" 2>/dev/null\n")
	b.WriteString("  rm -f \"$DBPATH-wal\" \"$DBPATH-shm\" 2>/dev/null\n")
	b.WriteString("fi\n")
	// The outcome is recorded before the previous build is started again: the
	// server that comes back reads it and reports it in the audit trail.
	b.WriteString("Write-Last 'rolled_back' 'the new server did not report a healthy boot'\n")
	fmt.Fprintf(&b, "\"$EXE\"%s >/dev/null 2>&1 &\n", unixArgSuffix(p.Args))
	b.WriteString("cleanup\n")
	b.WriteString("rm -f \"$0\" 2>/dev/null\n")
	b.WriteString("exit 1\n")
	return b.String()
}

// unixArgSuffix renders the original command line for a POSIX shell.
func unixArgSuffix(args []string) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, quoteSh(a))
	}
	return " " + strings.Join(parts, " ")
}

// renderWindowsRestartScript generates the Windows restarter.
//
// The restarter is a PowerShell script: the wait-for-exit, the retry loop
// around the file swap and the rollback are far easier to get right there
// than in cmd batch syntax, and PowerShell ships with every supported
// Windows release.
func renderWindowsRestartScript(p restartPlan) string {
	var b strings.Builder
	b.WriteString("# ForgeC2 hot-update restarter (generated; safe to delete).\n")
	b.WriteString("$ErrorActionPreference = 'Continue'\n\n")
	fmt.Fprintf(&b, "$Exe = %s\n", quotePs(p.Exe))
	fmt.Fprintf(&b, "$New = %s\n", quotePs(p.New))
	fmt.Fprintf(&b, "$Prev = %s\n", quotePs(p.Prev))
	fmt.Fprintf(&b, "$Pending = %s\n", quotePs(p.Pending))
	fmt.Fprintf(&b, "$OkFile = %s\n", quotePs(p.OK))
	fmt.Fprintf(&b, "$LastFile = %s\n", quotePs(p.Last))
	fmt.Fprintf(&b, "$LockFile = %s\n", quotePs(p.Lock))
	fmt.Fprintf(&b, "$Target = %s\n", quotePs(p.Target))
	fmt.Fprintf(&b, "$DbPath = %s\n", quotePs(p.DBPath))
	fmt.Fprintf(&b, "$DbBackup = %s\n", quotePs(p.DBBackup))
	fmt.Fprintf(&b, "$OldPid = %d\n", p.OldPID)
	fmt.Fprintf(&b, "$WorkDir = %s\n", quotePs(p.WorkDir))
	fmt.Fprintf(&b, "$Deadline = %d\n", p.deadline())
	b.WriteString("$ArgLine = " + quotePs(windowsArgLine(p.Args)) + "\n\n")
	b.WriteString("function Write-Last([string]$status, [string]$err) {\n")
	b.WriteString("  $lines = @(\"status=$status\", \"version=$Target\", \"at=$([DateTime]::UtcNow.ToString('o'))\")\n")
	b.WriteString("  if ($err -ne '') { $lines += \"error=$err\" }\n")
	b.WriteString("  Set-Content -LiteralPath $LastFile -Value $lines -Encoding ASCII\n")
	b.WriteString("}\n\n")
	b.WriteString("function Start-Server {\n")
	b.WriteString("  if ($ArgLine -eq '') {\n")
	b.WriteString("    return Start-Process -FilePath $Exe -WorkingDirectory $WorkDir -PassThru -WindowStyle Hidden\n")
	b.WriteString("  }\n")
	b.WriteString("  return Start-Process -FilePath $Exe -ArgumentList $ArgLine -WorkingDirectory $WorkDir -PassThru -WindowStyle Hidden\n")
	b.WriteString("}\n\n")
	b.WriteString("Set-Location -LiteralPath $WorkDir -ErrorAction SilentlyContinue\n\n")
	b.WriteString("# 1. wait for the old process to exit\n")
	b.WriteString("$waited = 0\n")
	b.WriteString("while ($waited -lt $Deadline) {\n")
	b.WriteString("  if ($null -eq (Get-Process -Id $OldPid -ErrorAction SilentlyContinue)) { break }\n")
	b.WriteString("  Start-Sleep -Seconds 1\n")
	b.WriteString("  $waited++\n")
	b.WriteString("}\n")
	b.WriteString("if ($null -ne (Get-Process -Id $OldPid -ErrorAction SilentlyContinue)) {\n")
	b.WriteString("  Write-Last 'failed' 'the previous server process did not exit'\n")
	b.WriteString("  Remove-Item -LiteralPath $LockFile -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("  exit 1\n")
	b.WriteString("}\n\n")
	b.WriteString("# 2. back up the current binary so a failed boot can be undone\n")
	b.WriteString("try {\n")
	b.WriteString("  Copy-Item -LiteralPath $Exe -Destination $Prev -Force -ErrorAction Stop\n")
	b.WriteString("} catch {\n")
	b.WriteString("  Write-Last 'failed' ('could not back up the current binary: ' + $_.Exception.Message)\n")
	b.WriteString("  Remove-Item -LiteralPath $LockFile -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("  exit 1\n")
	b.WriteString("}\n\n")
	b.WriteString("# 3. install the new binary (the old image may still hold the file lock)\n")
	b.WriteString("$installed = $false\n")
	b.WriteString("for ($i = 0; $i -lt 10; $i++) {\n")
	b.WriteString("  try {\n")
	b.WriteString("    Move-Item -LiteralPath $New -Destination $Exe -Force -ErrorAction Stop\n")
	b.WriteString("    $installed = $true\n")
	b.WriteString("    break\n")
	b.WriteString("  } catch {\n")
	b.WriteString("    Start-Sleep -Milliseconds 500\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")
	b.WriteString("if (-not $installed) {\n")
	b.WriteString("  Write-Last 'failed' 'could not install the new binary'\n")
	b.WriteString("  Remove-Item -LiteralPath $Prev -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("  Remove-Item -LiteralPath $LockFile -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("  exit 1\n")
	b.WriteString("}\n\n")
	b.WriteString("# 4. start the new server with the original command line\n")
	b.WriteString("$proc = $null\n")
	b.WriteString("try { $proc = Start-Server } catch { $proc = $null }\n\n")
	b.WriteString("# 5. wait for the new server to report a healthy boot\n")
	b.WriteString("$waited = 0\n")
	b.WriteString("while ($waited -lt $Deadline) {\n")
	b.WriteString("  if (Test-Path -LiteralPath $OkFile) {\n")
	b.WriteString("    Write-Last 'success' ''\n")
	b.WriteString("    Remove-Item -LiteralPath $New,$Pending,$OkFile,$LockFile,$DbBackup -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("    Remove-Item -LiteralPath $Prev -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("    Remove-Item -LiteralPath $MyInvocation.MyCommand.Path -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("    exit 0\n")
	b.WriteString("  }\n")
	b.WriteString("  if ($null -eq $proc -or $proc.HasExited) { break }\n")
	b.WriteString("  Start-Sleep -Seconds 1\n")
	b.WriteString("  $waited++\n")
	b.WriteString("}\n\n")
	b.WriteString("# 6. the new server never came up: roll back to the backup\n")
	b.WriteString("try { if (-not $proc.HasExited) { $proc.Kill() } } catch {}\n")
	b.WriteString("if (Test-Path -LiteralPath $Prev) {\n")
	b.WriteString("  Move-Item -LiteralPath $Prev -Destination $Exe -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("}\n")
	b.WriteString("# 6b. restore the database snapshot: the failed build may have migrated it\n")
	b.WriteString("if ($DbPath -ne '' -and (Test-Path -LiteralPath $DbBackup)) {\n")
	b.WriteString("  Copy-Item -LiteralPath $DbBackup -Destination $DbPath -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("  Remove-Item -LiteralPath ($DbPath + '-wal'), ($DbPath + '-shm') -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("}\n")
	// The outcome is recorded before the previous build is started again: the
	// server that comes back reads it and reports it in the audit trail.
	b.WriteString("Write-Last 'rolled_back' 'the new server did not report a healthy boot'\n")
	b.WriteString("Start-Server | Out-Null\n")
	b.WriteString("Remove-Item -LiteralPath $New,$Pending,$OkFile,$LockFile,$DbBackup -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("Remove-Item -LiteralPath $MyInvocation.MyCommand.Path -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("exit 1\n")
	return b.String()
}

// windowsArgLine renders the original command line for the Windows
// command-line parser (the rules CommandLineToArgvW implements).
func windowsArgLine(args []string) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, windowsQuoteArg(a))
	}
	return strings.Join(parts, " ")
}

// windowsQuoteArg quotes one argument for the Windows command line.
//
// The rules are the ones the C runtime uses: an argument with a space or a
// quote is wrapped in quotes, backslashes are doubled in front of a quote
// and a quote is escaped with a backslash.
func windowsQuoteArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if !strings.ContainsAny(arg, " \t\"") {
		return arg
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for i := 0; i < len(arg); i++ {
		c := arg[i]
		switch {
		case c == '\\':
			slashes++
			continue
		case c == '"':
			b.WriteString(strings.Repeat("\\", slashes*2+1))
			slashes = 0
			b.WriteByte('"')
			continue
		}
		b.WriteString(strings.Repeat("\\", slashes))
		slashes = 0
		b.WriteByte(c)
	}
	b.WriteString(strings.Repeat("\\", slashes*2))
	b.WriteByte('"')
	return b.String()
}

// quotePs renders a value as a PowerShell single-quoted string.
func quotePs(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// quoteCmdArg renders a value as a cmd.exe argument. It is used for the
// paths the restarter embeds in a cmd context.
func quoteCmdArg(value string) string {
	escaped := strings.ReplaceAll(value, "%", "%%")
	return `"` + strings.ReplaceAll(escaped, `"`, `""`) + `"`
}

// formatPID renders a process id for the generated scripts.
func formatPID(pid int) string {
	return strconv.Itoa(pid)
}
