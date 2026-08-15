//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"encoding/base64"
	"io"
	"net"
	"testing"
)

func setP2PSecret(t *testing.T, b64 string) {
	t.Helper()
	orig := P2PSharedSecret
	P2PSharedSecret = b64
	t.Cleanup(func() { P2PSharedSecret = orig })
}

func TestP2PNoAuthWhenSecretEmpty(t *testing.T) {
	setP2PSecret(t, "")
	server, client := net.Pipe()
	done := make(chan bool, 1)
	go func() { done <- p2pServerAuth(server) }()
	clientOk := p2pClientAuth(client)
	serverOk := <-done
	if !clientOk || !serverOk {
		t.Fatalf("with empty secret both sides must pass: client=%v server=%v", clientOk, serverOk)
	}
}

func TestP2PAuthSuccessWithSharedKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	setP2PSecret(t, base64.StdEncoding.EncodeToString(key))

	server, client := net.Pipe()
	done := make(chan bool, 1)
	go func() { done <- p2pServerAuth(server) }()
	clientOk := p2pClientAuth(client)
	serverOk := <-done
	if !clientOk || !serverOk {
		t.Fatalf("with matching secret both sides must pass: client=%v server=%v", clientOk, serverOk)
	}
}

func TestP2PAuthFailsWithWrongKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	setP2PSecret(t, base64.StdEncoding.EncodeToString(key))

	server, client := net.Pipe()
	done := make(chan bool, 1)
	go func() { done <- p2pServerAuth(server) }()
	// Simulate a peer that cannot prove the key: read the nonce, reply with a
	// bogus MAC instead of the real HMAC.
	var nonce [32]byte
	if _, err := io.ReadFull(client, nonce[:]); err != nil {
		t.Fatalf("read nonce: %v", err)
	}
	client.Write(make([]byte, 32)) // wrong MAC
	serverOk := <-done
	if serverOk {
		t.Fatalf("server must reject a peer that fails the HMAC challenge")
	}
}

// TestMalleableWrapStripRoundTrip verifies the response-side malleable cover
// applied to raw (non-HTTP) links (team-server TCP, P2P parent reply) is
// invertible by the client-side strip, for every prepend/append combination.
func TestMalleableWrapStripRoundTrip(t *testing.T) {
	origPre := MalleablePrepend
	origApp := MalleableAppend
	t.Cleanup(func() {
		MalleablePrepend = origPre
		MalleableAppend = origApp
	})

	cases := []struct{ pre, app string }{
		{"", ""},
		{"<html>", ""},
		{"", "</html>"},
		{"<html>", "</html>"},
		{"PRE-", "-POST"},
	}
	body := []byte(`{"uuid":"x","c":"y"}`)
	for _, c := range cases {
		MalleablePrepend = c.pre
		MalleableAppend = c.app
		wrapped := wrapMalleableResponse(body)
		if !bytes.Equal(stripMalleableWrapping(wrapped), body) {
			t.Fatalf("round-trip failed for pre=%q app=%q: %s", c.pre, c.app, stripMalleableWrapping(wrapped))
		}
	}
}
