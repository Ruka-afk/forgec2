package server

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/forgec2/forgec2/internal/payload"
)

// buildSignatureInfo is the small, lock-safe subset of a signature report
// stored on a BuildJob. x509 certificates are not retained by the server.
type buildSignatureInfo struct {
	Status string
	Signer string
	Issuer string
	Error  string
}

// inspectBuildSignature classifies the embedded Authenticode signature of a
// completed direct Windows EXE/DLL build. ZIP/LNK delivery bundles are skipped:
// they are not PE files, and the EXE inside is not what the download endpoint
// serves directly.
func inspectBuildSignature(platform, format, output string) (buildSignatureInfo, bool) {
	if platform != "windows" || (format != "exe" && format != "dll") {
		return buildSignatureInfo{}, false
	}
	switch strings.ToLower(filepath.Ext(output)) {
	case ".exe", ".dll":
	default:
		return buildSignatureInfo{}, false
	}
	data, err := os.ReadFile(output)
	if err != nil {
		slog.Warn("Build signature: cannot read completed artifact", "path", output, "err", err)
		return buildSignatureInfo{}, false
	}
	report := payload.ClassifyPESignature(data, nil)
	return buildSignatureInfo{
		Status: string(report.Status),
		Signer: report.SignerSubject,
		Issuer: report.SignerIssuer,
		Error:  report.Error,
	}, true
}
