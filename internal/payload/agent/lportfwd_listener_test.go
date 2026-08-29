//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"net"
	"strconv"
	"testing"
	"time"
)

// freeListenerPort grabs an ephemeral port and releases it so a listener can
// be started on it without colliding in the test process.
func freeListenerPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// TestLPortFwdTunnelLoopback exercises the agent half of the tunnel end to
// end, in-process, with the test acting as the teamserver:
//
//	local process -> lportfwd listener -> outbound frames (connect+data)
//	inbound frames (data/close)   -> local process socket
func TestLPortFwdTunnelLoopback(t *testing.T) {
	target := "93.184.216.34:443" // never dialed here; carried in connect frame
	lport := freeListenerPort(t)

	addr, err := startLPortForward(lport, target)
	if err != nil {
		t.Fatalf("startLPortForward: %v", err)
	}
	if addr == "" {
		t.Fatal("empty bind address")
	}

	local, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("local process dial: %v", err)
	}
	defer local.Close()

	payload := []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")
	if _, err := local.Write(payload); err != nil {
		t.Fatalf("local write: %v", err)
	}

	// Drain frames as the beacon would.
	var connectSeen, dataSeen bool
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !(connectSeen && dataSeen) {
		frames := lportfwdCollectOutbound()
		for _, f := range frames {
			switch f.Action {
			case "lportfwd_connect":
				connectSeen = true
				if string(f.Data) != target {
					t.Fatalf("connect frame target = %q, want %q", string(f.Data), target)
				}
			case "lportfwd_data":
				dataSeen = true
				if string(f.Data) != string(payload) {
					t.Fatalf("data frame = %q, want %q", string(f.Data), string(payload))
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !connectSeen || !dataSeen {
		t.Fatalf("frames not collected: connect=%v data=%v", connectSeen, dataSeen)
	}

	// Server -> agent direction: inject response bytes.
	resp := []byte("HTTP/1.1 200 OK\r\n\r\n")
	lportfwdHandleFrames([]socksFrame{{ConnID: connIDFor(t, lport), Action: "lportfwd_data", Data: resp}})
	local.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, len(resp))
	if _, err := ioReadFull(local, buf); err != nil {
		t.Fatalf("reading tunneled response: %v", err)
	}
	if string(buf) != string(resp) {
		t.Fatalf("response = %q, want %q", string(buf), string(resp))
	}

	// Server closes the target leg -> local socket must close.
	cid := connIDFor(t, lport)
	lportfwdHandleFrames([]socksFrame{{ConnID: cid, Action: "lportfwd_close"}})
	local.SetReadDeadline(time.Now().Add(2 * time.Second))
	one := make([]byte, 1)
	if n, err := local.Read(one); n != 0 {
		t.Fatalf("expected EOF after close frame, got n=%d err=%v", n, err)
	}

	// Beacon drain: closed legs are reaped only after their residual frames
	// (the trailing close) ship on the next collect. Without this the entry
	// would linger in lpfConns until the next beacon, polluting the next test.
	lportfwdCollectOutbound()
}

// TestStopLPortForward covers stop semantics: active connections torn down,
// second stop errors, listener refuses new dials.
func TestStopLPortForward(t *testing.T) {
	lport := freeListenerPort(t)
	if _, err := startLPortForward(lport, "10.9.9.9:80"); err != nil {
		t.Fatalf("start: %v", err)
	}
	local, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(lport)), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	// Wait until the accept loop has registered the tunnel; stop-vs-accept is
	// inherently racy and the test wants the teardown path with one live conn.
	deadline := time.Now().Add(3 * time.Second)
	for {
		lpfMu.Lock()
		n := len(lpfConns)
		lpfMu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("tunnel never registered")
		}
		time.Sleep(5 * time.Millisecond)
	}

	n, err := stopLPortForward(lport)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if n != 1 {
		t.Fatalf("stopped conns = %d, want 1", n)
	}
	local.SetReadDeadline(time.Now().Add(2 * time.Second))
	one := make([]byte, 1)
	if _, rerr := local.Read(one); rerr == nil {
		t.Fatal("local connection survived stop")
	}
	if _, err := stopLPortForward(lport); err == nil {
		t.Fatal("second stop must error")
	}
}

// connIDFor finds the single live tunnel ConnID (tests keep exactly one).
func connIDFor(t *testing.T, lport int) uint64 {
	t.Helper()
	lpfMu.Lock()
	defer lpfMu.Unlock()
	for cid := range lpfConns {
		return cid
	}
	t.Fatalf("no live lportfwd connection for :%d", lport)
	return 0
}

func ioReadFull(c net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := c.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
