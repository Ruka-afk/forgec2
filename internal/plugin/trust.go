package plugin

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Package trust (manifest schema v2).
//
// A plugin is executable code that runs on the team server, so "it is on disk"
// is not evidence of provenance. The manifest may therefore carry:
//
//	digest:    "sha256:<hex>"   canonical digest over every package file
//	signature: "<hex>"         Ed25519 signature over the digest string
//	publisher: "acme-labs"     informational label
//
// The digest covers the entry point and every helper file, so editing a
// library inside a signed package invalidates it. The signature covers the
// digest, which keeps the signing input tiny and stable across file moves.

const (
	// maxPluginPackageFiles bounds how many files a package may contribute to
	// the digest, so a pathological directory cannot make loading expensive.
	maxPluginPackageFiles = 512
	// maxPluginPackageFileBytes bounds one digested file.
	maxPluginPackageFileBytes = 16 << 20 // 16 MiB
)

// trustPolicy is the server-side plugin trust configuration.
type trustPolicy struct {
	// TrustedKeys are Ed25519 public keys (hex) accepted for package
	// signatures. Empty means "no signatures can be verified".
	TrustedKeys []ed25519.PublicKey
	// RequireSigned refuses unsigned packages. Off by default because the 52
	// bundled plugins ship unsigned; turn it on once a key is configured.
	RequireSigned bool
}

// TrustStatus describes the outcome of verifying a package.
type TrustStatus struct {
	Verified bool   // digest matched and (if present) signature verified
	Signed   bool   // manifest carried a signature
	Unsigned bool   // no digest present
	Detail   string // human-readable reason, surfaced in logs/UI
}

// SetTrust configures package verification. Called once at server startup.
func (m *Manager) SetTrust(trustedKeys []ed25519.PublicKey, requireSigned bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trust = trustPolicy{TrustedKeys: trustedKeys, RequireSigned: requireSigned}
} // trustPolicySnapshot copies the policy for use outside the lock.
func (m *Manager) trustPolicySnapshot() trustPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.trust
}

// VerificationPolicy builds a verification policy for one-off checks (cmd/
// sign-plugin, tests). The returned value can be passed straight to
// VerifyPackageWithPolicy.
func VerificationPolicy(trustedKeys []ed25519.PublicKey, requireSigned bool) trustPolicy {
	return trustPolicy{TrustedKeys: trustedKeys, RequireSigned: requireSigned}
}

