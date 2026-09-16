package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
)

// TestTLSCertLoaderHotReload proves regenerate-style replacement takes
// effect without a restart: after overwriting the files, GetCertificate
// serves the new pair.
func TestTLSCertLoaderHotReload(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")
	if err := crypto.GenerateSelfSignedCert(certPath, keyPath); err != nil {
		t.Fatalf("generate: %v", err)
	}
	loader := newTLSCertLoader(certPath, keyPath)
	first, err := loader.GetCertificate(nil)
	if err != nil || first == nil {
		t.Fatalf("initial load: %v", err)
	}
	days, err := loader.daysUntilExpiry()
	if err != nil || days <= 0 {
		t.Fatalf("fresh cert must have positive expiry: days=%d err=%v", days, err)
	}

	// Force mtime change and replace the pair with a fresh one.
	time.Sleep(1100 * time.Millisecond)
	if err := crypto.GenerateSelfSignedCert(certPath+"2", keyPath+"2"); err != nil {
		t.Fatalf("generate replacement paths: %v", err)
	}
	// Overwrite via temp files (GenerateSelfSignedCert skips existing).
	mustCopyFile(t, certPath+"2", certPath)
	mustCopyFile(t, keyPath+"2", keyPath)

	second, err := loader.GetCertificate(nil)
	if err != nil || second == nil {
		t.Fatalf("reload: %v", err)
	}
	if string(first.Certificate[0]) == string(second.Certificate[0]) {
		t.Fatal("loader kept serving the stale pair after file replacement")
	}
}

func mustCopyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0600); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}
