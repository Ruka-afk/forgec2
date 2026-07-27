package payload

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"
)

type CertEntry struct {
	Cert    *x509.Certificate
	Key     *rsa.PrivateKey
	CertPEM []byte
	KeyPEM  []byte
}

var (
	certPool     []CertEntry
	certPoolOnce sync.Once
	certPoolMu   sync.RWMutex
)

func buildFakeCert(org string, years int) (CertEntry, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return CertEntry{}, fmt.Errorf("rsa.GenerateKey: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return CertEntry{}, fmt.Errorf("rand.Int: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{org},
		},
		NotBefore:             time.Now().Add(-365 * 24 * time.Hour),
		NotAfter:              time.Now().Add(time.Duration(years) * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return CertEntry{}, fmt.Errorf("x509.CreateCertificate: %w", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return CertEntry{}, fmt.Errorf("x509.ParseCertificate: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return CertEntry{Cert: cert, Key: key, CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

func initCertPool() {
	orgs := []string{
		"Microsoft Corporation",
		"Google LLC",
		"Amazon Web Services",
		"Apple Inc.",
		"Mozilla Foundation",
		"Adobe Inc.",
		"Intel Corporation",
		"VMware Inc.",
		"Oracle America Inc.",
		"Cisco Systems Inc.",
		"Symantec Corporation",
		"DigiCert Inc.",
	}
	for _, org := range orgs {
		entry, err := buildFakeCert(org, 3)
		if err != nil {
			slog.Error("Failed to build fake cert", "org", org, "err", err)
			continue
		}
		certPool = append(certPool, entry)
	}
}

func GetRandomCert() CertEntry {
	certPoolOnce.Do(initCertPool)
	certPoolMu.RLock()
	defer certPoolMu.RUnlock()
	if len(certPool) == 0 {
		return CertEntry{}
	}
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(certPool))))
	if err != nil {
		return certPool[0]
	}
	return certPool[idx.Int64()]
}

func GetCertByOrg(org string) (CertEntry, error) {
	certPoolOnce.Do(initCertPool)
	certPoolMu.RLock()
	defer certPoolMu.RUnlock()
	for _, c := range certPool {
		if len(c.Cert.Subject.Organization) > 0 && c.Cert.Subject.Organization[0] == org {
			return c, nil
		}
	}
	return CertEntry{}, fmt.Errorf("no cert for org: %s", org)
}

func ListCertOrgs() []string {
	certPoolOnce.Do(initCertPool)
	certPoolMu.RLock()
	defer certPoolMu.RUnlock()
	out := make([]string, 0, len(certPool))
	for _, c := range certPool {
		if len(c.Cert.Subject.Organization) > 0 {
			out = append(out, c.Cert.Subject.Organization[0])
		}
	}
	return out
}
