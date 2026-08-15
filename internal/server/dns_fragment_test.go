package server

import (
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

type dnsTestWriter struct {
	msgs []*dns.Msg
}

func (w *dnsTestWriter) LocalAddr() net.Addr  { return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 53} }
func (w *dnsTestWriter) RemoteAddr() net.Addr { return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5353} }
func (w *dnsTestWriter) WriteMsg(m *dns.Msg) error {
	w.msgs = append(w.msgs, m)
	return nil
}
func (w *dnsTestWriter) Write(b []byte) (int, error) { w.msgs = append(w.msgs, nil); return len(b), nil }
func (w *dnsTestWriter) Close() error                { return nil }
func (w *dnsTestWriter) TsigStatus() error           { return nil }
func (w *dnsTestWriter) TsigTimersOnly(bool)         {}
func (w *dnsTestWriter) Hijack()                     {}

func fragQName(domain, agentID string, total, idx int, payload []byte) string {
	enc := base32.StdEncoding.EncodeToString(payload)
	enc = strings.TrimRight(enc, "=")
	base := agentID + "." + strconv.Itoa(total) + "_" + strconv.Itoa(idx) + "."
	var labels []string
	for j := 0; j < len(enc); j += 63 {
		e := j + 63
		if e > len(enc) {
			e = len(enc)
		}
		labels = append(labels, enc[j:e])
	}
	return strings.TrimSuffix(base+strings.Join(labels, "."), ".") + ".dns." + domain
}

func TestDNSFragmentAssemblyReassemblesBeacon(t *testing.T) {
	dl := NewDNSBeaconListener("dns.evil.test", "127.0.0.1", 0, ":0")
	defer dl.Stop()

	body := make([]byte, 500)
	for i := range body {
		body[i] = byte(i % 251)
	}
	agentID := strings.Repeat("a", 32)
	parts := 9
	chunk := len(body) / parts

	var got []byte
	var calls int
	dl.SetHandler(func(id string, req []byte) []byte {
		calls++
		if id != agentID {
			t.Fatalf("handler got agent %q want %q", id, agentID)
		}
		got = append(got[:0], req...)
		return []byte("H")
	})

	w := &dnsTestWriter{}
	for i := 0; i < parts; i++ {
		start := i * chunk
		end := start + chunk
		if i == parts-1 {
			end = len(body)
		}
		q := fragQName(dl.Domain, agentID, parts, i, body[start:end])
		qname := q
		if !strings.HasSuffix(qname, ".") {
			qname += "."
		}
		r := new(dns.Msg)
		r.SetQuestion(qname, dns.TypeTXT)
		dl.handleQuery(w, r)
	}

	_ = calls
	if got == nil || !bytesEqual(got, body) {
		t.Fatalf("reassembled body mismatch: got %d bytes want %d", len(got), len(body))
	}
	if calls != 1 {
		t.Fatalf("handler should be called once, got %d", calls)
	}
	if len(w.msgs) != parts {
		t.Fatalf("expected %d responses, got %d", parts, len(w.msgs))
	}
	// First two fragments return an immediate blank ack, final carries real data.
	for i := 0; i < parts-1; i++ {
		if len(w.msgs[i].Answer) != 1 {
			t.Fatalf("response %d should carry a single blank TXT", i)
		}
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDNSFragmentRejectsMalformedMeta(t *testing.T) {
	dl := NewDNSBeaconListener("dns.evil.test", "127.0.0.1", 0, ":0")
	defer dl.Stop()
	dl.SetHandler(func(string, []byte) []byte { return []byte("X") })

	w := &dnsTestWriter{}
	// total exceeds the cap (65) — must be rejected with FORMERR.
	bad := fragQName("dns.evil.test", strings.Repeat("b", 32), 65, 0, []byte("frag"))
	if !strings.HasSuffix(bad, ".") {
		bad += "."
	}
	r := new(dns.Msg)
	r.SetQuestion(bad, dns.TypeTXT)
	dl.handleQuery(w, r)
	if len(w.msgs) == 0 {
		t.Fatal("expected a response message")
	}
	if w.msgs[0].Rcode != dns.RcodeFormatError {
		t.Fatalf("malformed fragment should return FORMERR, got %d", w.msgs[0].Rcode)
	}
}

// TestDNSAAAAResponseDecodes verifies AAAA (IPv6) DNS tunneling: the server
// packs the base64 response into 16-byte AAAA rdata chunks and the agent-side
// parser (parseDNSResponse) must reconstruct the payload. Before the fix the
// agent decoded the IPv6 address string (e.g. "2001:db8::1") as base64, which
// always failed.
func TestDNSAAAAResponseDecodes(t *testing.T) {
	dl := NewDNSBeaconListener("dns.evil.test", "127.0.0.1", 0, ":0")
	defer dl.Stop()

	respPayload := []byte(`{"seq":1,"reg_ok":true}`)
	agentID := strings.Repeat("a", 32)
	dl.SetHandler(func(id string, req []byte) []byte {
		return respPayload
	})

	w := &dnsTestWriter{}
	q := agentID + ".dns.dns.evil.test."
	r := new(dns.Msg)
	r.SetQuestion(q, dns.TypeAAAA)
	dl.handleQuery(w, r)

	if len(w.msgs) != 1 {
		t.Fatalf("expected 1 response, got %d", len(w.msgs))
	}
	m := w.msgs[0]
	if len(m.Answer) == 0 {
		t.Fatal("expected AAAA answer records")
	}

	var combined strings.Builder
	for _, rr := range m.Answer {
		a, ok := rr.(*dns.AAAA)
		if !ok {
			t.Fatalf("expected AAAA record, got %T", rr)
		}
		combined.WriteString(string(a.AAAA))
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(combined.String()))
	if err != nil {
		t.Fatalf("base64 decode of AAAA payload failed: %v (raw=%q)", err, combined.String())
	}
	if !bytesEqual(decoded, respPayload) {
		t.Fatalf("AAAA payload mismatch: got %q want %q", decoded, respPayload)
	}
}

// TestDNSFragmentMapCardinalityCapped ensures a flood of distinct (spoofable)
// agent-ID labels cannot grow the in-flight fragment map without bound. The
// listener must evict the oldest incomplete assembly once at capacity.
func TestDNSFragmentMapCardinalityCapped(t *testing.T) {
	dl := &DNSBeaconListener{
		frags: make(map[string]*dnsFragState),
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	label := enc.EncodeToString([]byte("abcd"))

	const flood = maxDNSFragments + 500
	for i := 0; i < flood; i++ {
		id := fmt.Sprintf("agent-%d", i)
		// total=2 with only idx=0 → stays in the map as incomplete.
		_, complete, ok := dl.collectFragment(id, "2_0", []string{label})
		if !ok {
			t.Fatalf("collectFragment rejected a valid fragment at i=%d", i)
		}
		if complete {
			t.Fatalf("unexpected completion at i=%d", i)
		}
	}
	if len(dl.frags) > maxDNSFragments {
		t.Fatalf("fragment map exceeded cap: got %d, want <= %d", len(dl.frags), maxDNSFragments)
	}
}