package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
)

// GenerateSigningKey generates a new ed25519 key pair
func GenerateSigningKey() (publicKey, privateKey []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return []byte(pub), []byte(priv), nil
}

// SignData signs data with the given private key. Returns nil if the key is
// not a valid ed25519 private key size (would otherwise panic).
func SignData(data, privateKey []byte) []byte {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil
	}
	return ed25519.Sign(ed25519.PrivateKey(privateKey), data)
}

// VerifySignature verifies a signature against data with the given public key.
// Returns false for invalid key/signature sizes instead of panicking.
func VerifySignature(data, signature, publicKey []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(publicKey), data, signature)
}
