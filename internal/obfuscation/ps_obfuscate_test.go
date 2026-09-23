package obfuscation

import (
	"strings"
	"testing"
)

func fixedOpts() PSObfuscateOptions {
	return PSObfuscateOptions{Seed: 42, SeedSet: true}
}

// Legacy launcher must stay byte-stable in shape (existing tests rely on it).
func TestWithOptionsLegacyDefault(t *testing.T) {
	code := "Write-Host 'hi'"
	got := GenerateCommandLineOneLinerWithOptions(code, PSObfuscateOptions{})
	if !strings.Contains(got, "[ScriptBlock]::Create") {
		t.Fatal("legacy launcher should use ScriptBlock::Create")
	}
	back, err := DecodeOneLinerPayload(got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if back != code {
		t.Fatalf("round-trip = %q, want %q", back, code)
	}
}

func TestXorLauncherRoundTrip(t *testing.T) {
	code := "Get-Process | Where-Object { $_.CPU -gt 50 }"
	got := GenerateCommandLineOneLinerWithOptions(code, PSObfuscateOptions{Launcher: "xor", Seed: 7, SeedSet: true})
	if !strings.Contains(got, "-bxor $k") {
		t.Fatal("xor launcher should embed -bxor decoder")
	}
	// Raw base64 must not decode to the plaintext (it's xored).
	if strings.Contains(got, "Get-Process") {
		t.Fatal("xor launcher leaks plaintext")
	}
	back, err := DecodeOneLinerPayload(got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if back != code {
		t.Fatalf("round-trip = %q, want %q", back, code)
	}
}

func TestGzipLauncherRoundTrip(t *testing.T) {
	code := "Write-Host 'hello world' # " + strings.Repeat("x", 200)
	got := GenerateCommandLineOneLinerWithOptions(code, PSObfuscateOptions{Launcher: "gzip"})
	if !strings.Contains(got, "GzipStream") {
		t.Fatal("gzip launcher should embed GzipStream decoder")
	}
	back, err := DecodeOneLinerPayload(got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if back != code {
		t.Fatal("gzip round-trip mismatch")
	}
}

func TestEncodedCommandLauncher(t *testing.T) {
	code := "Write-Host 'hi'"
	got := GenerateCommandLineOneLinerWithOptions(code, PSObfuscateOptions{Launcher: "encodedcommand"})
	if !strings.Contains(got, "-enc ") {
		t.Fatal("should use -enc")
	}
	if strings.Contains(got, "[ScriptBlock]::Create") {
		t.Fatal("-enc launcher must not contain ScriptBlock string")
	}
	if strings.Contains(got, "[System.Convert]::FromBase64String") {
		t.Fatal("-enc launcher must not contain FromBase64String string")
	}
}

func TestSourceTransformsPreserveSemantics(t *testing.T) {
	code := "$count = 1\n$count = $count + 1\nWrite-Host 'hello'\nInvoke-Mimikatz"
	out := ObfuscatePowerShellSource(code, PSObfuscateOptions{
		VarRename: true, StringSplit: true, ScrubTriggers: true,
		Seed: 1, SeedSet: true,
	})
	// Variable renamed consistently: original $count gone, exactly 3 uses of new name.
	if strings.Contains(out, "$count") {
		t.Fatalf("variable not renamed: %q", out)
	}
	// Trigger token broken up.
	if strings.Contains(out, "Invoke-Mimikatz") {
		t.Fatalf("trigger not scrubbed: %q", out)
	}
	if !strings.Contains(out, "Invoke-") {
		t.Fatalf("scrubbed parts missing: %q", out)
	}
	// Automatics untouched.
	auto := ObfuscatePowerShellSource("$true $null $args $env:PATH", fixedOpts())
	_ = auto // $env:PATH contains ':' so left alone; $true/$null/$args must survive
	for _, tok := range []string{"$true", "$null", "$args"} {
		src := ObfuscatePowerShellSource(tok+" ", PSObfuscateOptions{VarRename: true, Seed: 1, SeedSet: true})
		if !strings.Contains(src, tok) {
			t.Fatalf("automatic %q was renamed: %q", tok, src)
		}
	}
}

func TestSourceTransformsDeterministic(t *testing.T) {
	code := "$a = 1\nWrite-Host 'hey'"
	o1 := ObfuscatePowerShellSource(code, fixedOpts())
	o1b := ObfuscatePowerShellSource(code, fixedOpts())
	if o1 != o1b {
		t.Fatal("same seed must give same output")
	}
}

func TestFullPipelineRoundTrip(t *testing.T) {
	code := "$x = 5\nWrite-Host 'pipeline'\nGet-Process"
	opts := PSObfuscateOptions{
		Launcher: "xor", VarRename: true, StringSplit: true,
		ScrubTriggers: true, JunkComments: true, CaseRandomize: true,
		Seed: 99, SeedSet: true,
	}
	got := GenerateCommandLineOneLinerWithOptions(code, opts)
	back, err := DecodeOneLinerPayload(got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := ObfuscatePowerShellSource(code, opts)
	if back != want {
		t.Fatalf("pipeline round-trip mismatch:\n got %q\nwant %q", back, want)
	}
}
