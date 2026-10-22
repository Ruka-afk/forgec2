package server

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// 鈹€鈹€ Marker protocol 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// TestUpdatePendingRoundTrip proves the pending marker survives a round trip:
// the restarter and the new server read it to agree on what is being
// installed.
func TestUpdatePendingRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, updatePendingFile)
	want := &updatePending{
		From:      "v2.7.0",
		To:        "v2.7.1",
		PID:       4242,
		StartedAt: time.Now().UTC().Truncate(time.Second),
		Exe:       filepath.Join(dir, "forgec2-server.exe"),
		New:       filepath.Join(dir, updateNewFile),
		Prev:      filepath.Join(dir, updatePrevFile),
		WorkDir:   dir,
		Args:      []string{"-config", "config.yaml"},
	}
	if err := writeUpdatePending(path, want); err != nil {
		t.Fatalf("writeUpdatePending: %v", err)
	}
	got, err := readUpdatePending(path)
	if err != nil {
		t.Fatalf("readUpdatePending: %v", err)
	}
	if got.From != want.From || got.To != want.To || got.PID != want.PID {
		t.Fatalf("pending = %+v", got)
	}
	if len(got.Args) != 2 || got.Args[0] != "-config" || got.Args[1] != "config.yaml" {
		t.Fatalf("args = %v", got.Args)
	}
}

// TestUpdateLastRoundTrip proves the outcome marker is readable.
func TestUpdateLastRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), updateLastFile)
	if err := writeUpdateLast(path, updateStatusRolledBack, "v2.7.1", "the new server did not report a healthy boot"); err != nil {
		t.Fatalf("writeUpdateLast: %v", err)
	}
	res, err := readUpdateLast(path)
	if err != nil {
		t.Fatalf("readUpdateLast: %v", err)
	}
	if res.Status != updateStatusRolledBack || res.Version != "v2.7.1" {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(res.Error, "healthy boot") {
		t.Fatalf("error = %q", res.Error)
	}
}

// TestMarkUpdateBootOKOnlyForThePendingBuild proves the boot marker is only
// written by the binary the restarter was asked to install. A stale marker
// must never vouch for a different build.
func TestMarkUpdateBootOKOnlyForThePendingBuild(t *testing.T) {
	dir := t.TempDir()
	if err := writeUpdatePending(filepath.Join(dir, updatePendingFile), &updatePending{From: "v2.7.0", To: "v2.7.1"}); err != nil {
		t.Fatalf("writeUpdatePending: %v", err)
	}

	wrote, err := markUpdateBootOK(dir, "v2.6.9")
	if err != nil {
		t.Fatalf("markUpdateBootOK: %v", err)
	}
	if wrote {
		t.Fatal("a build that is not the pending one wrote the boot marker")
	}
	if _, err := os.Stat(filepath.Join(dir, updateOKFile)); err == nil {
		t.Fatal("the ok marker was written by the wrong build")
	}

	wrote, err = markUpdateBootOK(dir, "v2.7.1")
	if err != nil {
		t.Fatalf("markUpdateBootOK: %v", err)
	}
	if !wrote {
		t.Fatal("the pending build did not write the boot marker")
	}
	if _, err := os.Stat(filepath.Join(dir, updateOKFile)); err != nil {
		t.Fatalf("the ok marker is missing: %v", err)
	}
}

// TestMarkUpdateBootOKWithoutPendingIsHarmless proves a normal boot (no
// update pending) writes nothing.
func TestMarkUpdateBootOKWithoutPendingIsHarmless(t *testing.T) {
	dir := t.TempDir()
	wrote, err := markUpdateBootOK(dir, "v2.7.1")
	if err != nil {
		t.Fatalf("markUpdateBootOK: %v", err)
	}
	if wrote {
		t.Fatal("a boot without a pending update wrote the boot marker")
	}
}

// 鈹€鈹€ Update lock 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// TestAcquireUpdateLockRefusesASecondHolder proves two updates cannot run at
// the same time against the same directory.
func TestAcquireUpdateLockRefusesASecondHolder(t *testing.T) {
	dir := t.TempDir()
	release, err := acquireUpdateLock(dir)
	if err != nil {
		t.Fatalf("acquireUpdateLock: %v", err)
	}
	if _, err := acquireUpdateLock(dir); err == nil {
		t.Fatal("a second update was allowed to run")
	}
	release()
	if _, err := acquireUpdateLock(dir); err != nil {
		t.Fatalf("the lock was not released: %v", err)
	}
}

