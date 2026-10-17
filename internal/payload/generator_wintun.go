package payload

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// agentAssetsRoot is the non-Go agent subtree embedded in the server: the
// optional Wintun runtimes staged by scripts/fetch-wintun.{ps1,sh}. It is
// absent in checkouts that never fetched them, and everything below tolerates
// that: generation simply keeps the honest no-Wintun stub.
const agentAssetsRoot = "agent/assets"

// extractAgentAssets copies the embedded agent assets subtree (when present)
// into the temp build directory, preserving the relative layout, so the
// per-arch //go:embed directives in the agent's tun_wintun_*.go files can pick
// up the DLL for the requested target architecture.
func extractAgentAssets(efs fs.FS, dir string) error {
	if _, err := fs.Stat(efs, agentAssetsRoot); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return fs.WalkDir(efs, agentAssetsRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Paths are relative to the agent package root in the build tree
		// (the top-level .go files land there flat), so keep the "assets/"
		// component: agent/assets/... -> <dir>/assets/...
		rel := strings.TrimPrefix(p, "agent/")
		if rel == "" {
			return nil
		}
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		data, err := fs.ReadFile(efs, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
}

// wintunBuildTags returns the build tags for an agent compile: "wintun" only
// when building for a Windows GOARCH whose staged DLL is present in the build
// tree. Without the asset the tag is omitted, the tagged sources are excluded
// and tun_start reports the honest "no bundled Wintun" error, so generation
// never breaks for operators who have not fetched the runtime.
func wintunBuildTags(goos, goarch, workDir string) []string {
	if goos != "windows" {
		return nil
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return nil
	}
	dll := filepath.Join(workDir, "assets", "wintun", goarch, "wintun.dll")
	if info, err := os.Stat(dll); err != nil || info.IsDir() {
		return nil
	}
	return []string{"wintun"}
}