// PackageDigest computes the canonical digest of a plugin package. It covers
// every regular file in dir plus the shared paths listed in includes (resolved
// against root, the directory the loader scanned). Build artifacts and
// interpreter caches are excluded: they are produced at runtime and would
// otherwise make a package "fail verification" the first time it runs.
//
// Output form: "sha256:<hex>".
func PackageDigest(dir string, includes []string, root string) (string, error) {
	entries := map[string]string{} // canonical path -> hex sha256
	collect := func(base string) error {
		return filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if digestExcludedDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() || digestExcludedFile(d.Name()) {
				return nil
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			if len(entries) >= maxPluginPackageFiles {
				return fmt.Errorf("plugin package exceeds %d files", maxPluginPackageFiles)
			}
			sum, herr := fileSHA256(path)
			if herr != nil {
				return herr
			}
			entries[rel] = sum
			return nil
		})
	}
	if err := collect(dir); err != nil {
		return "", err
	}
	for _, inc := range includes {
		inc = strings.TrimSpace(inc)
		if inc == "" {
			continue
		}
		inc = filepath.Clean(filepath.FromSlash(inc))
		if filepath.IsAbs(inc) || strings.HasPrefix(inc, "..") {
			return "", fmt.Errorf("includes entry %q must stay inside the package tree", inc)
		}
		if err := collect(filepath.Join(root, inc)); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("includes path %q does not exist", inc)
			}
			return "", err
		}
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("plugin package has no payload files")
	}
	paths := make([]string, 0, len(entries))
	for p := range entries {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		fmt.Fprintf(h, "%s:%s\n", p, entries[p])
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// digestExcludedDir reports directories that are runtime or VCS noise rather
// than package payload.
func digestExcludedDir(name string) bool {
	switch name {
	case "__pycache__", ".git", ".svn", ".hg", "node_modules", ".idea", ".vscode":
		return true
	}
	return false
}

// digestExcludedFile reports files that must not influence verification: the
// manifest itself (it carries the digest), signature sidecars, interpreter
// byte-code and editor droppings.
func digestExcludedFile(name string) bool {
	switch name {
	case "manifest.yaml", "manifest.yml", ".DS_Store", "Thumbs.db":
		return true
	}
	if strings.HasSuffix(name, ".sig") || strings.HasSuffix(name, ".pyc") ||
		strings.HasSuffix(name, ".pyo") || strings.HasSuffix(name, ".swp") ||
		strings.HasSuffix(name, "~") {
		return true
	}
	return false
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.Size() > maxPluginPackageFileBytes {
		return "", fmt.Errorf("plugin file %s exceeds %d bytes", filepath.Base(path), maxPluginPackageFileBytes)
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, maxPluginPackageFileBytes+1)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SignPackageDigest signs a digest string with an Ed25519 private key,
// returning the hex-encoded signature. Used by cmd/sign-plugin.
func SignPackageDigest(priv ed25519.PrivateKey, digest string) string {
	return hex.EncodeToString(ed25519.Sign(priv, []byte(digest)))
}

// VerifyPackage checks a package on disk against its manifest. root is the
// directory the loader scanned, which anchors any shared paths in
// manifest.Includes. A digest mismatch is always fatal (the package changed
// after it was signed); an absent digest is fatal only in RequireSigned mode.
func (m *Manager) VerifyPackage(dir, root string, manifest *Manifest) error {
	status, err := VerifyPackageWithPolicy(dir, root, manifest, m.trustPolicySnapshot())
	if err != nil {
		return err
	}
	switch {
	case status.Verified:
		slog.Info("Plugin package verified", "plugin", manifest.Name, "version", manifest.Version,
			"digest", manifest.Digest, "signed", status.Signed)
	case status.Unsigned:
		slog.Warn("Plugin package is unsigned", "plugin", manifest.Name, "dir", dir,
			"hint", "run cmd/sign-plugin or set plugins.trusted_keys + plugins.require_signed")
	default:
		slog.Warn("Plugin package signature not verified", "plugin", manifest.Name, "reason", status.Detail)
	}
	return nil
}

// VerifyPackageWithPolicy is the pure verification step (no logging, no
// manager state), so callers and tests can apply their own policy.
func VerifyPackageWithPolicy(dir, root string, manifest *Manifest, policy trustPolicy) (TrustStatus, error) {
	if root == "" {
		root = dir
	}
	digest := strings.TrimSpace(manifest.Digest)
	if digest == "" {
		if policy.RequireSigned {
			return TrustStatus{Unsigned: true, Detail: "package has no digest and plugins.require_signed is set"},
				fmt.Errorf("plugin %q is unsigned and plugins.require_signed is enabled", manifest.Name)
		}
		return TrustStatus{Unsigned: true, Detail: "no digest in manifest"}, nil
	}
	actual, err := PackageDigest(dir, manifest.Includes, root)
	if err != nil {
		return TrustStatus{}, fmt.Errorf("plugin %q: cannot compute package digest: %w", manifest.Name, err)
	}
	if !strings.EqualFold(actual, digest) {
		return TrustStatus{Detail: "digest mismatch"}, fmt.Errorf("plugin %q package digest mismatch (manifest %s, on disk %s)", manifest.Name, digest, actual)
	}

	sigHex := strings.TrimSpace(manifest.Signature)
	if sigHex == "" {
		if policy.RequireSigned {
			return TrustStatus{Detail: "digest present but unsigned"}, fmt.Errorf("plugin %q has a digest but no signature and plugins.require_signed is enabled", manifest.Name)
		}
		// Digest matches the manifest, so the operator can pin it out of band.
		return TrustStatus{Detail: "digest ok, unsigned"}, nil
	}
	if len(policy.TrustedKeys) == 0 {
		return TrustStatus{Detail: "signature present but no trusted keys configured"},
			fmt.Errorf("plugin %q is signed but plugins.trusted_keys is empty; configure a key to verify it", manifest.Name)
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return TrustStatus{Detail: "malformed signature"}, fmt.Errorf("plugin %q signature is not %d hex-encoded bytes", manifest.Name, ed25519.SignatureSize)
	}
	for _, pub := range policy.TrustedKeys {
		if ed25519.Verify(pub, []byte(digest), sig) {
			return TrustStatus{Verified: true, Signed: true, Detail: "signature verified"}, nil
		}
	}
	return TrustStatus{Signed: true, Detail: "signature does not match any trusted key"},
		fmt.Errorf("plugin %q signature is not valid for any configured plugins.trusted_keys entry", manifest.Name)
}

// ParseTrustedPluginKeys decodes hex Ed25519 public keys from config.
func ParseTrustedPluginKeys(hexKeys []string) ([]ed25519.PublicKey, error) {
	var out []ed25519.PublicKey
	for _, k := range hexKeys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		raw, err := hex.DecodeString(k)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("plugins.trusted_keys entry must be %d hex chars (32-byte Ed25519 public key)", ed25519.PublicKeySize*2)
		}
		out = append(out, ed25519.PublicKey(raw))
	}
	return out, nil
}
