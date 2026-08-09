package server

import (
	"encoding/hex"
	"testing"
)

const testCredKeyHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

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

func TestEncryptDecryptSecretRoundtrip(t *testing.T) {
	plaintext := "JBSWY3DPEHPK3PXP"

	encrypted, err := encryptSecret(plaintext, testCredKeyHex)
	if err != nil {
		t.Fatalf("encryptSecret() error: %v", err)
	}
	if encrypted == plaintext {
		t.Error("encrypted should differ from plaintext")
	}
	if encrypted == "" {
		t.Error("encrypted should not be empty")
	}

	decrypted, err := decryptSecret(encrypted, testCredKeyHex)
	if err != nil {
		t.Fatalf("decryptSecret() error: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("roundtrip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptSecret_EmptyString(t *testing.T) {
	got, err := encryptSecret("", testCredKeyHex)
	if err != nil {
		t.Fatalf("encryptSecret(\"\") error: %v", err)
	}
	if got != "" {
		t.Errorf("encryptSecret(\"\") = %q, want empty", got)
	}
}

func TestDecryptSecret_EmptyString(t *testing.T) {
	got, err := decryptSecret("", testCredKeyHex)
	if err != nil {
		t.Fatalf("decryptSecret(\"\") error: %v", err)
	}
	if got != "" {
		t.Errorf("decryptSecret(\"\") = %q, want empty", got)
	}
}

func TestDecryptSecret_WrongKey(t *testing.T) {
	encrypted, err := encryptSecret("sensitive", testCredKeyHex)
	if err != nil {
		t.Fatalf("encryptSecret() error: %v", err)
	}

	_, err = decryptSecret(encrypted, "ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100")
	if err == nil {
		t.Error("decryptSecret with wrong key should fail")
	}
}

func TestDecryptSecret_InvalidBase64(t *testing.T) {
	_, err := decryptSecret("not-valid-base64!!!", testCredKeyHex)
	if err == nil {
		t.Error("decryptSecret with invalid base64 should fail")
	}
}

func TestEncryptSecret_InvalidKey(t *testing.T) {
	if _, err := encryptSecret("data", "too-short"); err == nil {
		t.Error("encryptSecret with non-hex short key should fail")
	}
	if _, err := encryptSecret("data", ""); err == nil {
		t.Error("encryptSecret with empty key should fail")
	}
}

func TestKeyHexTo32(t *testing.T) {
	want, _ := hex.DecodeString(testCredKeyHex)
	got, err := keyHexTo32(testCredKeyHex)
	if err != nil {
		t.Fatalf("keyHexTo32() error: %v", err)
	}
	if string(got) != string(want) {
		t.Error("keyHexTo32 should decode the hex string verbatim")
	}
	if _, err := keyHexTo32("zz"); err == nil {
		t.Error("keyHexTo32 with invalid hex should fail")
	}
	if _, err := keyHexTo32(""); err == nil {
		t.Error("keyHexTo32 with empty string should fail")
	}
}