// TestAcquireUpdateLockReclaimsAStaleLock proves a lock left behind by a
// crashed run does not block every future update.
func TestAcquireUpdateLockReclaimsAStaleLock(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, updateLockFile)
	if err := os.WriteFile(lockPath, []byte("pid=1\n"), 0o600); err != nil {
		t.Fatalf("seed lock: %v", err)
	}
	old := time.Now().Add(-2 * updateLockStaleAfter)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("age lock: %v", err)
	}
	release, err := acquireUpdateLock(dir)
	if err != nil {
		t.Fatalf("a stale lock was not reclaimed: %v", err)
	}
	release()
}

// 鈹€鈹€ Script rendering 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// TestRestartScriptKeepsTheOriginalCommandLine proves the restarter starts
// the new build with the same flags the running server was started with.
// The previous script started it with no arguments at all, so a server
// started with "-config other.yaml" came back on the default configuration.
func TestRestartScriptKeepsTheOriginalCommandLine(t *testing.T) {
	plan := restartPlan{
		Exe:     `/opt/forgec2/forgec2-server`,
		WorkDir: `/opt/forgec2`,
		Args:    []string{"-config", "/etc/forgec2/config.yaml", "-log-format", "json"},
	}
	unix := renderUnixRestartScript(plan)
	if !strings.Contains(unix, `'/etc/forgec2/config.yaml'`) {
		t.Fatalf("the unix script lost the config flag:\n%s", unix)
	}
	if !strings.Contains(unix, `'-log-format'`) || !strings.Contains(unix, `'json'`) {
		t.Fatalf("the unix script lost the log format flag:\n%s", unix)
	}

	win := renderWindowsRestartScript(plan)
	if !strings.Contains(win, `-ArgumentList $ArgLine`) {
		t.Fatalf("the windows script does not pass the command line:\n%s", win)
	}
	if !strings.Contains(win, `-config /etc/forgec2/config.yaml`) {
		t.Fatalf("the windows script lost the config flag:\n%s", win)
	}
}

// TestRestartScriptWaitsForTheOldProcess proves the restarter waits for the
// server it replaces to be gone before it touches the binary.
func TestRestartScriptWaitsForTheOldProcess(t *testing.T) {
	plan := restartPlan{OldPID: 4711}
	unix := renderUnixRestartScript(plan)
	if !strings.Contains(unix, "OLD_PID=4711") || !strings.Contains(unix, "kill -0") {
		t.Fatalf("the unix script does not wait for the old process:\n%s", unix)
	}
	win := renderWindowsRestartScript(plan)
	if !strings.Contains(win, "$OldPid = 4711") || !strings.Contains(win, "Get-Process -Id $OldPid") {
		t.Fatalf("the windows script does not wait for the old process:\n%s", win)
	}
}

// TestRestartScriptRollsBackWhenTheNewBuildNeverComesUp proves the rollback
// path is part of the generated script.
func TestRestartScriptRollsBackWhenTheNewBuildNeverComesUp(t *testing.T) {
	plan := restartPlan{}
	unix := renderUnixRestartScript(plan)
	if !strings.Contains(unix, `mv -f "$PREV" "$EXE"`) {
		t.Fatalf("the unix script cannot restore the backup:\n%s", unix)
	}
	if !strings.Contains(unix, "rolled_back") {
		t.Fatalf("the unix script does not record the rollback:\n%s", unix)
	}
	win := renderWindowsRestartScript(plan)
	if !strings.Contains(win, "Move-Item -LiteralPath $Prev -Destination $Exe") {
		t.Fatalf("the windows script cannot restore the backup:\n%s", win)
	}
	if !strings.Contains(win, "rolled_back") {
		t.Fatalf("the windows script does not record the rollback:\n%s", win)
	}
}

