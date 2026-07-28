//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
)

var pinnedCertSHA256 []byte // decoded from PinnedCertSHA256Str in init

func initTLSPinning() {
	if PinnedCertSHA256Str == "" {
		return
	}
	h, err := hex.DecodeString(PinnedCertSHA256Str)
	if err != nil || len(h) != 32 {
		if Debug {
			fmt.Printf("[!] Invalid pinned cert hash (need 64 hex chars): %v\n", err)
		}
		return
	}
	pinnedCertSHA256 = h
	if Debug {
		fmt.Printf("[+] Certificate pinning enabled: %s\n", PinnedCertSHA256Str)
	}
}

func newAgentTLSConfig(serverName string) *tls.Config {
	cfg := &tls.Config{
		InsecureSkipVerify: SkipTLSVerify,
	}
	if serverName != "" {
		cfg.ServerName = serverName
	}
	if len(pinnedCertSHA256) > 0 {
		cfg.VerifyPeerCertificate = verifyPinnedCert
	}
	return cfg
}

func verifyPinnedCert(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	if len(rawCerts) == 0 {
		return fmt.Errorf("no server certificate presented")
	}
	certHash := sha256.Sum256(rawCerts[0])
	if !bytes.Equal(certHash[:], pinnedCertSHA256) {
		return fmt.Errorf("certificate pin mismatch: got %s, want %s",
			hex.EncodeToString(certHash[:]), hex.EncodeToString(pinnedCertSHA256))
	}
	return nil
}
