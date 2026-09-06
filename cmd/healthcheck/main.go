package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"
)

// The server enables TLS by default (self-signed cert), so plain HTTP fails.
// Try HTTPS (skip verify: the cert is self-signed by design) first, then HTTP.
func main() {
	httpsClient := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // healthcheck against our own self-signed cert
	}
	if resp, err := httpsClient.Get("https://localhost:8000/health"); err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "healthcheck got status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get("http://localhost:8000/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
		os.Exit(1)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "healthcheck got status %d\n", resp.StatusCode)
		os.Exit(1)
	}
	os.Exit(0)
}
