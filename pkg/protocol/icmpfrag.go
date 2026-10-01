package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// ICMP fragmentation: v2 envelopes routinely exceed a single Echo payload.
// Each fragment is:
//
//	magic[4] = "FC2I"
//	msgid[4] little-endian
//	total[2] little-endian fragment count
//	index[2] little-endian 0-based index
//	payload
//
// Max payload per fragment is ICMPFragMaxPayload so the Echo stays well under
// typical 1500-byte MTU after IP/ICMP headers.

const (
	ICMPFragMagic      = 0x49324346 // "FC2I" little-endian
	ICMPFragHeaderSize = 12
	ICMPFragMaxPayload = 512
	ICMPFragTTL        = 30 * time.Second
)

// ICMPFragSplit chops body into on-wire ICMP payloads.
func ICMPFragSplit(body []byte) [][]byte {
	if len(body) == 0 {
		return nil
	}
	var msgID uint32
	_ = binary.Read(rand.Reader, binary.LittleEndian, &msgID)
	if msgID == 0 {
		msgID = uint32(time.Now().UnixNano())
	}
	n := (len(body) + ICMPFragMaxPayload - 1) / ICMPFragMaxPayload
	if n == 0 {
		n = 1
	}
	out := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		start := i * ICMPFragMaxPayload
		end := start + ICMPFragMaxPayload
		if end > len(body) {
			end = len(body)
		}
		chunk := body[start:end]
		buf := make([]byte, ICMPFragHeaderSize+len(chunk))
		binary.LittleEndian.PutUint32(buf[0:4], ICMPFragMagic)
		binary.LittleEndian.PutUint32(buf[4:8], msgID)
		binary.LittleEndian.PutUint16(buf[8:10], uint16(n))
		binary.LittleEndian.PutUint16(buf[10:12], uint16(i))
		copy(buf[ICMPFragHeaderSize:], chunk)
		out = append(out, buf)
	}
	return out
}

// ICMPFragParse returns (msgid, total, index, payload, ok).
func ICMPFragParse(p []byte) (msgID uint32, total, index int, payload []byte, ok bool) {
	if len(p) < ICMPFragHeaderSize {
		return 0, 0, 0, nil, false
	}
	if binary.LittleEndian.Uint32(p[0:4]) != ICMPFragMagic {
		return 0, 0, 0, nil, false
	}
	msgID = binary.LittleEndian.Uint32(p[4:8])
	total = int(binary.LittleEndian.Uint16(p[8:10]))
	index = int(binary.LittleEndian.Uint16(p[10:12]))
	if total <= 0 || total > 256 || index < 0 || index >= total {
		return 0, 0, 0, nil, false
	}
	return msgID, total, index, append([]byte(nil), p[ICMPFragHeaderSize:]...), true
}

// ICMPAssembler reassembles fragmented ICMP C2 payloads keyed by peer+id+msgid.
type ICMPAssembler struct {
	mu    sync.Mutex
	items map[string]*icmpAssembly
}

// maxICMPAssemblies caps distinct in-flight assemblies (same rationale as the
// DNS fragmenter cap): keys are attacker-influenceable, TTL sweeping is lazy.
// Pair with icmpFragMaxAssembly the fleet-wide worst case stays bounded at
// maxICMPAssemblies * icmpFragMaxAssembly = 256 MiB instead of the
// 256 fragments * 8 KiB read-buffer part size * 4096 keys ~= 8 GiB a flood
// could previously pin.
const maxICMPAssemblies = 1024

// icmpFragMaxAssembly caps total reassembled bytes per key. 256 KiB is far
// above any legitimate C2 envelope (implants fragment bodies of at most a
// few KiB at 512-byte parts); beyond that the stream is an attack.
const icmpFragMaxAssembly = 256 * 1024

type icmpAssembly struct {
	total int
	parts map[int][]byte
	bytes int
	last  time.Time
}

func NewICMPAssembler() *ICMPAssembler {
	return &ICMPAssembler{items: make(map[string]*icmpAssembly)}
}

func (a *ICMPAssembler) Add(key string, total, index int, payload []byte) ([]byte, error) {
	if a == nil {
		return nil, fmt.Errorf("nil assembler")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.gcLocked()
	// Cardinality cap: keys embed the (spoofable) source address, so a flood
	// of unique keys inserts entries faster than the lazy TTL sweep can drop
	// them — unauthenticated remote memory exhaustion. Mirror the DNS
	// fragmenter: cap distinct assemblies and evict the oldest.
	if _, ok := a.items[key]; !ok && len(a.items) >= maxICMPAssemblies {
		oldestKey := ""
		var oldest time.Time
		for k, st := range a.items {
			if oldestKey == "" || st.last.Before(oldest) {
				oldestKey = k
				oldest = st.last
			}
		}
		if oldestKey != "" {
			delete(a.items, oldestKey)
		}
	}
	st, ok := a.items[key]
	if !ok {
		st = &icmpAssembly{total: total, parts: make(map[int][]byte), last: time.Now()}
		a.items[key] = st
	}
	// Byte cap: an attacker can pad every part with 8 KiB read-buffer-sized
	// payloads (total is capped at 256 by ICMPFragParse, ~= 2 MiB per key);
	// bound the stream itself and discard it once it crosses the limit. The
	// check uses the conservative upper bound (re-counts a duplicate index),
	// which can only reject earlier, never later.
	if st.bytes+len(payload) > icmpFragMaxAssembly {
		delete(a.items, key)
		return nil, fmt.Errorf("fragment assembly exceeds %d bytes", icmpFragMaxAssembly)
	}
	st.last = time.Now()
	if st.total != total {
		return nil, fmt.Errorf("fragment total mismatch")
	}
	// Recompute rather than accumulate so a re-sent index (duplicate part)
	// is counted once. n <= 256 parts, cost is negligible.
	st.parts[index] = payload
	st.bytes = 0
	for _, p := range st.parts {
		st.bytes += len(p)
	}
	if len(st.parts) < total {
		return nil, nil
	}
	var size int
	for i := 0; i < total; i++ {
		p, ok := st.parts[i]
		if !ok {
			return nil, nil
		}
		size += len(p)
	}
	out := make([]byte, 0, size)
	for i := 0; i < total; i++ {
		out = append(out, st.parts[i]...)
	}
	delete(a.items, key)
	return out, nil
}

func (a *ICMPAssembler) gcLocked() {
	cutoff := time.Now().Add(-ICMPFragTTL)
	for k, st := range a.items {
		if st.last.Before(cutoff) {
			delete(a.items, k)
		}
	}
}

// GC sweeps TTL-expired entries now, instead of waiting for the next Add to
// lazy-sweep them. Long-lived listeners (the server beacon loop) run it on a
// timer so a stopped flood cannot pin expired assemblies in memory
// indefinitely.
func (a *ICMPAssembler) GC() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.gcLocked()
}

// ICMPMaybePlain returns p unchanged when it is not an FC2I fragment, so
// un-fragmented (tiny) beacons still work.
func ICMPMaybePlain(p []byte) bool {
	if len(p) < ICMPFragHeaderSize {
		return true
	}
	return binary.LittleEndian.Uint32(p[0:4]) != ICMPFragMagic
}
