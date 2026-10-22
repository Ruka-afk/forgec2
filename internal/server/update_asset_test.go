package server

import (
	"strings"
	"testing"
)

// releaseAssetsFixture is the asset list of a real ForgeC2 release (v2.7.0).
// It carries one raw binary per platform, a checksum and a signature per
// binary, an SBOM and the one-click bundles.
func releaseAssetsFixture() []GitHubAsset {
	names := []string{
		"forgec2-server-linux-amd64",
		"forgec2-server-linux-amd64.sbom.json",
		"forgec2-server-linux-amd64.sha256",
		"forgec2-server-linux-amd64.sha256.sig",
		"forgec2-server-linux-arm64",
		"forgec2-server-linux-arm64.sha256",
		"forgec2-server-linux-arm64.sha256.sig",
		"forgec2-server-windows-amd64.exe",
		"forgec2-server-windows-amd64.exe.sha256",
		"forgec2-server-windows-amd64.exe.sha256.sig",
		"forgec2-server-windows-arm64.exe",
		"forgec2-server-windows-arm64.exe.sha256",
		"forgec2-server-windows-arm64.exe.sha256.sig",
		"forgec2-v2.7.0-linux-amd64.tar.gz",
		"forgec2-v2.7.0-linux-amd64.tar.gz.sha256",
		"forgec2-v2.7.0-windows-amd64.zip",
	}
	assets := make([]GitHubAsset, 0, len(names))
	for _, n := range names {
		assets = append(assets, GitHubAsset{Name: n, BrowserDownloadURL: "https://example.invalid/" + n})
	}
	return assets
}

// TestSelectBinaryAssetPicksTheRunningPlatform proves the selector installs
// the binary that matches the platform it runs on. The previous matcher
// hard-coded ".exe" and matched any name containing "windows", "amd64" or
// "x64": it found nothing on Linux and could pick the wrong architecture.
func TestSelectBinaryAssetPicksTheRunningPlatform(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
	}{
		{"linux", "amd64", "forgec2-server-linux-amd64"},
		{"linux", "arm64", "forgec2-server-linux-arm64"},
		{"windows", "amd64", "forgec2-server-windows-amd64.exe"},
		{"windows", "arm64", "forgec2-server-windows-arm64.exe"},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			got, err := selectBinaryAsset(releaseAssetsFixture(), tc.goos, tc.goarch)
			if err != nil {
				t.Fatalf("selectBinaryAsset: %v", err)
			}
			if got.Name != tc.want {
				t.Fatalf("selected %q, want %q", got.Name, tc.want)
			}
		})
	}
}

// TestSelectBinaryAssetNeverPicksASidecarOrBundle proves the sidecars
// (checksums, signatures, SBOMs) and the one-click bundles are never
// mistaken for the server binary.
func TestSelectBinaryAssetNeverPicksASidecarOrBundle(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			got, err := selectBinaryAsset(releaseAssetsFixture(), goos, goarch)
			if err != nil {
				t.Fatalf("%s/%s: %v", goos, goarch, err)
			}
			lower := strings.ToLower(got.Name)
			for _, forbidden := range []string{".sha256", ".sig", ".json", ".tar.gz", ".zip"} {
				if strings.HasSuffix(lower, forbidden) {
					t.Fatalf("%s/%s selected the sidecar %q", goos, goarch, got.Name)
				}
			}
		}
	}
}

// TestSelectBinaryAssetRefusesWhenThePlatformIsMissing proves the selector
// fails loudly instead of installing a binary for another platform.
func TestSelectBinaryAssetRefusesWhenThePlatformIsMissing(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "forgec2-server-windows-amd64.exe"},
		{Name: "forgec2-server-windows-amd64.exe.sha256"},
	}
	if _, err := selectBinaryAsset(assets, "linux", "amd64"); err == nil {
		t.Fatal("a Windows-only release was accepted on Linux")
	}
	if _, err := selectBinaryAsset(assets, "windows", "arm64"); err == nil {
		t.Fatal("an amd64-only release was accepted on arm64")
	}
}

