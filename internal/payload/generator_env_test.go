package payload

import (
	"os"
	"strings"
	"testing"
)

func TestGoModuleEnvInjectsConfiguredProxyWhenUnset(t *testing.T) {
	t.Setenv("GOPROXY", "")
	SetConfiguredGoProxy("https://example.invalid,direct")
	t.Cleanup(func() { SetConfiguredGoProxy("") })

	env := goModuleEnv("CGO_ENABLED=0")
	found := ""
	count := 0
	for _, e := range env {
		if strings.HasPrefix(e, "GOPROXY=") {
			found = e
			count++
		}
	}
	if found != "GOPROXY=https://example.invalid,direct" {
		t.Fatalf("expected configured GOPROXY, got %q (all=%v)", found, env)
	}
	if count != 1 {
		t.Fatalf("expected a single GOPROXY entry, got %d", count)
	}
	foundCGO := false
	for _, e := range env {
		if e == "CGO_ENABLED=0" {
			foundCGO = true
		}
	}
	if !foundCGO {
		t.Fatal("expected extra env to be appended")
	}
}

func TestGoModuleEnvDoesNotOverrideExistingGOPROXY(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	SetConfiguredGoProxy("https://example.invalid,direct")
	t.Cleanup(func() { SetConfiguredGoProxy("") })

	env := goModuleEnv()
	var proxies []string
	for _, e := range env {
		if strings.HasPrefix(e, "GOPROXY=") {
			proxies = append(proxies, e)
		}
	}
	// Process env keeps GOPROXY=off; we must not append a second value.
	if os.Getenv("GOPROXY") != "off" {
		t.Fatalf("test env GOPROXY=%q", os.Getenv("GOPROXY"))
	}
	for _, p := range proxies {
		if p == "GOPROXY=https://example.invalid,direct" {
			t.Fatalf("configured proxy overrode GOPROXY env: %v", proxies)
		}
	}
}
