// Command sign-plugin stamps a ForgeC2 plugin package with a canonical digest
// and an Ed25519 signature, and can verify an already-stamped package.
//
// Usage:
//
//	# one-time key generation (offline; store the seed as PLUGIN_SIGNING_KEY)
//	go run ./cmd/sign-plugin -gen
//
//	# stamp a package directory in place (writes manifest.digest/signature fields)
//	PLUGIN_SIGNING_KEY=<64-hex-seed> go run ./cmd/sign-plugin plugins/recon/hostinfo
//
//	# verify a package against a trusted public key
//	go run ./cmd/sign-plugin -verify -key <64-hex-pubkey> plugins/recon/hostinfo
//
// The digest covers every package file except the manifest and any .sig
// sidecar, so editing a helper invalidates the signature. The signature is an
// Ed25519 signature over the digest string; the server verifies it against
// plugins.trusted_keys when the package is loaded or installed.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/forgec2/forgec2/internal/plugin"
)

func main() {
	gen := flag.Bool("gen", false, "generate a plugin signing keypair and print seed+pubkey")
	verify := flag.Bool("verify", false, "verify a stamped package instead of signing it")
	keyHex := flag.String("key", "", "trusted Ed25519 public key (hex) for -verify")
	flag.Parse()

	if *gen {
		pub, seed, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fail("generate: %v", err)
		}
		fmt.Println("PLUGIN_SIGNING_KEY=" + hex.EncodeToString(seed.Seed()))
		fmt.Println("plugins.trusted_keys=[\"" + hex.EncodeToString(pub) + "\"]")
		return
	}

	if flag.NArg() == 0 {
		fail("usage: sign-plugin [-gen] [-verify -key <pubkey-hex>] <plugin-dir> [...]")
	}

	if *verify {
		trusted, err := plugin.ParseTrustedPluginKeys([]string{*keyHex})
		if err != nil {
			fail("key: %v", err)
		}
		if len(trusted) == 0 {
			fail("-verify requires -key <64-hex-pubkey>")
		}
		policy := plugin.VerificationPolicy(trusted, false)
		for _, dir := range flag.Args() {
			status, err := plugin.VerifyPackageWithPolicy(dir, mustManifest(dir), policy)
			switch {
			case err != nil:
				fmt.Printf("FAIL %s: %v\n", dir, err)
				os.Exit(1)
			case status.Verified:
				fmt.Printf("OK   %s: signature verified (%s)\n", dir, status.Detail)
			default:
				fmt.Printf("WARN %s: %s\n", dir, status.Detail)
			}
		}
		return
	}

	seedHex := os.Getenv("PLUGIN_SIGNING_KEY")
	seed, err := hex.DecodeString(seedHex)
	if err != nil || len(seed) != ed25519.SeedSize {
		fail("PLUGIN_SIGNING_KEY must be 64 hex chars (32-byte Ed25519 seed); run with -gen first")
	}
	priv := ed25519.NewKeyFromSeed(seed)

	for _, dir := range flag.Args() {
		manifest := mustManifest(dir)
		digest, err := plugin.PackageDigest(dir)
		if err != nil {
			fail("%s: %v", dir, err)
		}
		manifest.Digest = digest
		manifest.Signature = plugin.SignPackageDigest(priv, digest)
		if err := manifest.Save(dir); err != nil {
			fail("%s: %v", dir, err)
		}
		fmt.Printf("signed %s (digest %s)\n", dir, digest)
	}
}

func mustManifest(dir string) *plugin.Manifest {
	path := filepath.Join(dir, "manifest.yaml")
	manifest, err := plugin.LoadManifest(path)
	if err != nil {
		fail("%v", err)
	}
	return manifest
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
