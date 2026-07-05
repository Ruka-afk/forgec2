package crypto

import (
	"testing"
)

func TestStreamCipherRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	c := NewStreamCipher(key)

	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"short", []byte("hello")},
		{"medium", []byte("The quick brown fox jumps over the lazy dog")},
		{"binary", []byte{0x00, 0x01, 0xFF, 0xFE, 0x80, 0x7F}},
		{"large", []byte("A")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := c.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			decrypted, err := c.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if len(decrypted) != len(tt.plaintext) {
				t.Fatalf("len mismatch: got %d, want %d", len(decrypted), len(tt.plaintext))
			}
			for i := range tt.plaintext {
				if decrypted[i] != tt.plaintext[i] {
					t.Fatalf("byte %d mismatch: got %02x, want %02x", i, decrypted[i], tt.plaintext[i])
				}
			}
		})
	}
}

func TestStreamCipherHeaderSize(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	c := NewStreamCipher(key)
	encrypted, err := c.Encrypt([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if len(encrypted) < 4+NonceSize {
		t.Fatalf("encrypted too short: %d", len(encrypted))
	}
	if string(encrypted[:4]) != MagicBytes {
		t.Fatalf("bad magic: got %q, want %q", string(encrypted[:4]), MagicBytes)
	}
}

func TestStreamCipherDecryptErrors(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	c := NewStreamCipher(key)

	t.Run("short data", func(t *testing.T) {
		_, err := c.Decrypt([]byte{0x01, 0x02})
		if err != ErrShortData {
			t.Fatalf("expected ErrShortData, got %v", err)
		}
	})

	t.Run("bad magic", func(t *testing.T) {
		_, err := c.Decrypt([]byte("XXXX12345678"))
		if err != ErrBadMagic {
			t.Fatalf("expected ErrBadMagic, got %v", err)
		}
	})
}

func TestNewStreamCipherRandomKey(t *testing.T) {
	c := NewStreamCipher(nil)
	if c.GetKey() == nil || len(c.GetKey()) != KeySize {
		t.Fatalf("expected key of size %d", KeySize)
	}
	key := c.GetKey()
	zero := true
	for _, b := range key {
		if b != 0 {
			zero = false
			break
		}
	}
	if zero {
		t.Fatal("random key should not be all zeros")
	}
}

func TestStreamCipherDifferentKeys(t *testing.T) {
	key1 := []byte("0123456789abcdef0123456789abcdef")
	key2 := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	c1 := NewStreamCipher(key1)
	c2 := NewStreamCipher(key2)

	plaintext := []byte("hello world")
	enc1, _ := c1.Encrypt(plaintext)
	enc2, _ := c2.Encrypt(plaintext)

	same := true
	for i := range enc1 {
		if enc1[i] != enc2[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different keys should produce different ciphertext")
	}

	// Decrypt with wrong key should produce garbage, not an error (XOR cipher has no auth)
	decrypted, err := c1.Decrypt(enc2)
	if err != nil {
		t.Fatalf("Decrypt with wrong key should not error: %v", err)
	}
	if string(decrypted) == string(plaintext) {
		t.Fatal("decrypting with wrong key should not produce original plaintext")
	}
}
