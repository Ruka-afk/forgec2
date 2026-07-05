package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "server.crt")
	keyPath := filepath.Join(tmpDir, "server.key")

	err := GenerateSelfSignedCert(certPath, keyPath)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() error = %v", err)
	}

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Fatal("cert file was not created")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Fatal("key file was not created")
	}

	certData, _ := os.ReadFile(certPath)
	if len(certData) == 0 {
		t.Fatal("cert file is empty")
	}
	keyData, _ := os.ReadFile(keyPath)
	if len(keyData) == 0 {
		t.Fatal("key file is empty")
	}
}

func TestGenerateSelfSignedCertSkipExisting(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "server.crt")
	keyPath := filepath.Join(tmpDir, "server.key")

	err := GenerateSelfSignedCert(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}

	// Modify the cert file
	os.WriteFile(certPath, []byte("modified"), 0644)

	// Should NOT regenerate since files exist
	err = GenerateSelfSignedCert(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(certPath)
	if string(data) != "modified" {
		t.Fatal("cert was regenerated when it should have been skipped")
	}
}
