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
// Host/port follow FORGEC2_HOST/FORGEC2_PORT so custom-port deploys don't
// report unhealthy forever.
func endpoint(scheme string) string {
	host := os.Getenv("FORGEC2_HOST")
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	port := os.Getenv("FORGEC2_PORT")
	if port == "" {
		port = "8000"
	}
	return scheme + "://" + host + ":" + port + "/health"
}

func check(client *http.Client, url string) (int, error) {
	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

func main() {
	httpsClient := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // healthcheck against our own self-signed cert
	}
	if code, err := check(httpsClient, endpoint("https")); err == nil {
		if code == 200 {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "healthcheck got status %d\n", code)
		os.Exit(1)
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	code, err := check(httpClient, endpoint("http"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
		os.Exit(1)
	}
	if code != 200 {
		fmt.Fprintf(os.Stderr, "healthcheck got status %d\n", code)
		os.Exit(1)
	}
	os.Exit(0)
}