// TestSelectBinaryAssetRefusesAnAmbiguousRelease proves the selector refuses
// to guess when two assets match the platform.
func TestSelectBinaryAssetRefusesAnAmbiguousRelease(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "forgec2-server-linux-x86_64"},
		{Name: "forgec2-server-linux-x64"},
	}
	_, err := selectBinaryAsset(assets, "linux", "amd64")
	if err == nil {
		t.Fatal("an ambiguous release was accepted")
	}
	if !strings.Contains(err.Error(), "refusing to guess") {
		t.Fatalf("error %q does not explain the refusal", err)
	}
}

// TestSelectBinaryAssetPrefersTheCanonicalName proves the exact release
// name wins even when another asset would also match the platform.
func TestSelectBinaryAssetPrefersTheCanonicalName(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "forgec2-server-linux-amd64"},
		{Name: "forgec2-server-linux-amd64-v2"},
	}
	got, err := selectBinaryAsset(assets, "linux", "amd64")
	if err != nil {
		t.Fatalf("selectBinaryAsset: %v", err)
	}
	if got.Name != "forgec2-server-linux-amd64" {
		t.Fatalf("selected %q, want the canonical name", got.Name)
	}
}

// TestSelectBinaryAssetUnderstandsArchitectureAliases proves the common
// spellings of an architecture resolve to the same platform, and that "arm"
// never matches inside "arm64".
func TestSelectBinaryAssetUnderstandsArchitectureAliases(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "forgec2-server-linux-x86_64"},
		{Name: "forgec2-server-linux-aarch64"},
		{Name: "forgec2-server-linux-arm"},
	}
	got, err := selectBinaryAsset(assets, "linux", "amd64")
	if err != nil {
		t.Fatalf("amd64: %v", err)
	}
	if got.Name != "forgec2-server-linux-x86_64" {
		t.Fatalf("amd64 selected %q", got.Name)
	}
	got, err = selectBinaryAsset(assets, "linux", "arm64")
	if err != nil {
		t.Fatalf("arm64: %v", err)
	}
	if got.Name != "forgec2-server-linux-aarch64" {
		t.Fatalf("arm64 selected %q", got.Name)
	}
	got, err = selectBinaryAsset(assets, "linux", "arm")
	if err != nil {
		t.Fatalf("arm: %v", err)
	}
	if got.Name != "forgec2-server-linux-arm" {
		t.Fatalf("arm selected %q", got.Name)
	}
}

// TestChecksumAssetBelongsToTheBinary proves the checksum is derived from
// the selected binary. The previous implementation took whichever ".sha256"
// happened to be listed first, which in a multi-platform release is the
// checksum of a different file.
func TestChecksumAssetBelongsToTheBinary(t *testing.T) {
	assets := releaseAssetsFixture()
	binary, err := selectBinaryAsset(assets, "windows", "amd64")
	if err != nil {
		t.Fatalf("selectBinaryAsset: %v", err)
	}
	checksum, ok := findAssetByName(assets, binary.Name+".sha256")
	if !ok {
		t.Fatalf("no checksum for %s", binary.Name)
	}
	if !strings.HasSuffix(checksum.Name, "forgec2-server-windows-amd64.exe.sha256") {
		t.Fatalf("checksum asset = %q", checksum.Name)
	}
	if _, ok := findAssetByName(assets, "forgec2-server-linux-amd64.sha256"); !ok {
		t.Fatal("the linux checksum was not found")
	}
}

// TestMatchAssetPlatformIgnoresBundles proves the platform matcher only
// accepts raw binaries.
func TestMatchAssetPlatformIgnoresBundles(t *testing.T) {
	if _, _, ok := matchAssetPlatform("forgec2-v2.7.0-linux-amd64.tar.gz"); ok {
		t.Fatal("a bundle was accepted as a binary")
	}
	if _, _, ok := matchAssetPlatform("forgec2-server-linux-amd64.sbom.json"); ok {
		t.Fatal("an SBOM was accepted as a binary")
	}
	if _, _, ok := matchAssetPlatform("forgec2-server-linux-amd64.sha256"); ok {
		t.Fatal("a checksum file was accepted as a binary")
	}
	os, arch, ok := matchAssetPlatform("forgec2-server-linux-amd64")
	if !ok || os != "linux" || arch != "amd64" {
		t.Fatalf("platform = %q/%q ok=%v", os, arch, ok)
	}
}
