package server

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Release asset naming.
//
// The release pipeline publishes one raw server binary per platform named
//
//	forgec2-server-<os>-<arch>[.exe]
//
// plus a "<binary>.sha256" checksum, a "<binary>.sha256.sig" detached
// signature, an SBOM and the one-click bundles. Only the raw binary may be
// installed over the running server, so the selector below rejects every
// sidecar and every bundle instead of pattern-matching loosely: a wrong pick
// means either an unbootable binary (wrong architecture) or a checksum that
// belongs to a different file.
//
// The previous implementation hard-coded ".exe" and matched any asset whose
// name contained "windows", "amd64" or "x64". That picked the wrong
// architecture on arm64 and found nothing at all on Linux, and the checksum
// was taken from whichever ".sha256" happened to be listed first.

// serverBinaryPrefix is the asset name prefix of the raw server binary.
const serverBinaryPrefix = "forgec2-server-"

// nonBinarySuffixes are the sidecar/bundle suffixes that must never be
// mistaken for the server binary.
var nonBinarySuffixes = []string{
	".sha256", ".sha512", ".sha1", ".md5", ".sig", ".asc",
	".json", ".xml", ".sbom", ".txt", ".md", ".csv",
	".zip", ".gz", ".tgz", ".tar", ".bz2", ".xz", ".7z",
	".ps1", ".sh", ".bat", ".cmd", ".exe.manifest",
}

// aliasPattern matches one platform token inside an asset name. The
// boundary keeps "arm" from matching inside "arm64" and keeps the
// architecture families apart.
type aliasPattern struct {
	canonical    string
	alternatives []string
	re           *regexp.Regexp
}

// osAliasPatterns map the GOOS of the running server to the tokens used in
// release asset names.
var osAliasPatterns = []aliasPattern{
	{canonical: "linux", alternatives: []string{"linux"}},
	{canonical: "windows", alternatives: []string{"windows", "win"}},
	{canonical: "darwin", alternatives: []string{"darwin", "macos", "osx"}},
	{canonical: "freebsd", alternatives: []string{"freebsd"}},
	{canonical: "netbsd", alternatives: []string{"netbsd"}},
	{canonical: "openbsd", alternatives: []string{"openbsd"}},
}

// archAliasPatterns map the GOARCH of the running server to the tokens used
// in release asset names. "x86" is deliberately absent from the 386 family:
// it also occurs inside "x86_64", which belongs to amd64.
var archAliasPatterns = []aliasPattern{
	{canonical: "amd64", alternatives: []string{"amd64", "x86_64", "x86-64", "x64"}},
	{canonical: "arm64", alternatives: []string{"arm64", "aarch64"}},
	{canonical: "386", alternatives: []string{"i386", "386"}},
	{canonical: "arm", alternatives: []string{"armv7", "armhf", "arm"}},
}

func init() {
	for i := range osAliasPatterns {
		osAliasPatterns[i].re = compileAliasPattern(osAliasPatterns[i].alternatives)
	}
	for i := range archAliasPatterns {
		archAliasPatterns[i].re = compileAliasPattern(archAliasPatterns[i].alternatives)
	}
}

// compileAliasPattern builds a boundary-anchored regex for the alternatives,
// longest first so the longest spelling wins ("x86_64" over "x86").
func compileAliasPattern(alternatives []string) *regexp.Regexp {
	sorted := make([]string, len(alternatives))
	copy(sorted, alternatives)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	quoted := make([]string, 0, len(sorted))
	for _, alt := range sorted {
		quoted = append(quoted, regexp.QuoteMeta(alt))
	}
	return regexp.MustCompile(`(?i)(^|[-_.])(` + strings.Join(quoted, "|") + `)([-_.]|$)`)
}

// matchAlias returns the canonical name of the longest alternative that
// occurs in the given asset name.
func matchAlias(name string, patterns []aliasPattern) (string, bool) {
	best := ""
	bestLen := 0
	for _, p := range patterns {
		m := p.re.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		if len(m[2]) > bestLen {
			bestLen = len(m[2])
			best = p.canonical
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

// isRawBinaryName reports whether the asset name can be the server binary.
// Everything with a sidecar or bundle suffix is excluded.
func isRawBinaryName(lower string) bool {
	for _, suffix := range nonBinarySuffixes {
		if strings.HasSuffix(lower, suffix) {
			return false
		}
	}
	return true
}

// canonicalBinaryName is the asset name the release pipeline publishes for
// the given platform.
func canonicalBinaryName(goos, goarch string) string {
	name := serverBinaryPrefix + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// matchAssetPlatform returns the platform a raw binary asset was built for.
// ok is false for anything that is not a raw server binary, or whose name
// does not name an operating system and an architecture.
func matchAssetPlatform(name string) (goos, goarch string, ok bool) {
	lower := strings.ToLower(name)
	if !isRawBinaryName(lower) {
		return "", "", false
	}
	if !strings.Contains(lower, "server") {
		return "", "", false
	}
	osName, osOK := matchAlias(lower, osAliasPatterns)
	archName, archOK := matchAlias(lower, archAliasPatterns)
	if !osOK || !archOK {
		return "", "", false
	}
	return osName, archName, true
}

// selectBinaryAsset picks the release asset that matches the running
// platform. The canonical name wins outright; otherwise the asset must name
// exactly this operating system and architecture. An ambiguous release is an
// error: guessing wrong installs a binary that cannot run.
func selectBinaryAsset(assets []GitHubAsset, goos, goarch string) (GitHubAsset, error) {
	canonical := canonicalBinaryName(goos, goarch)
	for _, a := range assets {
		if strings.EqualFold(a.Name, canonical) {
			return a, nil
		}
	}

	targetOS, osOK := matchAlias(goos, osAliasPatterns)
	targetArch, archOK := matchAlias(goarch, archAliasPatterns)
	if !osOK || !archOK {
		return GitHubAsset{}, fmt.Errorf("unsupported platform %s/%s", goos, goarch)
	}

	var matches []GitHubAsset
	for _, a := range assets {
		assetOS, assetArch, ok := matchAssetPlatform(a.Name)
		if !ok {
			continue
		}
		if assetOS == targetOS && assetArch == targetArch {
			matches = append(matches, a)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return GitHubAsset{}, fmt.Errorf("release has no %s/%s server binary (assets: %s)",
			goos, goarch, strings.Join(assetNames(assets), ", "))
	default:
		return GitHubAsset{}, fmt.Errorf("release has %d %s/%s server binaries (%s); refusing to guess",
			len(matches), goos, goarch, strings.Join(assetNames(matches), ", "))
	}
}

// findAssetByName returns the asset with the given name (case-insensitive).
func findAssetByName(assets []GitHubAsset, name string) (GitHubAsset, bool) {
	for _, a := range assets {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return GitHubAsset{}, false
}

// assetNames lists asset names for error messages.
func assetNames(assets []GitHubAsset) []string {
	names := make([]string, 0, len(assets))
	for _, a := range assets {
		names = append(names, a.Name)
	}
	return names
}
