package server

import (
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal", "report.pdf", "report.pdf"},
		{"spaces", "my file.txt", "my_file.txt"},
		{"path traversal", "../../../etc/passwd", "passwd"},
		{"dangerous chars", "file\r\nContent-Type: evil", "file__Content-Type__evil"},
		{"empty", "", "download"},
		{"only special", "!@#$%^&*()", "__________"},
		{"mixed", "agent<1>.bin", "agent_1_.bin"},
		{"unicode stripped", "résumé.pdf", "r_sum_.pdf"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeFilename(tc.input)
			if got != tc.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestCSRFTokenEncryptDecryptRoundtrip(t *testing.T) {
	secret := "test-jwt-secret-key-12345"
	plaintext := "JBSWY3DPEHPK3PXP"

	encrypted, err := encryptSecret(plaintext, secret)
	if err != nil {
		t.Fatalf("encryptSecret() error: %v", err)
	}
	if encrypted == plaintext {
		t.Error("encrypted should differ from plaintext")
	}
	if encrypted == "" {
		t.Error("encrypted should not be empty")
	}

	decrypted, err := decryptSecret(encrypted, secret)
	if err != nil {
		t.Fatalf("decryptSecret() error: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("roundtrip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptSecret_EmptyString(t *testing.T) {
	got, err := encryptSecret("", "key")
	if err != nil {
		t.Fatalf("encryptSecret(\"\") error: %v", err)
	}
	if got != "" {
		t.Errorf("encryptSecret(\"\") = %q, want empty", got)
	}
}

func TestDecryptSecret_EmptyString(t *testing.T) {
	got, err := decryptSecret("", "key")
	if err != nil {
		t.Fatalf("decryptSecret(\"\") error: %v", err)
	}
	if got != "" {
		t.Errorf("decryptSecret(\"\") = %q, want empty", got)
	}
}

func TestDecryptSecret_WrongKey(t *testing.T) {
	encrypted, err := encryptSecret("sensitive", "correct-key")
	if err != nil {
		t.Fatalf("encryptSecret() error: %v", err)
	}

	_, err = decryptSecret(encrypted, "wrong-key")
	if err == nil {
		t.Error("decryptSecret with wrong key should fail")
	}
}

func TestDecryptSecret_InvalidBase64(t *testing.T) {
	_, err := decryptSecret("not-valid-base64!!!", "key")
	if err == nil {
		t.Error("decryptSecret with invalid base64 should fail")
	}
}

func TestCSRFTokenDeriveDeterministic(t *testing.T) {
	key1 := deriveTOTPKey("my-secret")
	key2 := deriveTOTPKey("my-secret")
	if len(key1) != 32 {
		t.Errorf("key length = %d, want 32", len(key1))
	}
	for i := range key1 {
		if key1[i] != key2[i] {
			t.Fatal("deriveTOTPKey should be deterministic for same input")
		}
	}
}

func TestCSRFTokenDeriveDifferentKeys(t *testing.T) {
	key1 := deriveTOTPKey("secret-a")
	key2 := deriveTOTPKey("secret-b")
	same := true
	for i := range key1 {
		if key1[i] != key2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different secrets should produce different keys")
	}
}
