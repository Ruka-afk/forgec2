//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"net"
	"testing"
	"time"
)

// TestRportFwdPivot exercises the cross-platform reverse-port-forward core:
// dialing a pivot target, emitting the connected frame, echoing data back as
// rportfwd_data frames, and clean teardown. This is the logic that previously
// was Windows-only; it now backs pivoting on Linux and macOS too.
func TestRportFwdPivot(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 64)
		for {
			n, e2 := c.Read(buf)
			if n > 0 {
				if _, werr := c.Write(buf[:n]); werr != nil {
					return
				}
			}
			if e2 != nil {
				return
			}
		}
	}()

	const connID = uint64(0xDEADBEEF)
	rportfwdDial(connID, ln.Addr().String())

	deadline := time.Now().Add(3 * time.Second)
	connected := false
	for time.Now().Before(deadline) {
		for _, f := range rportfwdCollectOutbound() {
			if f.ConnID == connID && f.Action == "rportfwd_connected" {
				connected = true
			}
		}
		if connected {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !connected {
		t.Fatal("rportfwd_connected frame not emitted")
	}

	rportfwdWrite(connID, []byte("ping"))
	got := ""
	for time.Now().Before(deadline) {
		for _, f := range rportfwdCollectOutbound() {
			if f.ConnID == connID && f.Action == "rportfwd_data" {
				got = string(f.Data)
			}
		}
		if got != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got != "ping" {
		t.Fatalf("expected echoed 'ping', got %q", got)
	}
	rportfwdClose(connID)

	// After close the connection must be gone from the registry.
	rportfwdMu.Lock()
	_, still := rportfwdConns[connID]
	rportfwdMu.Unlock()
	if still {
		t.Fatal("rportfwd connection not cleaned up after close")
	}
}
