package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func mustURLReq(t *testing.T, raw string) *http.Request {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url %q: %v", raw, err)
	}
	return &http.Request{URL: u}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	return mustURLReq(t, raw).URL
}

// selfSignedLeaf returns a DER-encoded self-signed certificate plus the parsed
// form, standing in for the teamserver certificate a pin is generated from.
func selfSignedLeaf(t *testing.T, dnsName string) ([]byte, *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: dnsName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{dnsName},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return der, cert
}

func sha256Sum(b []byte) [32]byte { return sha256.Sum256(b) }

// withC2URLs installs a C2 URL list for the duration of a test.
func withC2URLs(t *testing.T, urls ...string) {
	t.Helper()
	prevURLs := c2URLsSnapshot()
	prevIdx := currentC2Idx.Load()
	c2URLsStore(urls, 0)
	t.Cleanup(func() {
		c2URLsStore(prevURLs, prevIdx)
	})
}

// TestC2URLListSecureOnly proves the downgrade guard keys off the configured
// URL list: an all-TLS implant is secure-only, any cleartext entry (or a bare
// host:port) keeps lab-style failover available.
func TestC2URLListSecureOnly(t *testing.T) {
	cases := []struct {
		name   string
		urls   []string
		secure bool
	}{
		{"https only", []string{"https://c2.example:8443"}, true},
		{"https failover list", []string{"https://a:443", "https://b:8443"}, true},
		{"wss only", []string{"wss://c2.example/ws"}, true},
		{"grpcs only", []string{"grpcs://c2.example:443"}, true},
		{"quic only", []string{"quic://c2.example:4433"}, true},
		{"ssh only", []string{"ssh://c2.example:2222"}, true},
		{"mixed tls schemes", []string{"https://a:443", "quic://b:4433"}, true},
		{"https + http", []string{"https://a:443", "http://b:80"}, false},
		{"http only", []string{"http://c2.example"}, false},
		{"ws cleartext", []string{"ws://c2.example/ws"}, false},
		{"h2c", []string{"h2c://c2.example"}, false},
		{"tcp cleartext", []string{"tcp://c2.example:4444"}, false},
		{"dns config", []string{"c2.example:53"}, false},
		{"empty", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withC2URLs(t, tc.urls...)
			if got := c2URLListSecureOnly(); got != tc.secure {
				t.Fatalf("c2URLListSecureOnly() = %v, want %v (urls=%v)", got, tc.secure, tc.urls)
			}
		})
	}
}

// TestSecureOnlyTransportGating proves failover candidates never include a
// cleartext transport for an all-TLS implant, while a lab implant keeps them.
func TestSecureOnlyTransportGating(t *testing.T) {
	withC2URLs(t, "https://c2.example:8443")
	for _, clear := range []string{"tcp", "udp", "dns", "icmp", "h2c"} {
		if c2SecureOnlyTransportAllowed(clear) {
			t.Errorf("secure-only implant allowed cleartext transport %q", clear)
		}
	}
	if !c2SecureOnlyTransportAllowed("http") {
		t.Error("secure-only implant with an https URL must keep the HTTP-family transport")
	}
	if !c2SecureOnlyTransportAllowed("quic") {
		t.Error("quic is TLS and must remain allowed")
	}

	// Lab implant (cleartext configured) keeps every transport.
	withC2URLs(t, "http://c2.example:8000")
	for _, name := range []string{"tcp", "udp", "dns", "icmp", "h2c", "http"} {
		if !c2SecureOnlyTransportAllowed(name) {
			t.Errorf("lab implant lost transport %q", name)
		}
	}
}

// TestApplyTransportRefusesCleartextWhenSecureOnly proves the switch itself is
// gated, not just candidate construction: a cleartext rotation request is a
// no-op for an all-TLS implant.
func TestApplyTransportRefusesCleartextWhenSecureOnly(t *testing.T) {
	withC2URLs(t, "https://c2.example:8443")
	setProtocolAndTransport("https", "http")
	applyTransport("tcp")
	if got := getProtocol(); got == "tcp" {
		t.Fatal("applyTransport switched a secure-only implant to tcp://")
	}

	withC2URLs(t, "http://c2.example:8000")
	applyTransport("tcp")
	if got := getProtocol(); got != "tcp" {
		t.Fatalf("lab implant did not rotate to tcp: protocol=%q", got)
	}
	setProtocolAndTransport("http", "http")
}

