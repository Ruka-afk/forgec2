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
const maxICMPAssemblies = 4096

type icmpAssembly struct {
	total int
	parts map[int][]byte
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
	st.last = time.Now()
	if st.total != total {
		return nil, fmt.Errorf("fragment total mismatch")
	}
	st.parts[index] = payload
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

// ICMPMaybePlain returns p unchanged when it is not an FC2I fragment, so
// un-fragmented (tiny) beacons still work.
func ICMPMaybePlain(p []byte) bool {
	if len(p) < ICMPFragHeaderSize {
		return true
	}
	return binary.LittleEndian.Uint32(p[0:4]) != ICMPFragMagic
}
