package protocol

import (
	"bytes"
	"testing"
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