// TestRejectCleartextRedirect proves an https beacon refuses a redirect to
// http:// while still allowing https hops.
func TestRejectCleartextRedirect(t *testing.T) {
	via := []*http.Request{{URL: mustURL(t, "https://c2.example/collect")}}

	if err := rejectCleartextRedirect(mustURLReq(t, "https://other.example/collect"), via); err != nil {
		t.Fatalf("https->https redirect refused: %v", err)
	}
	err := rejectCleartextRedirect(mustURLReq(t, "http://c2.example/collect"), via)
	if err == nil {
		t.Fatal("https->http downgrade accepted")
	}
	if !strings.Contains(err.Error(), "refusing cleartext redirect") {
		t.Fatalf("unexpected error: %v", err)
	}
	// A cleartext origin keeps lab behaviour (no downgrade to prevent).
	viaPlain := []*http.Request{{URL: mustURLReq(t, "http://lab.example/collect").URL}}
	if err := rejectCleartextRedirect(mustURLReq(t, "http://lab.example/x"), viaPlain); err != nil {
		t.Fatalf("lab redirect refused: %v", err)
	}
}

// TestPinnedCertVerification proves pin verification enforces the hash and,
// when a server name is known, the hostname — and that a pin now actually
// authenticates a self-signed certificate (the stdlib chain check can no
// longer reject it before the pin is consulted).
func TestPinnedCertVerification(t *testing.T) {
	origPin := pinnedCertSHA256
	t.Cleanup(func() { pinnedCertSHA256 = origPin })

	der, cert := selfSignedLeaf(t, "c2.example")
	sum := sha256Sum(der)
	pinnedCertSHA256 = sum[:]

	// Matching pin + matching hostname passes.
	if err := verifyPinnedCert([][]byte{der}, "c2.example"); err != nil {
		t.Fatalf("valid pin rejected: %v", err)
	}
	// Hostname mismatch fails.
	if err := verifyPinnedCert([][]byte{der}, "other.example"); err == nil {
		t.Fatal("pin accepted for the wrong hostname")
	}
	// Wrong pin fails.
	pinnedCertSHA256 = make([]byte, 32)
	if err := verifyPinnedCert([][]byte{der}, "c2.example"); err == nil {
		t.Fatal("mismatched pin accepted")
	}
	// No certificate fails.
	pinnedCertSHA256 = sum[:]
	if err := verifyPinnedCert(nil, "c2.example"); err == nil {
		t.Fatal("missing certificate accepted")
	}
	_ = cert
}

// TestNewAgentTLSConfigPinReplacesChainValidation proves that with a pin the
// config no longer depends on the (failing) default chain verification.
func TestNewAgentTLSConfigPinReplacesChainValidation(t *testing.T) {
	origPin := pinnedCertSHA256
	origSkip := SkipTLSVerify
	t.Cleanup(func() { pinnedCertSHA256 = origPin; SkipTLSVerify = origSkip })

	SkipTLSVerify = false
	der, _ := selfSignedLeaf(t, "c2.example")
	sum := sha256Sum(der)
	pinnedCertSHA256 = sum[:]

	cfg := newAgentTLSConfig("c2.example")
	if !cfg.InsecureSkipVerify {
		t.Fatal("pinned config must skip default chain verification")
	}
	if cfg.VerifyPeerCertificate == nil {
		t.Fatal("pinned config must install a verifier")
	}
	if err := cfg.VerifyPeerCertificate([][]byte{der}, nil); err != nil {
		t.Fatalf("self-signed pinned certificate rejected: %v", err)
	}
	// Without a pin the stdlib verification path is untouched.
	pinnedCertSHA256 = nil
	cfg = newAgentTLSConfig("c2.example")
	if cfg.InsecureSkipVerify != SkipTLSVerify {
		t.Fatal("unpinned config must keep default verification behaviour")
	}
	if cfg.VerifyPeerCertificate != nil {
		t.Fatal("unpinned config must not install a pin verifier")
	}
}

