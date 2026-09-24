package plugin

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/forgec2/forgec2/internal/testutil"
)

func writePackage(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func newSigningKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

// TestPackageDigestIsStableAndCoversHelpers proves the digest is deterministic
// and that helper files (not just the entry point) are covered.
func TestPackageDigestIsStableAndCoversHelpers(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, map[string]string{
		"manifest.yaml": "name: demo\nversion: 1.0.0\ntype: report\nentry: main.py\ninterpreter: python3\n",
		"main.py":       "print('{}')\n",
		"lib/helper.py": "VALUE = 1\n",
	})

	first, err := PackageDigest(dir)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	second, err := PackageDigest(dir)
	if err != nil {
		t.Fatalf("digest again: %v", err)
	}
	if first != second {
		t.Fatalf("digest is not deterministic: %s vs %s", first, second)
	}

	// A manifest-only change (adding the digest itself) must not change it.
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"),
		[]byte("name: demo\nversion: 1.0.0\ntype: report\nentry: main.py\ninterpreter: python3\ndigest: "+first+"\n"), 0o600); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}
	third, err := PackageDigest(dir)
	if err != nil {
		t.Fatalf("digest after manifest change: %v", err)
	}
	if third != first {
		t.Fatal("manifest metadata must not affect the package digest")
	}

	// A helper edit must change it.
	if err := os.WriteFile(filepath.Join(dir, "lib/helper.py"), []byte("VALUE = 2\n"), 0o600); err != nil {
		t.Fatalf("edit helper: %v", err)
	}
	changed, err := PackageDigest(dir)
	if err != nil {
		t.Fatalf("digest after edit: %v", err)
	}
	if changed == first {
		t.Fatal("helper file edit did not change the package digest")
	}
}

// TestVerifyPackageSignatureLifecycle covers the full trust matrix: unsigned
// (allowed by default, refused in strict mode), valid signature, wrong key,
// tampered file and malformed signature.
func TestVerifyPackageSignatureLifecycle(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, map[string]string{
		"main.py": "print('{}')\n",
	})
	manifest := &Manifest{Name: "demo", Version: "1.0.0", Type: "report", Entry: "main.py", Interpreter: "python3"}

	pub, priv := newSigningKey(t)
	policy := VerificationPolicy([]ed25519.PublicKey{pub}, true)

	// Unsigned + strict => refused.
	if _, err := VerifyPackageWithPolicy(dir, manifest, policy); err == nil {
		t.Fatal("strict policy accepted an unsigned package")
	}
	// Unsigned + permissive => allowed but flagged.
	status, err := VerifyPackageWithPolicy(dir, manifest, VerificationPolicy(nil, false))
	if err != nil || !status.Unsigned {
		t.Fatalf("unsigned package in permissive mode: status=%+v err=%v", status, err)
	}

	// Stamp it correctly.
	digest, err := PackageDigest(dir)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	manifest.Digest = digest
	manifest.Signature = SignPackageDigest(priv, digest)
	status, err = VerifyPackageWithPolicy(dir, manifest, policy)
	if err != nil || !status.Verified || !status.Signed {
		t.Fatalf("valid signature rejected: status=%+v err=%v", status, err)
	}

	// Signature from an untrusted key.
	_, otherPriv := newSigningKey(t)
	wrongSig := &Manifest{Name: manifest.Name, Digest: digest, Signature: SignPackageDigest(otherPriv, digest)}
	if _, err := VerifyPackageWithPolicy(dir, wrongSig, policy); err == nil {
		t.Fatal("signature from an untrusted key accepted")
	}

	// Digest present but no signature while strict.
	unsignedButStamped := &Manifest{Name: manifest.Name, Digest: digest}
	if _, err := VerifyPackageWithPolicy(dir, unsignedButStamped, policy); err == nil {
		t.Fatal("strict policy accepted a digest-only package")
	}

	// Signature present but server has no trusted keys configured.
	if _, err := VerifyPackageWithPolicy(dir, manifest, VerificationPolicy(nil, false)); err == nil {
		t.Fatal("signed package accepted without trusted keys")
	}

	// Malformed signature.
	bad := &Manifest{Name: manifest.Name, Digest: digest, Signature: "not-hex"}
	if _, err := VerifyPackageWithPolicy(dir, bad, policy); err == nil {
		t.Fatal("malformed signature accepted")
	}

	// Tampered payload invalidates the digest.
	writePackage(t, dir, map[string]string{"main.py": "print('tampered')\n"})
	if _, err := VerifyPackageWithPolicy(dir, manifest, policy); err == nil {
		t.Fatal("tampered package accepted")
	}
}

// TestManagerRegisterRefusesTamperedPackage proves enforcement is wired into
// the registration path, not just the helper.
func TestManagerRegisterRefusesTamperedPackage(t *testing.T) {
	m := NewManager(testutil.SetupTestDB(t))
	pub, priv := newSigningKey(t)
	m.SetTrust([]ed25519.PublicKey{pub}, true)

	dir := filepath.Join(t.TempDir(), "demo")
	writePackage(t, dir, map[string]string{"main.py": "print('{}')\n"})
	digest, err := PackageDigest(dir)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	manifest := &Manifest{
		Name: "demo", Version: "1.0.0", Type: "report", Entry: "main.py",
		Interpreter: "python3", Digest: digest, Signature: SignPackageDigest(priv, digest),
	}
	// Point the manager at the staged directory.
	m.mu.Lock()
	m.pluginDir = filepath.Dir(dir)
	m.mu.Unlock()
	if err := m.registerAtDir(manifest, dir); err != nil {
		t.Fatalf("valid signed package rejected: %v", err)
	}

	// Same manifest, modified payload => registration must fail.
	writePackage(t, dir, map[string]string{"main.py": "print('evil')\n"})
	other := &Manifest{
		Name: "demo2", Version: "1.0.0", Type: "report", Entry: "main.py",
		Interpreter: "python3", Digest: digest, Signature: SignPackageDigest(priv, digest),
	}
	if err := m.registerAtDir(other, dir); err == nil {
		t.Fatal("tampered package registered")
	}
}

// TestParseTrustedPluginKeys proves malformed key material is rejected loudly
// rather than silently disabling verification.
func TestParseTrustedPluginKeys(t *testing.T) {
	pub, _ := newSigningKey(t)
	hexPub := make([]byte, ed25519.PublicKeySize)
	copy(hexPub, pub)
	keys, err := ParseTrustedPluginKeys([]string{"", "  " + hexString(hexPub) + "  "})
	if err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if _, err := ParseTrustedPluginKeys([]string{"zz"}); err == nil {
		t.Fatal("non-hex key accepted")
	}
	if _, err := ParseTrustedPluginKeys([]string{"abcd"}); err == nil {
		t.Fatal("short key accepted")
	}
}

func hexString(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&0x0F])
	}
	return string(out)
}
