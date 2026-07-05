package server

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"sync"
)

// TCPProtoListener is a protobuf-wire-format TCP listener.
// Wire format: [4-byte big-endian length][payload bytes].
// This is transport-compatible with the agent's simplified protobuf transport.
type TCPProtoListener struct {
	addr    string
	ln      net.Listener
	handler func(agentID string, reqJSON []byte) []byte
	wg      sync.WaitGroup
	quit    chan struct{}
}

func NewTCPProtoListener(addr string) *TCPProtoListener {
	return &TCPProtoListener{addr: addr, quit: make(chan struct{})}
}

func (l *TCPProtoListener) SetHandler(h func(agentID string, reqJSON []byte) []byte) {
	l.handler = h
}

func (l *TCPProtoListener) Start() error {
	var err error
	l.ln, err = net.Listen("tcp", l.addr)
	if err != nil {
		return err
	}

	go l.serve()
	slog.Info("TCP Proto listener started", "addr", l.addr)
	return nil
}

func (l *TCPProtoListener) Stop() error {
	close(l.quit)
	if l.ln != nil {
		l.ln.Close()
	}
	l.wg.Wait()
	return nil
}

func (l *TCPProtoListener) serve() {
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			select {
			case <-l.quit:
				return
			default:
				slog.Error("TCP Proto accept error", "err", err)
				continue
			}
		}
		l.wg.Add(1)
		go l.handleConn(conn)
	}
}

func (l *TCPProtoListener) handleConn(conn net.Conn) {
	defer conn.Close()
	defer l.wg.Done()

	// Read length-prefixed request
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		slog.Debug("TCP Proto read length failed", "remote", conn.RemoteAddr(), "err", err)
		return
	}
	reqLen := binary.BigEndian.Uint32(lenBuf)
	if reqLen > 10*1024*1024 {
		slog.Warn("TCP Proto request too large", "size", reqLen, "remote", conn.RemoteAddr())
		return
	}

	reqData := make([]byte, reqLen)
	if _, err := io.ReadFull(conn, reqData); err != nil {
		slog.Debug("TCP Proto read payload failed", "remote", conn.RemoteAddr(), "err", err)
		return
	}

	// Parse agent ID
	var envelope struct {
		UUID string `json:"uuid,omitempty"`
	}
	agentID := ""
	if err := json.Unmarshal(reqData, &envelope); err == nil {
		agentID = envelope.UUID
	}

	// Process beacon
	respData := l.handler(agentID, reqData)

	// Write response
	respLen := uint32(len(respData))
	binary.BigEndian.PutUint32(lenBuf, respLen)
	if _, err := conn.Write(lenBuf); err != nil {
		slog.Debug("TCP Proto write length failed", "err", err)
		return
	}
	if _, err := conn.Write(respData); err != nil {
		slog.Debug("TCP Proto write payload failed", "err", err)
		return
	}
}

// startTCPProtoListener registers the listener in server startup
func (s *Server) startTCPProtoListener() {
	addr := s.cfg.Server.GRPCAddr
	if addr == "" {
		return
	}
	listener := NewTCPProtoListener(addr)
	listener.SetHandler(func(agentID string, reqJSON []byte) []byte {
		var req beaconRequest
		if len(reqJSON) > 0 {
			if err := json.Unmarshal(reqJSON, &req); err != nil {
				slog.Error("TCP Proto beacon handler unmarshal error", "err", err)
			}
		}
		if req.UUID == "" {
			req.UUID = agentID
		}
		resp := s.processBeacon(req, "")
		respJSON, err := json.Marshal(resp)
		if err != nil {
			slog.Error("TCP Proto marshal response failed", "error", err)
			return nil
		}
		return respJSON
	})
	if err := listener.Start(); err != nil {
		slog.Error("Failed to start TCP Proto listener", "addr", addr, "err", err)
	}
	s.tcpProtoListener = listener
	slog.Info("TCP Proto listener registered", "addr", addr)
}
