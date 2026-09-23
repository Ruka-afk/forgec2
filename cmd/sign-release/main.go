// Command sign-release creates detached Ed25519 signatures for release
// assets (hot-update trust anchor: crypto.update_signing_key).
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
// Each <file> gains a sibling <file>.sig containing the hex-encoded
// 64-byte Ed25519 signature over the exact file bytes. The server verifies
// "<checksumURL>.sig" against crypto.update_signing_key before hot update.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
)

func main() {
	gen := flag.Bool("gen", false, "generate a fresh signing keypair and print seed+pubkey")
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

	seedHex := os.Getenv("RELEASE_SIGNING_KEY")
	seed, err := hex.DecodeString(seedHex)
	if err != nil || len(seed) != ed25519.SeedSize {
		fmt.Fprintln(os.Stderr, "RELEASE_SIGNING_KEY must be 64 hex chars (32-byte Ed25519 seed)")
		os.Exit(1)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: sign-release [-gen] <file> [...]")
		os.Exit(1)
	}
	for _, path := range flag.Args() {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read "+path+":", err)
			os.Exit(1)
		}
		sig := ed25519.Sign(priv, data)
		if err := os.WriteFile(path+".sig", []byte(hex.EncodeToString(sig)), 0600); err != nil {
			fmt.Fprintln(os.Stderr, "write "+path+".sig:", err)
			os.Exit(1)
		}
		fmt.Println("signed " + path + ".sig")
	}
}