// TestWindowsQuoteArgFollowsCommandLineRules pins the quoting of the original
// command line: an argument with a space stays one argument.
func TestWindowsQuoteArgFollowsCommandLineRules(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"with space", `"with space"`},
		{`with"quote`, `"with` + `\"quote` + `"`},
		{"", `""`},
	}
	for _, tc := range cases {
		if got := windowsQuoteArg(tc.in); got != tc.want {
			t.Fatalf("windowsQuoteArg(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	line := windowsArgLine([]string{"-config", `C:\Program Files\forgec2\config.yaml`})
	if !strings.Contains(line, `"-config"`) {
		// A flag without spaces needs no quotes.
		if !strings.Contains(line, "-config") {
			t.Fatalf("command line lost the flag: %q", line)
		}
	}
	if !strings.Contains(line, `"C:\Program Files\forgec2\config.yaml"`) {
		t.Fatalf("command line lost the quoted path: %q", line)
	}
}

// 鈹€鈹€ Restarter integration 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// TestUpdateHelperProcess is not a test on its own: it is the stand-in for
// the freshly installed server binary in the restarter tests. The restarter
// starts the "new" binary; this helper reports a healthy boot (or, when
// FORGEC2_UPDATE_HELPER is set to "fail", stays silent) so the test can
// exercise both the success and the rollback path.
func TestUpdateHelperProcess(t *testing.T) {
	mode := os.Getenv("FORGEC2_UPDATE_HELPER")
	if mode == "" {
		t.Skip("helper process")
	}
	if mode == "ok" {
		if path := os.Getenv("FORGEC2_UPDATE_OK_PATH"); path != "" {
			_ = os.WriteFile(path, []byte("v9.9.9\n"), 0o600)
		}
	}
	time.Sleep(200 * time.Millisecond)
}

// restarterFixture prepares a directory that looks like an installation: a
// running binary, a downloaded replacement and the marker files.
type restarterFixture struct {
	dir      string
	exePath  string
	newPath  string
	prevPath string
	okPath   string
	lastPath string
	plan     restartPlan
}

func newRestarterFixture(t *testing.T, helperMode string) *restarterFixture {
	t.Helper()
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	selfData, err := os.ReadFile(self)
	if err != nil {
		t.Fatalf("read test binary: %v", err)
	}

	exeName := "forgec2-server"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	exePath := filepath.Join(dir, exeName)
	if err := os.WriteFile(exePath, selfData, 0o700); err != nil {
		t.Fatalf("seed installed binary: %v", err)
	}

	updateDir := updateStateDir(exePath)
	if err := os.MkdirAll(updateDir, 0o700); err != nil {
		t.Fatalf("create update dir: %v", err)
	}
	// The downloaded build differs in size from the installed one, so the
	// test can tell which one is in place after the swap.
	newPath := filepath.Join(updateDir, updateNewFile)
	newData := append(append([]byte{}, selfData...), []byte("FORGEC2-UPDATE-MARKER")...)
	if err := os.WriteFile(newPath, newData, 0o700); err != nil {
		t.Fatalf("seed downloaded binary: %v", err)
	}

	t.Setenv("FORGEC2_UPDATE_HELPER", helperMode)
	t.Setenv("FORGEC2_UPDATE_OK_PATH", filepath.Join(updateDir, updateOKFile))

	f := &restarterFixture{
		dir:      dir,
		exePath:  exePath,
		newPath:  newPath,
		prevPath: filepath.Join(updateDir, updatePrevFile),
		okPath:   filepath.Join(updateDir, updateOKFile),
		lastPath: filepath.Join(updateDir, updateLastFile),
	}
	f.plan = restartPlan{
		Exe:      exePath,
		New:      newPath,
		Prev:     f.prevPath,
		Pending:  filepath.Join(updateDir, updatePendingFile),
		OK:       f.okPath,
		Last:     f.lastPath,
		Lock:     filepath.Join(updateDir, updateLockFile),
		WorkDir:  dir,
		Args:     []string{"-test.run=TestUpdateHelperProcess"},
		OldPID:   999999, // a process that does not exist: the wait ends at once
		Target:   "v9.9.9",
		Deadline: 3,
	}
	return f
}

// run executes the generated restarter and returns its output.
func (f *restarterFixture) run(t *testing.T) string {
	t.Helper()
	script, err := renderRestartScript(runtime.GOOS, f.plan)
	if err != nil {
		t.Fatalf("renderRestartScript: %v", err)
	}
	scriptName := updateScriptUnix
	if runtime.GOOS == "windows" {
		scriptName = updateScriptWin
	}
	scriptPath := filepath.Join(f.dir, scriptName)
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write restarter: %v", err)
	}
	outPath := filepath.Join(f.dir, "restarter.out")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create output file: %v", err)
	}
	defer out.Close()

	cmd, err := restartCommand(scriptPath)
	if err != nil {
		t.Fatalf("restartCommand: %v", err)
	}
	cmd.Dir = f.plan.WorkDir
	cmd.Stdout = out
	cmd.Stderr = out
	_ = cmd.Run()

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read restarter output: %v", err)
	}
	return string(data)
}

