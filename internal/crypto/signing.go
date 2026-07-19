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

// SignData signs data with the given private key
func SignData(data, privateKey []byte) []byte {
	return ed25519.Sign(ed25519.PrivateKey(privateKey), data)
}

// VerifySignature verifies a signature against data with the given public key
func VerifySignature(data, signature, publicKey []byte) bool {
	return ed25519.Verify(ed25519.PublicKey(publicKey), data, signature)
}
