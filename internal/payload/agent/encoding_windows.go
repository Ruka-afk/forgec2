//go:build windows
// +build windows

package main

import (
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// decodeShellOutput converts raw Windows console bytes to UTF-8.
// cmd.exe on Chinese Windows emits GBK (CP936); PowerShell may emit UTF-16LE.
func decodeShellOutput(raw []byte, shell string) string {
	if len(raw) == 0 {
		return ""
	}

	// UTF-8 BOM
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
		if utf8.Valid(raw) {
			return string(raw)
		}
	}

	// UTF-16 LE BOM (common for PowerShell)
	if len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE {
		return decodeUTF16LE(raw[2:])
	}

	// UTF-16 LE without BOM (ASCII chars followed by null bytes)
	if looksLikeUTF16LE(raw) {
		if s := decodeUTF16LE(raw); s != "" {
			return s
		}
	}

	// Already valid UTF-8 (e.g. chcp 65001 or PowerShell UTF-8 mode)
	if utf8.Valid(raw) {
		return string(raw)
	}

	// GBK / CP936 — default OEM code page on Chinese Windows cmd.exe
	if utf8Bytes, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), raw); err == nil && len(utf8Bytes) > 0 {
		return string(utf8Bytes)
	}

	return string(raw)
}

func looksLikeUTF16LE(b []byte) bool {
	if len(b) < 4 || len(b)%2 != 0 {
		return false
	}
	nulls := 0
	for i := 1; i < len(b) && i < 64; i += 2 {
		if b[i] == 0 {
			nulls++
		}
	}
	return nulls >= 2
}

func decodeUTF16LE(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = uint16(b[i*2]) | uint16(b[i*2+1])<<8
	}
	return string(utf16.Decode(u16))
}