// TestParseDNSResponseRejectsMisdirectedAnswers proves the DNS parser now
// refuses replies that are not answers, carry an error rcode, belong to a
// different transaction, or echo a different question.
func TestParseDNSResponseRejectsMisdirectedAnswers(t *testing.T) {
	qname := "abc.example.com"
	qtype := uint16(16)
	payload := []byte("hello-c2")

	query := buildDNSQuery(qname, qtype)
	txid := binary.BigEndian.Uint16(query[0:2])

	good := dnsResponseWithTXT(t, txid, qname, qtype, payload)
	if got := parseDNSResponse(good, qtype, txid, qname); string(got) != string(payload) {
		t.Fatalf("valid response rejected (got %q)", got)
	}

	// Different transaction id (cache poisoning / delayed answer).
	if got := parseDNSResponse(good, qtype, txid^0xFFFF, qname); got != nil {
		t.Fatalf("mismatched txid accepted: %q", got)
	}

	// Not a response (QR clear).
	notResponse := append([]byte(nil), good...)
	notResponse[2] &^= 0x80
	if got := parseDNSResponse(notResponse, qtype, txid, qname); got != nil {
		t.Fatalf("query packet accepted as a response: %q", got)
	}

	// Error rcode (NXDOMAIN) with a TXT answer still present.
	errRcode := append([]byte(nil), good...)
	errRcode[3] = 3
	if got := parseDNSResponse(errRcode, qtype, txid, qname); got != nil {
		t.Fatalf("NXDOMAIN response accepted: %q", got)
	}

	// Different question name.
	other := dnsResponseWithTXT(t, txid, "other.example.net", qtype, payload)
	if got := parseDNSResponse(other, qtype, txid, qname); got != nil {
		t.Fatalf("response for another qname accepted: %q", got)
	}

	// Different qtype echoed in the question.
	wrongType := dnsResponseWithTXT(t, txid, qname, 28, payload)
	if got := parseDNSResponse(wrongType, qtype, txid, qname); got != nil {
		t.Fatalf("response for another qtype accepted: %q", got)
	}
}

// dnsResponseWithTXT builds a minimal NOERROR response with one TXT record.
// DNS tunnelling carries base64 text in the record, so the payload is encoded
// the same way the real server does.
func dnsResponseWithTXT(t *testing.T, txid uint16, qname string, qtype uint16, payload []byte) []byte {
	t.Helper()
	txt := []byte(base64.StdEncoding.EncodeToString(payload))
	var pkt []byte
	hdr := make([]byte, 12)
	binary.BigEndian.PutUint16(hdr[0:2], txid)
	hdr[2] = 0x81                           // QR=1, RD=1
	hdr[3] = 0x80                           // RA=1, rcode=0
	binary.BigEndian.PutUint16(hdr[4:6], 1) // QDCOUNT
	binary.BigEndian.PutUint16(hdr[6:8], 1) // ANCOUNT
	pkt = append(pkt, hdr...)

	pkt = append(pkt, encodeDNSName(qname)...)
	qtypeBytes := make([]byte, 4)
	binary.BigEndian.PutUint16(qtypeBytes[0:2], qtype)
	binary.BigEndian.PutUint16(qtypeBytes[2:4], 1) // IN
	pkt = append(pkt, qtypeBytes...)

	// Answer: same name, TXT, one length-prefixed chunk.
	pkt = append(pkt, encodeDNSName(qname)...)
	rr := make([]byte, 10)
	binary.BigEndian.PutUint16(rr[0:2], 16) // TXT
	binary.BigEndian.PutUint16(rr[2:4], 1)  // IN
	binary.BigEndian.PutUint32(rr[4:8], 60) // TTL
	binary.BigEndian.PutUint16(rr[8:10], uint16(len(txt)+1))
	pkt = append(pkt, rr...)
	pkt = append(pkt, byte(len(txt)))
	pkt = append(pkt, txt...)
	return pkt
}
