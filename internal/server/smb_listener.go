package server

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
)

// newSMBListener is a package-level var so Windows can override via init()
var newSMBListener = defaultSMBListen

func defaultSMBListen(addr string) (net.Listener, error) {
	dir := filepath.Dir(addr)
	if dir != "." {
		os.MkdirAll(dir, 0700)
	}
	os.Remove(addr)
	return net.Listen("unix", addr)
}

func (s *Server) startSMBListener() {
	listenAddr := s.cfg.Server.SMBPipe
	if listenAddr == "" {
		slog.Warn("SMB listener: no pipe/addr configured")
		return
	}

	ln, err := newSMBListener(listenAddr)
	if err != nil {
		slog.Error("Failed to start SMB listener", "addr", listenAddr, "err", err)
		return
	}
	slog.Info("SMB transport layer listening", "addr", listenAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go s.handleSMBConnection(conn)
	}
}

func (s *Server) handleSMBConnection(conn net.Conn) {
	defer conn.Close()
	slog.Info("SMB agent connected", "remote", conn.RemoteAddr().String())

	for {
		var msgLen uint32
		if err := binary.Read(conn, binary.BigEndian, &msgLen); err != nil {
			return
		}
		if msgLen == 0 || msgLen > 16*1024*1024 {
			return
		}

		buf := make([]byte, msgLen)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}

		var req beaconRequest
		if err := json.Unmarshal(buf, &req); err != nil {
			slog.Error("SMB bad beacon json", "err", err)
			return
		}

		resp := s.processBeacon(req, "")

		respBytes, _ := json.Marshal(resp)
		if err := binary.Write(conn, binary.BigEndian, uint32(len(respBytes))); err != nil {
			return
		}
		if _, err := conn.Write(respBytes); err != nil {
			return
		}
	}
}

