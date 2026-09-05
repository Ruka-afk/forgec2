package server

import (
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
)

// ── Periodic sweeps, port utils, raw TCP/UDP transports ───────────────────

func (s *Server) runPeriodicCleanup() {
	s.cleanupOldData()
	ticker := time.NewTicker(PeriodicCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupOldData()
		}
	}
}

func (s *Server) periodicRPortFwdCleanup() {
	ticker := time.NewTicker(RPortFwdCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupStaleRPortFwd()
			s.cleanupStaleLPortFwd()
		}
	}
}

// cleanupStaleLPortFwd drops tunneled lportfwd connections whose agent no
// longer exists or is offline, mirroring the rportfwd sweep.
func (s *Server) cleanupStaleLPortFwd() {
	s.lportfwdMu.Lock()
	agentIDs := make([]string, 0, len(s.lportfwdTargets))
	for _, t := range s.lportfwdTargets {
		agentIDs = append(agentIDs, t.agentID)
	}
	s.lportfwdMu.Unlock()
	if len(agentIDs) == 0 {
		return
	}
	var agents []db.Implant
	if err := s.db.Where("id IN ?", agentIDs).Limit(len(agentIDs)).Find(&agents).Error; err != nil {
		slog.Error("Failed to batch-load agents for lportfwd cleanup", "error", err)
		return
	}
	online := make(map[string]bool, len(agents))
	for i := range agents {
		online[agents[i].ID] = agents[i].Status == "online"
	}
	for _, id := range agentIDs {
		if !online[id] {
			s.cleanupLPortFwdForAgent(id)
		}
	}
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// isPortAvailable checks whether the given host:port can be listened on.
func isPortAvailable(host string, port int) bool {
	addr := host + ":" + itoa(port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// startTCPListener starts a raw TCP transport listener for agents using Protocol=tcp.
// Uses length-prefixed JSON (4-byte BE len + JSON) for BeaconRequest / BeaconResponse.
func (s *Server) startTCPListener() {
	ln, err := net.Listen("tcp", s.cfg.Server.TCPAddr)
	if err != nil {
		slog.Error("Failed to start TCP listener", "addr", s.cfg.Server.TCPAddr, "err", err)
		return
	}
	s.tcpLn = ln
	slog.Info("TCP transport layer listening", "addr", s.cfg.Server.TCPAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				slog.Error("TCP accept error", "addr", s.cfg.Server.TCPAddr, "err", err)
				continue
			}
		}
		s.wg.Add(1)
		go s.handleTCPConnection(conn)
	}
}

func (s *Server) handleTCPConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()
	slog.Info("TCP agent connected", "remote", conn.RemoteAddr().String())

	for {
		conn.SetReadDeadline(time.Now().Add(TCPReadDeadline))
		if err := conn.SetWriteDeadline(time.Now().Add(TCPWriteDeadline)); err != nil {
			slog.Error("TCP set write deadline error", "remote", conn.RemoteAddr().String(), "error", err)
		}

		// Read length prefix (big endian uint32)
		var msgLen uint32
		if err := binary.Read(conn, binary.BigEndian, &msgLen); err != nil {
			return
		}
		if msgLen == 0 || msgLen > TCPMaxMessageSize {
			return
		}

		buf := make([]byte, msgLen)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}

		env, req, kind := s.decodeBeaconEnvelope(buf)
		if kind == frameRejected {
			return
		}

		var respBytes []byte
		if kind == frameEncrypted {
			resp := s.processBeacon(req, "")
			if s.sessionManager.NeedsRekey(req.UUID, BeaconSessionRekeyMessages) {
				resp.Rekey = true
			}
			var ok bool
			// Wrap the response in the same transport envelope as HTTP so the
			// agent-side ECDH handshake/decryption logic works over TCP too.
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

		// Apply the same malleable prepend/append cover used by the HTTP
		// transport so raw TCP links are not distinguishable by an un-wrapped
		// JSON envelope on the wire (I2). The agent strips these bytes on read.
		respBytes = s.applyMalleableWrapping(respBytes)
		if err := binary.Write(conn, binary.BigEndian, uint32(len(respBytes))); err != nil {
			return
		}
		if _, err := conn.Write(respBytes); err != nil {
			return
		}
	}
}

// startUDPListener starts a connectionless UDP datagram transport for agents
// using Protocol=udp. Each beacon is a single datagram carrying the same raw
// v2 envelope as the TCP transport; the server replies with one datagram.
// Payloads must fit within the link MTU (the agent caps sends at 16MiB but
// real-world UDP is limited by Path MTU — large results should use a
// connection-oriented transport).
func (s *Server) startUDPListener() {
	pc, err := net.ListenPacket("udp", s.cfg.Server.UDPAddr)
	if err != nil {
		slog.Error("Failed to start UDP listener", "addr", s.cfg.Server.UDPAddr, "err", err)
		return
	}
	s.udpConn = pc
	slog.Info("UDP transport layer listening", "addr", s.cfg.Server.UDPAddr)

	buf := make([]byte, 16*1024*1024)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer pc.Close()
		for {
			pc.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				select {
				case <-s.ctx.Done():
					return
				default:
				}
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				slog.Error("UDP read error", "err", err)
				return
			}
			if n == 0 {
				continue
			}
			data := make([]byte, n)
			copy(data, buf[:n])
			resp := s.handleUDPBeacon(data, addr)
			if len(resp) == 0 {
				continue
			}
			if _, err := pc.WriteTo(resp, addr); err != nil {
				slog.Error("UDP write error", "remote", addr.String(), "err", err)
			}
		}
	}()
}

// handleUDPBeacon processes one UDP beacon datagram and returns the response
// datagram bytes (or nil to send nothing). It reuses the shared raw-listener
// beacon core so UDP behaves identically to TCP/ICMP at the envelope level.
func (s *Server) handleUDPBeacon(data []byte, addr net.Addr) []byte {
	resp := s.handleListenerBeacon("", data)
	if len(resp) == 0 {
		return nil
	}
	// Mirror the TCP transport's optional malleable cover (a no-op unless a
	// malleable profile with prepend/append is configured).
	return s.applyMalleableWrapping(resp)
}

type udpPacketCloser struct {
	pc net.PacketConn
}

func (u *udpPacketCloser) Close() error {
	if u == nil || u.pc == nil {
		return nil
	}
	return u.pc.Close()
}

func (s *Server) startExtraUDPListener(key string) error {
	addr := strings.TrimPrefix(key, "udp://")
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	s.extraListenersMu.Lock()
	s.extraListeners[key] = &udpPacketCloser{pc: pc}
	s.extraListenersMu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer pc.Close()
		buf := make([]byte, 16*1024*1024)
		for {
			select {
			case <-s.ctx.Done():
				return
			default:
			}
			n, raddr, readErr := pc.ReadFrom(buf)
			if readErr != nil {
				select {
				case <-s.ctx.Done():
					return
				default:
				}
				return
			}
			if n == 0 {
				continue
			}
			data := make([]byte, n)
			copy(data, buf[:n])
			resp := s.handleUDPBeacon(data, raddr)
			if len(resp) == 0 {
				continue
			}
			_, _ = pc.WriteTo(resp, raddr)
		}
	}()
	slog.Info("Extra UDP listener started", "addr", addr, "key", key)
	return nil
}
