//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// freeTCPPort reserves a throwaway TCP port on 0.0.0.0 and releases it, so the
// agent listener can bind the same number a moment later.
func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// startEchoServer runs a tiny echo server on 127.0.0.1:0 and returns its addr.
func startEchoServer() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	go func() {
		for {
			c, e := ln.Accept()
			if e != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}(c)
		}
	}()
	return ln.Addr().String(), nil
}

// roundTrip writes payload to addr and returns the first line received.
func roundTrip(t *testing.T, addr, payload string, timeout time.Duration) (string, error) {
	t.Helper()
	d := net.Dialer{Timeout: timeout}
	c, err := d.Dial("tcp", addr)
	if err != nil {
		return "", err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(timeout))
	if _, err := io.WriteString(c, payload); err != nil {
		return "", err
	}
	return bufio.NewReader(c).ReadString('\n')
}

// TestRPortFwdListenerBridges tests the agent-side pivot listener:
// connect to <agent:lport> and reach a host on the agent's network.
func TestRPortFwdListenerBridges(t *testing.T) {
	target, err := startEchoServer()
	if err != nil {
		t.Fatal(err)
	}
	port := freeTCPPort(t)
	laddr, err := startRPortForward(port, target)
	if err != nil {
		t.Fatal(err)
	}
	host, _, _ := net.SplitHostPort(laddr)
	if host == "" {
		t.Fatalf("expected a non-empty bind address, got %q", laddr)
	}

	t.Cleanup(func() { _, _ = stopRPortForward(port) })

	agentAddr := fmt.Sprintf("127.0.0.1:%d", port)
	for _, flush := range []bool{false, true} {
		got, err := roundTrip(t, agentAddr, "ping\n", 3*time.Second)
		if err != nil {
			t.Fatalf("round trip %v: %v", flush, err)
		}
		if got != "ping\n" {
			t.Fatalf("round trip expected echo, got %q", got)
		}
	}

	// A second connection also works while the first is still open.
	got, err := roundTrip(t, agentAddr, "again\n", 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got != "again\n" {
		t.Fatalf("expected echo, got %q", got)
	}
}

// TestRPortFwdListenerStopClosesBridges verifies stopRPortForward shuts the
// listener down and refuses to double-stop.
func TestRPortFwdListenerStopClosesBridges(t *testing.T) {
	target, err := startEchoServer()
	if err != nil {
		t.Fatal(err)
	}
	port := freeTCPPort(t)
	if _, err := startRPortForward(port, target); err != nil {
		t.Fatal(err)
	}
	agentAddr := fmt.Sprintf("127.0.0.1:%d", port)
	if got, err := roundTrip(t, agentAddr, "pre\n", 3*time.Second); err != nil || got != "pre\n" {
		t.Fatalf("pre-stop round trip failed: %v %q", err, got)
	}

	if n, err := stopRPortForward(port); err != nil || n != 0 {
		t.Fatalf("stopRPortForward = (%d, %v), want (0, nil)", n, err)
	}
	// Port must refuse new connections after stop.
	if _, err := net.DialTimeout("tcp", agentAddr, 200*time.Millisecond); err == nil {
		t.Fatal("listener still accepting after stop")
	}
	// Double stop reports an explicit error.
	if _, err := stopRPortForward(port); err == nil {
		t.Fatal("expected error stopping an inactive listener")
	}
}

// TestRPortFwdListenerArgValidation covers the honest error paths surfaced to
// the task handler (no silent stubs).
func TestRPortFwdListenerArgValidation(t *testing.T) {
	echo, err := startEchoServer()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		lport  int
		target string
	}{
		{"zero port", 0, echo},
		{"port too large", 65536, echo},
		{"negative port", -1, echo},
		{"empty target", 11111, ""},
		{"target missing port", 11112, "10.0.0.5"},
		{"target bad port", 11113, "10.0.0.5:notaport"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := startRPortForward(c.lport, c.target); err == nil {
				t.Fatalf("expected error for lport=%d target=%q", c.lport, c.target)
			}
		})
	}
}

// TestRPortFwdListenerRejectsDuplicatePort ensures a second start on the same
// port is refused rather than silently replacing the first listener.
func TestRPortFwdListenerRejectsDuplicatePort(t *testing.T) {
	echo, err := startEchoServer()
	if err != nil {
		t.Fatal(err)
	}
	port := freeTCPPort(t)
	t.Cleanup(func() { _, _ = stopRPortForward(port) })
	if _, err := startRPortForward(port, echo); err != nil {
		t.Fatal(err)
	}
	_, err = startRPortForward(port, "10.0.0.1:80")
	if err == nil || !strings.Contains(err.Error(), "already listening") {
		t.Fatalf("duplicate start: err=%v, want 'already listening'", err)
	}
}