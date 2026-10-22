// Command sign-release creates — and verifies — detached Ed25519 signatures
// for release assets (hot-update trust anchor: crypto.update_signing_key).
//
// Usage:
//
//	# one-time key generation (do this OFFLINE, store the seed in GitHub
//	# Secrets as RELEASE_SIGNING_KEY, publish the pubkey to operators):
//	go run ./cmd/sign-release -gen
//
//	# sign release checksum files (called by .github/workflows/release.yml):
//	RELEASE_SIGNING_KEY=<64-hex-seed> go run ./cmd/sign-release <file> [...]
//
//	# verify a downloaded checksum file as an operator (no secret needed):
//	go run ./cmd/sign-release -verify -key <64-hex-pubkey> <file> [...]
//
// Each <file> gains a sibling <file>.sig containing the hex-encoded 64-byte
// Ed25519 signature over the exact file bytes. The server verifies
// "<checksumURL>.sig" against crypto.update_signing_key before hot update.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	gen := flag.Bool("gen", false, "generate a fresh signing keypair and print seed+pubkey")
	verify := flag.Bool("verify", false, "verify <file>.sig instead of signing")
	pubKeyHex := flag.String("key", "", "64-hex Ed25519 public key (required with -verify)")
	flag.Parse()

	if *gen {
		pub, seed, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "generate:", err)
			os.Exit(1)
		}
		fmt.Println("RELEASE_SIGNING_KEY=" + hex.EncodeToString(seed.Seed()))
		fmt.Println("update_signing_key=" + hex.EncodeToString(pub))
		return
	}

	if *verify {
		if flag.NArg() == 0 {
			fmt.Fprintln(os.Stderr, "usage: sign-release -verify -key <64-hex-pubkey> <file> [...]")
			os.Exit(2)
		}
		pub, err := parsePublicKey(*pubKeyHex)
		if err != nil {
			fmt.Fprintln(os.Stderr, "verify:", err)
			os.Exit(2)
		}
		failed := false
		for _, path := range flag.Args() {
			if err := verifyFile(pub, path); err != nil {
				fmt.Println("FAIL " + err.Error())
				failed = true
				continue
			}
			fmt.Println("OK   " + path)
		}
		if failed {
			os.Exit(1)
		}
		return
	}

	seedHex := os.Getenv("RELEASE_SIGNING_KEY")
	seed, err := hex.DecodeString(seedHex)
	if err != nil || len(seed) != ed25519.SeedSize {
		fmt.Fprintln(os.Stderr, "RELEASE_SIGNING_KEY must be 64 hex chars (32-byte Ed25519 seed)")
		os.Exit(1)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: sign-release [-gen|-verify] <file> [...]")
		os.Exit(1)
	}
	for _, path := range flag.Args() {
		if err := signFile(priv, path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("signed " + path + ".sig")
	}
}

// parsePublicKey accepts the same 64-character hex form that
// crypto.update_signing_key carries, so an operator can paste it verbatim.
func parsePublicKey(hexKey string) (ed25519.PublicKey, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(hexKey))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("-key must be 64 hex characters (32-byte Ed25519 public key)")
	}
	return ed25519.PublicKey(raw), nil
}

// signFile writes the hex signature of path's exact bytes to path+".sig".
func signFile(priv ed25519.PrivateKey, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	sig := ed25519.Sign(priv, data)
	return os.WriteFile(path+".sig", []byte(hex.EncodeToString(sig)), 0600)
}

// verifyFile checks path against the detached path+".sig". A missing file, a
// malformed signature and a mismatch are all hard errors: half-verified
// releases are worse than rejected ones.
func verifyFile(pub ed25519.PublicKey, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	sigHex, err := os.ReadFile(path + ".sig")
	if err != nil {
		return fmt.Errorf("read %s.sig: %w", path, err)
	}
	sig, err := hex.DecodeString(strings.TrimSpace(string(sigHex)))
	if err != nil {
		return fmt.Errorf("%s.sig is not hex: %w", path, err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("%s.sig is %d bytes, want %d", path, len(sig), ed25519.SignatureSize)
	}
	if !ed25519.Verify(pub, data, sig) {
		return fmt.Errorf("%s: signature does not match", path)
	}
	return nil
}
