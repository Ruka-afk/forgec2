package protocol

import (
	"bytes"
	"testing"
	"time"
)

func TestICMPFragRoundTrip(t *testing.T) {
	body := bytes.Repeat([]byte("forgec2-icmp-frag-"), 80) // ~1440 bytes
	frags := ICMPFragSplit(body)
	if len(frags) < 2 {
		t.Fatalf("expected multiple fragments, got %d", len(frags))
	}
	asm := NewICMPAssembler()
	var got []byte
	for i, f := range frags {
		id, total, index, payload, ok := ICMPFragParse(f)
		if !ok {
			t.Fatalf("parse frag %d", i)
		}
		if total != len(frags) || index != i {
			t.Fatalf("hdr total=%d index=%d i=%d n=%d", total, index, i, len(frags))
		}
		key := "peer:1:" + itoa(int(id))
		out, err := asm.Add(key, total, index, payload)
		if err != nil {
			t.Fatal(err)
		}
		if out != nil {
			got = out
		}
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("reassembled %d want %d", len(got), len(body))
	}
}

func TestICMPFragParseRejectsGarbage(t *testing.T) {
	if _, _, _, _, ok := ICMPFragParse([]byte("not-a-fragment")); ok {
		t.Fatal("garbage parsed as fragment")
	}
}

// A hostile peer pads every part to the 8 KiB listener read-buffer size; the
// byte cap must discard the stream instead of buffering up to 2 MiB per key.
func TestICMPAsmByteCap(t *testing.T) {
	asm := NewICMPAssembler()
	part := bytes.Repeat([]byte{0x42}, 8192)
	const total = 64 // 64 x 8 KiB = 512 KiB > icmpFragMaxAssembly
	var rejected int
	for i := 0; i < total; i++ {
		out, err := asm.Add("k", total, i, part)
		if err != nil {
			rejected++
			// After the overflow the stream was discarded: the next add
			// starts clean and this one must be accepted again.
			if out != nil {
				t.Fatal("rejected add returned a payload")
			}
			if got, err2 := asm.Add("k", total, 0, part); err2 != nil || got != nil {
				t.Fatalf("fresh add after discard: out=%v err=%v", got != nil, err2)
			}
			break
		}
		if out != nil {
			t.Fatal("assembly completed past the byte cap")
		}
	}
	if rejected == 0 {
		t.Fatal("byte cap never triggered")
	}
}

func TestICMPAsmByteCapAllowsLegit(t *testing.T) {
	asm := NewICMPAssembler()
	// 64 x 512 = 32 KiB: far under the cap, must reassemble.
	body := bytes.Repeat([]byte("x"), 32*1024)
	frags := ICMPFragSplit(body)
	var got []byte
	for i, f := range frags {
		id, total, index, payload, ok := ICMPFragParse(f)
		if !ok {
			t.Fatalf("parse frag %d", i)
		}
		out, err := asm.Add("legit", total, index, payload)
		if err != nil {
			t.Fatal(err)
		}
		if out != nil {
			got = out
		}
		_ = id
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("reassembled %d want %d", len(got), len(body))
	}
}

// Distinct-key flood: the cardinality cap must hold and evict the oldest.
func TestICMPAsmCardinalityCap(t *testing.T) {
	asm := NewICMPAssembler()
	part := []byte("p")
	for i := 0; i < maxICMPAssemblies; i++ {
		if _, err := asm.Add("k:"+itoa(i), 2, 0, part); err != nil {
			t.Fatal(err)
		}
	}
	if len(asm.items) != maxICMPAssemblies {
		t.Fatalf("at cap: have %d want %d", len(asm.items), maxICMPAssemblies)
	}
	// One more distinct key must evict (k:0 is oldest) and hold the cap.
	// Age k:0 explicitly: within a tight loop every entry's timestamp ties
	// to the same tick, so without it the "oldest" scan is arbitrary. Five
	// seconds keeps it inside the TTL so the lazy GC leaves it alone and
	// this really exercises the LRU eviction.
	asm.items["k:0"].last = time.Now().Add(-5 * time.Second)
	if _, err := asm.Add("k:new", 2, 0, part); err != nil {
		t.Fatal(err)
	}
	if len(asm.items) != maxICMPAssemblies {
		t.Fatalf("after eviction: have %d want %d", len(asm.items), maxICMPAssemblies)
	}
	if _, ok := asm.items["k:0"]; ok {
		t.Fatal("oldest assembly not evicted")
	}
}

// GC must sweep TTL-expired entries on demand.
func TestICMPAsmGC(t *testing.T) {
	asm := NewICMPAssembler()
	asm.items["stale"] = &icmpAssembly{total: 2, parts: map[int][]byte{}, last: time.Now().Add(-2 * ICMPFragTTL)}
	asm.GC()
	if _, ok := asm.items["stale"]; ok {
		t.Fatal("stale assembly not swept")
	}
	// GC is a no-op on a nil receiver (agent builds a fresh assembler per
	// beacon; a mis-wired GC call must not panic).
	var nilAsm *ICMPAssembler
	nilAsm.GC()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
