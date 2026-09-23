package server

// Additional DNS listener contract tests. The fragmentation happy path,
// malformed-meta rejection, AAAA tunneling and fragment-map caps already live
// in dns_fragment_test.go; this file covers the legacy single-shot branch,
// the A-stub vs tunneled-A distinction, non-beacon queries, the XOR obscure
// round-trip, and the base32 decoder vectors. Helpers fragQName and
// dnsTestWriter are reused from dns_fragment_test.go (same package).

import (
	"encoding/base32"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

func testDNSListener() *DNSBeaconListener {
	dl := NewDNSBeaconListener("c2.example.test", "127.0.0.1", 0, ":0")
	dl.SetHandler(func(agentID string, req []byte) []byte {
		return []byte(`{"ok":true,"agent":"` + agentID + `"}`)
	})
	return dl
}

func askTXT(t *testing.T, dl *DNSBeaconListener, qname string) *dns.Msg {
	t.Helper()
	if !strings.HasSuffix(qname, ".") {
		qname += "."
	}
	r := new(dns.Msg)
	r.SetQuestion(qname, dns.TypeTXT)
	w := &dnsTestWriter{}
	dl.handleQuery(w, r)
	if len(w.msgs) != 1 || w.msgs[0] == nil {
		t.Fatalf("expected exactly 1 reply for %q, got %+v", qname, w.msgs)
	}
	return w.msgs[0]
}

func txtConcat(t *testing.T, m *dns.Msg) string {
	t.Helper()
	var sb strings.Builder
	for _, rr := range m.Answer {
		txt, ok := rr.(*dns.TXT)
		if !ok {
			t.Fatalf("answer is %T, want *dns.TXT", rr)
		}
		sb.WriteString(strings.Join(txt.Txt, ""))
	}
	return sb.String()
}

// TestDNSLegacySingleShot covers the no-metadata branch: data labels with no
// "<total>_<index>" label are decoded as one unfragmented base32 payload.
func TestDNSLegacySingleShot(t *testing.T) {
	dl := testDNSListener()
	defer dl.Stop()
	uuidHex := "00112233445566778899aabbccddeeff"
	payload := []byte(`{"uuid":"` + uuidHex + `","seq":7}`)
	enc := base32.StdEncoding.EncodeToString(payload)
	enc = strings.TrimRight(enc, "=")
	var labels []string
	for j := 0; j < len(enc); j += 63 {
		e := j + 63
		if e > len(enc) {
			e = len(enc)
		}
		labels = append(labels, enc[j:e])
	}
	qname := uuidHex + "." + strings.Join(labels, ".") + ".dns.c2.example.test"

	raw, err := base64.StdEncoding.DecodeString(txtConcat(t, askTXT(t, dl, qname)))
	if err != nil {
		t.Fatalf("response is not base64: %v", err)
	}
	if !strings.Contains(string(raw), `"ok":true`) {
		t.Fatalf("unexpected legacy response: %s", raw)
	}
}

// TestDNSNonBeaconTXTBlank verifies a TXT query without the ".dns." beacon
// infix gets a blank record and never reaches the beacon handler.
func TestDNSNonBeaconTXTBlank(t *testing.T) {
	dl := testDNSListener()
	defer dl.Stop()
	called := false
	dl.SetHandler(func(string, []byte) []byte { called = true; return []byte("X") })

	if got := txtConcat(t, askTXT(t, dl, "www.c2.example.test")); strings.TrimSpace(got) != "" {
		t.Fatalf("non-beacon query must get a blank record, got %q", got)
	}
	if called {
		t.Fatal("non-beacon query must not reach the beacon handler")
	}
}

// TestDNSAStubAndBeacon distinguishes stub resolution (plain A under the
// domain answers the configured AgentIP) from tunneled C2 (A at the beacon
// subdomain carries a base64 payload in 4-byte rdata chunks).
func TestDNSAStubAndBeacon(t *testing.T) {
	dl := testDNSListener()
	defer dl.Stop()

	ask := func(qname string) *dns.Msg {
		if !strings.HasSuffix(qname, ".") {
			qname += "."
		}
		r := new(dns.Msg)
		r.SetQuestion(qname, dns.TypeA)
		w := &dnsTestWriter{}
		dl.handleQuery(w, r)
		if len(w.msgs) != 1 || w.msgs[0] == nil {
			t.Fatalf("expected exactly 1 reply for %q", qname)
		}
		return w.msgs[0]
	}

	stub := ask("www.c2.example.test")
	if len(stub.Answer) != 1 {
		t.Fatalf("stub A query must get exactly 1 answer, got %d", len(stub.Answer))
	}
	if a, ok := stub.Answer[0].(*dns.A); !ok || a.A.String() != "127.0.0.1" {
		t.Fatalf("stub A answer wrong: %+v", stub.Answer[0])
	}

	uuidHex := "00112233445566778899aabbccddeeff"
	qname := fragQName("c2.example.test", uuidHex, 1, 0, []byte(`{"uuid":"`+uuidHex+`"}`))
	tun := ask(qname)
	if len(tun.Answer) == 0 {
		t.Fatal("beacon A query must get tunneled answers")
	}
	var combined strings.Builder
	for _, rr := range tun.Answer {
		a, ok := rr.(*dns.A)
		if !ok {
			t.Fatalf("tunneled answer is %T, want *dns.A", rr)
		}
		combined.WriteString(string(a.A.To4()))
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(combined.String()))
	if err != nil {
		t.Fatalf("tunneled A payload is not base64: %v (raw %q)", err, combined.String())
	}
	if !strings.Contains(string(raw), `"ok":true`) {
		t.Fatalf("unexpected tunneled response: %s", raw)
	}
}

// TestDNSObscureRoundTrip mirrors the Go agent (dns.go buildDNSQueryNames):
// XOR the fragment plaintext with the UUID key FIRST, then base32-encode.
// The server decodes base32 then XORs back; the base64 response is XORed too.
func TestDNSObscureRoundTrip(t *testing.T) {
	dl := testDNSListener()
	defer dl.Stop()
	dl.SetObscure(true)
	uuidHex := "deadbeefdeadbeefdeadbeefdeadbeef"
	payload := []byte(`{"uuid":"` + uuidHex + `","seq":3}`)

	obscured := xorBytesServer(payload, []byte(uuidHex))
	enc := base32.StdEncoding.EncodeToString(obscured)
	enc = strings.TrimRight(enc, "=")
	var labels []string
	for j := 0; j < len(enc); j += 63 {
		e := j + 63
		if e > len(enc) {
			e = len(enc)
		}
		labels = append(labels, enc[j:e])
	}
	qname := uuidHex + ".1_0." + strings.Join(labels, ".") + ".dns.c2.example.test"

	got := txtConcat(t, askTXT(t, dl, qname))
	// Wire order (dns_listener.go processBeacon) is base64(XOR(json)), so
	// decode first and de-obscure second — mirroring the Go agent's
	// parseDNSResponse + xorBytes(dnsObfuscateKey) path.
	decoded, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("obscured response is not base64: %v (raw %q)", err, got)
	}
	raw := xorBytesServer(decoded, []byte(uuidHex))
	if !strings.Contains(string(raw), `"ok":true`) {
		t.Fatalf("unexpected obscure response: %s", raw)
	}
}

func TestDNSDecodeBase32Vectors(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"MY", "f"},
		{"MFRGG===", "abc"},
		{"my", "f"}, // lowercased qname path stays decodable
	} {
		got, err := decodeBase32NoPad(tc.in)
		if err != nil {
			t.Fatalf("decodeBase32NoPad(%q) errored: %v", tc.in, err)
		}
		if string(got) != tc.want {
			t.Fatalf("decodeBase32NoPad(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if _, err := decodeBase32NoPad("!!!!"); err == nil {
		t.Fatal("garbage base32 must error")
	}
}
