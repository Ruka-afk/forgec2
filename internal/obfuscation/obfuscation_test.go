package obfuscation

import (
	"encoding/base64"
	"strings"
	"testing"
)

// These cover the zero-value PSObfuscateOptions path, which yields the legacy
// plain-base64 ScriptBlock::Create launcher. They used to call the removed
// GenerateCommandLineOneLiner wrapper; the behaviour now lives in
// GenerateCommandLineOneLinerWithOptions, which is what the generate handlers
// actually invoke via PresetForLevel.

func TestGenerateCommandLineOneLiner(t *testing.T) {
	code := "Write-Host 'Hello, World!'"
	result := GenerateCommandLineOneLinerWithOptions(code, PSObfuscateOptions{})

	// Assert the launcher SHAPE, not the exact interpreter spelling: the
	// generator varies it (powershell/pwsh, mixed case) as part of the
	// obfuscation pipeline, so pinning the prefix is a flaky assertion.
	if !strings.Contains(strings.ToLower(result), "-nop -w hidden -c ([scriptblock]::create(") {
		t.Fatalf("unexpected launcher shape: %s", result)
	}

	if !strings.Contains(result, "[ScriptBlock]::Create") {
		t.Fatal("should use ScriptBlock::Create")
	}

	if !strings.Contains(result, ".Invoke()") {
		t.Fatal("should call .Invoke()")
	}

	idx := strings.Index(result, "'")
	lastQuote := strings.LastIndex(result, "'")
	if idx == -1 || lastQuote == -1 || idx == lastQuote {
		t.Fatal("should contain quoted base64 string")
	}

	b64 := result[idx+1 : lastQuote]
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode error: %v", err)
	}
	if string(decoded) != code {
		t.Fatalf("decoded = %q, want %q", string(decoded), code)
	}
}

func TestGenerateCommandLineOneLinerSpecialChars(t *testing.T) {
	code := "Get-Process | Where-Object { $_.CPU -gt 50 }"
	result := GenerateCommandLineOneLinerWithOptions(code, PSObfuscateOptions{})

	if !strings.Contains(result, "[ScriptBlock]::Create") {
		t.Fatal("should use ScriptBlock::Create")
	}

	idx := strings.Index(result, "'")
	lastQuote := strings.LastIndex(result, "'")
	b64 := result[idx+1 : lastQuote]
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode error: %v", err)
	}
	if string(decoded) != code {
		t.Fatalf("decoded = %q, want %q", string(decoded), code)
	}
}

func TestGenerateCommandLineOneLinerEmpty(t *testing.T) {
	result := GenerateCommandLineOneLinerWithOptions("", PSObfuscateOptions{})
	if !strings.Contains(result, "''))") {
		t.Fatal("should handle empty code")
	}
}
