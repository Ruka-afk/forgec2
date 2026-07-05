package obfuscation

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateCommandLineOneLiner(t *testing.T) {
	code := "Write-Host 'Hello, World!'"
	result := GenerateCommandLineOneLiner(code)

	if !strings.HasPrefix(result, "powershell -nop -w hidden -c (") {
		t.Fatalf("unexpected prefix: %s", result[:40])
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
	result := GenerateCommandLineOneLiner(code)

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
	result := GenerateCommandLineOneLiner("")
	if !strings.Contains(result, "''))") {
		t.Fatal("should handle empty code")
	}
}
