package server

import (
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"
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
	s.smbLn = ln
	slog.Info("SMB transport layer listening", "addr", listenAddr)

	s.wg.Add(1)
	defer s.wg.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			slog.Error("SMB accept error", "addr", listenAddr, "err", err)
			time.Sleep(100 * time.Millisecond)
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

		env, req, kind := s.decodeBeaconEnvelope(buf)
		if kind == frameRejected {
			slog.Error("SMB beacon envelope rejected", "remote", conn.RemoteAddr().String())
			return
		}

		var respBytes []byte
		if kind == frameEncrypted {
			resp := s.processBeacon(req, "")
			if s.sessionManager != nil && s.sessionManager.NeedsRekey(req.UUID, BeaconSessionRekeyMessages) {
				resp.Rekey = true
			}
			var ok bool
			respBytes, ok = s.buildBeaconResponse(req.UUID, env.Seq, resp)
			if !ok {
				return
			}
		} else {
			var ok bool
			respBytes, ok = s.processAuthFrame(env, kind)
			if !ok {
				return
			}
		}
		respBytes = s.applyMalleableWrapping(respBytes)
		if err := binary.Write(conn, binary.BigEndian, uint32(len(respBytes))); err != nil {
			return
		}
		if _, err := conn.Write(respBytes); err != nil {
			return
		}
	}
}
