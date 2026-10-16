package payload

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

// MaxMTLSPEMMaterialBytes bounds one operator-supplied PEM entry (client
// certificate, key or CA). Real entries are a few KB; the cap keeps a paste
// accident or hostile form field from inflating build artifacts.
const MaxMTLSPEMMaterialBytes = 64 * 1024

// ValidateMTLSMaterial checks operator-supplied mTLS client credentials before
// a build embeds them. The certificate and key must be supplied together and
// form a valid PEM pair; the CA bundle is optional. Running the check at the
// API boundary rejects a build that could never complete a handshake up front,
// instead of shipping a binary that silently fails to authenticate (the agent
// only logs a certificate-load failure in debug builds).
func ValidateMTLSMaterial(certPEM, keyPEM, caPEM string) error {
	for _, entry := range []struct{ name, pem string }{
		{"certificate", certPEM},
		{"private key", keyPEM},
		{"CA bundle", caPEM},
	} {
		if len(entry.pem) > MaxMTLSPEMMaterialBytes {
			return fmt.Errorf("mTLS %s exceeds %d bytes", entry.name, MaxMTLSPEMMaterialBytes)
		}
	}
	if (certPEM == "") != (keyPEM == "") {
		return fmt.Errorf("mTLS client certificate and private key must be supplied together")
	}
	if certPEM != "" {
		if _, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM)); err != nil {
			return fmt.Errorf("mTLS certificate/key is not a valid PEM pair")
		}
	}
	if caPEM != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(caPEM)) {
			return fmt.Errorf("mTLS CA bundle contains no PEM certificates")
		}
	}
	return nil
}