// fileSHA256 returns the SHA-256 of a file.
func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestRestarterInstallsTheNewBuildAndReportsSuccess proves the happy path:
// the new binary is installed, the new server reports a healthy boot and the
// outcome is recorded.
func TestRestarterInstallsTheNewBuildAndReportsSuccess(t *testing.T) {
	f := newRestarterFixture(t, "ok")
	before := fileSHA256(t, f.exePath)

	out := f.run(t)
	t.Logf("restarter output:\n%s", out)

	res, err := readUpdateLast(f.lastPath)
	if err != nil {
		t.Fatalf("readUpdateLast: %v (output: %s)", err, out)
	}
	if res.Status != updateStatusSuccess {
		t.Fatalf("status = %q, want success (output: %s)", res.Status, out)
	}
	if after := fileSHA256(t, f.exePath); after == before {
		t.Fatal("the new binary was not installed")
	}
	if _, err := os.Stat(f.prevPath); err == nil {
		t.Fatal("the backup was left behind after a successful update")
	}
}

// TestRestarterRollsBackWhenTheNewBuildNeverReportsHealthyBoot proves the
// safety net: a build that does not come up is rolled back to the previous
// binary and the failure is recorded.
func TestRestarterRollsBackWhenTheNewBuildNeverReportsHealthyBoot(t *testing.T) {
	f := newRestarterFixture(t, "fail")
	before := fileSHA256(t, f.exePath)

	out := f.run(t)
	t.Logf("restarter output:\n%s", out)

	res, err := readUpdateLast(f.lastPath)
	if err != nil {
		t.Fatalf("readUpdateLast: %v (output: %s)", err, out)
	}
	if res.Status != updateStatusRolledBack {
		t.Fatalf("status = %q, want rolled_back (output: %s)", res.Status, out)
	}
	if after := fileSHA256(t, f.exePath); after != before {
		t.Fatal("the previous binary was not restored")
	}
}

// TestRestarterStartsTheNewBuildWithTheOriginalArguments proves the
// restarter passes the original command line to the new build: the stand-in
// binary only reports a healthy boot when it is started with the arguments
// the plan carries.
func TestRestarterStartsTheNewBuildWithTheOriginalArguments(t *testing.T) {
	f := newRestarterFixture(t, "ok")
	f.run(t)
	res, err := readUpdateLast(f.lastPath)
	if err != nil {
		t.Fatalf("readUpdateLast: %v", err)
	}
	if res.Status != updateStatusSuccess {
		t.Fatalf("status = %q, want success: the new build did not receive the original arguments", res.Status)
	}

	// The same restarter, but with arguments that do not start the helper:
	// the new build never reports a healthy boot, which proves the outcome
	// above came from the arguments and not from the restarter alone.
	g := newRestarterFixture(t, "ok")
	g.plan.Args = []string{"-test.run=TestNoSuchHelperTest"}
	g.run(t)
	res2, err := readUpdateLast(g.lastPath)
	if err != nil {
		t.Fatalf("readUpdateLast: %v", err)
	}
	if res2.Status != updateStatusRolledBack {
		t.Fatalf("status = %q, want rolled_back", res2.Status)
	}
}

// TestRestartCommandResolvesAnInterpreter proves the platform helper returns
// a runnable command.
func TestRestartCommandResolvesAnInterpreter(t *testing.T) {
	cmd, err := restartCommand(filepath.Join(t.TempDir(), "restart"))
	if err != nil {
		t.Fatalf("restartCommand: %v", err)
	}
	if cmd.Path == "" {
		t.Fatal("restartCommand returned an empty path")
	}
	if _, err := exec.LookPath(cmd.Path); err != nil {
		if runtime.GOOS == "windows" {
			// The interpreter is resolved from the system directory on
			// Windows, where it is not necessarily on PATH.
			return
		}
		t.Fatalf("interpreter %q is not runnable: %v", cmd.Path, err)
	}
}
