package payload

import (
	"runtime"
	"strings"
	"testing"
)

// TestCheckDLLBuildPreconditionsRejectsObfuscation is the deterministic half of
// the contract: garble cannot be combined with -buildmode=c-shared, and that
// is knowable without touching the host toolchain.
//
// Only real architectures are exercised, because resolveBuildArch runs first
// (same order as GenerateWindowsDLL, which resolves the arch at line 133 before
// handing off to buildAgentBinaryDLL) and an empty or bogus arch is rejected
// there — covered separately by TestCheckDLLBuildPreconditionsRejectsUnsupportedArch.
func TestCheckDLLBuildPreconditionsRejectsObfuscation(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		err := CheckDLLBuildPreconditions(arch, true)
		if err == nil {
			t.Fatalf("arch %q with obfuscation: got nil, want rejection", arch)
		}
		if !strings.Contains(err.Error(), "garble") {
			t.Fatalf("arch %q: error = %q, want it to name garble/obfuscation", arch, err.Error())
		}
	}
}

// TestCheckDLLBuildPreconditionsRejectsUnsupportedArch keeps the pre-flight
// check in step with resolveBuildArch, so a bad architecture is reported before
// a job is queued instead of inside it.
func TestCheckDLLBuildPreconditionsRejectsUnsupportedArch(t *testing.T) {
	err := CheckDLLBuildPreconditions("vax", false)
	if err == nil {
		t.Fatal("unsupported architecture: got nil, want rejection")
	}
	if !strings.Contains(err.Error(), "vax") {
		t.Fatalf("error = %q, want it to name the offending architecture", err.Error())
	}
}

// TestCheckDLLBuildPreconditionsMatchesTheBuildTimeCheck is the important one:
// the pre-flight must not reject anything the real build would accept. If the
// two disagree, a server that can produce DLLs would be told it cannot, which
// is worse than the slow failure this change set out to remove.
func TestCheckDLLBuildPreconditionsMatchesTheBuildTimeCheck(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		goarch, err := resolveBuildArch("windows", arch)
		if err != nil {
			t.Fatalf("resolveBuildArch(%q): %v", arch, err)
		}
		preflight := CheckDLLBuildPreconditions(arch, false)
		buildTime := checkCgoCrossCompiler(goarch)
		switch {
		case preflight == nil && buildTime != nil:
			t.Fatalf("arch %q: pre-flight passed but the build-time check would fail (%v)", arch, buildTime)
		case preflight != nil && buildTime == nil:
			t.Fatalf("arch %q: pre-flight rejected a build that would have succeeded (%v)", arch, preflight)
		}
	}
}

// TestCheckDLLBuildPreconditionsErrorIsActionable makes sure an operator is
// told what to install rather than just that something is missing. The message
// is the only thing they get, because the request is refused outright.
func TestCheckDLLBuildPreconditionsErrorIsActionable(t *testing.T) {
	err := CheckDLLBuildPreconditions("amd64", false)
	if err == nil {
		t.Skip("this host has a CGO toolchain for windows/amd64; nothing to refuse")
	}
	msg := err.Error()
	for _, want := range []string{"mingw", "install"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("refusal %q does not mention %q, so the operator is not told what to do", msg, want)
		}
	}
	t.Logf("host %s: refusal is %q", runtime.GOOS, msg)
}
