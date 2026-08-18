package main

import (
	"encoding/hex"
	"strings"
	"testing"
)

// ---- minimal DER builders for the Kerberos fixtures ----

func derLen(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	var b [4]byte
	i := 3
	for n > 0 {
		b[i] = byte(n & 0xff)
		i--
		n >>= 8
	}
	num := 3 - i
	out := []byte{byte(0x80 | num)}
	return append(out, b[4-num:]...)
}

func wrap(tag byte, content []byte) []byte {
	out := []byte{tag}
	return append(append(out, derLen(len(content))...), content...)
}

func ctxEl(tag byte, content []byte) []byte { return wrap(tag, content) }

func encInt(v int) []byte { return []byte{0x02, 0x01, byte(v)} }

func octet(content []byte) []byte { return wrap(0x04, content) }

func generalString(s string) []byte { return wrap(0x1b, []byte(s)) }

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// fixtureTicket builds the Ticket ([APPLICATION 1]) with the given enc-part.
func fixtureTicket(etype int, cipher []byte) []byte {
	encPart := wrap(0x30, concat(
		ctxEl(0xa0, encInt(etype)),
		ctxEl(0xa1, encInt(5)),
		ctxEl(0xa2, octet(cipher))))
	sname := ctxEl(0xa2, wrap(0x30, concat(
		ctxEl(0xa0, encInt(2)),
		ctxEl(0xa1, wrap(0x30, generalString("svc_http@CORP.LOCAL"))))))
	// RFC 4120: [APPLICATION 1] replaces the SEQUENCE tag — no nested 0x30.
	return wrap(0x61, concat(
		ctxEl(0xa0, encInt(5)),
		ctxEl(0xa1, generalString("CORP.LOCAL")),
		sname,
		ctxEl(0xa3, encPart)))
}

// fixtureAPREQ wraps a ticket into an AP-REQ ([APPLICATION 1]).
func fixtureAPREQ(ticket []byte) []byte {
	authenticator := ctxEl(0xa4, wrap(0x30, concat(
		ctxEl(0xa0, encInt(18)),
		ctxEl(0xa2, octet([]byte{0xde, 0xad})))))
	return wrap(0x61, concat(
		ctxEl(0xa0, encInt(5)),
		ctxEl(0xa1, encInt(14)),
		ctxEl(0xa2, wrap(0x03, []byte{0x00})),
		ctxEl(0xa3, ticket),
		authenticator))
}

// fixtureSPNEGO wraps an AP-REQ the way some .NET builds serialize it.
func fixtureSPNEGO(apreq []byte) []byte {
	spnegoOID := []byte{0x06, 0x06, 0x2b, 0x06, 0x01, 0x05, 0x05, 0x02}
	return wrap(0x60, wrap(0x30, concat(
		spnegoOID,
		ctxEl(0xa0, wrap(0x30,
			ctxEl(0xa0, wrap(0x30,
				ctxEl(0x60, apreq))))))))
}

func TestConvertKerberoastLineRC4ToHashcat(t *testing.T) {
	cipher := make([]byte, 56)
	for i := range cipher {
		cipher[i] = byte(i)
	}
	apreq := fixtureAPREQ(fixtureTicket(23, cipher))

	line := convertKerberoastLine("svc_http@CORP.LOCAL\t" + hex.EncodeToString(apreq))
	want := "$krb5tgs$23$*svc_http$CORP.LOCAL$svc_http@CORP.LOCAL*$" +
		hex.EncodeToString(cipher[:16]) + "$" + hex.EncodeToString(cipher[16:])
	if line != want {
		t.Fatalf("convert mismatch:\n got: %s\nwant: %s", line, want)
	}

	// The produced line must be structurally what hashcat 13100 expects:
	// "$krb5tgs$23$*user$realm$spn*$<32-hex checksum>$<long edata2>".
	if !strings.HasPrefix(line, "$krb5tgs$23$*") || !strings.Contains(line, "*$") {
		t.Fatalf("hashcat structure prefix wrong: %q", line)
	}
	afterAcct := line[strings.Index(line, "*$")+2:]
	fields := strings.Split(afterAcct, "$")
	if len(fields) != 2 || len(fields[0]) != 32 {
		t.Fatalf("checksum/edata2 fields wrong: %v", fields)
	}
	if len(fields[1]) < 64 {
		t.Fatalf("edata2 field too short: %d", len(fields[1]))
	}
}

func TestConvertKerberoastLineSPNEGOWrapped(t *testing.T) {
	cipher := make([]byte, 56)
	for i := range cipher {
		cipher[i] = byte(i)
	}
	apreq := fixtureAPREQ(fixtureTicket(23, cipher))
	wrapped := fixtureSPNEGO(apreq)

	line := convertKerberoastLine("svc_http@CORP.LOCAL\t" + hex.EncodeToString(wrapped))
	want := "$krb5tgs$23$*svc_http$CORP.LOCAL$svc_http@CORP.LOCAL*$" +
		hex.EncodeToString(cipher[:16]) + "$" + hex.EncodeToString(cipher[16:])
	if line != want {
		t.Fatalf("SPNEGO convert mismatch:\n got: %s\nwant: %s", line, want)
	}

	// A plain scan must never pick a false positive ticket from the *primitive*
	// OID bytes — here the .NET OID happens to be the first 0x61-free 0x60 tree.
	if !strings.HasPrefix(line, "$krb5tgs$") {
		t.Fatalf("unexpected conversion result: %q", line)
	}

	// Toolchain tolerance: an AP-REQ wrapped by a redundant SEQUENCE (tag
	// replaced instead of removed) must still convert.
	innerSeq := findAPREQ(apreq)
	redundant := wrap(0x61, wrap(0x30, innerSeq))
	line2 := convertKerberoastLine("svc_http@CORP.LOCAL\t" + hex.EncodeToString(redundant))
	if line2 != want {
		t.Fatalf("redundant-seq mismatch:\n got: %s\nwant: %s", line2, want)
	}
}

func TestConvertKerberoastLineNonRC4FallsBackToLegacy(t *testing.T) {
	cipher := make([]byte, 56)
	for i := range cipher {
		cipher[i] = byte(i)
	}
	// etype 18 (AES256): not 13100-crackable → keep "UPN:hex" so the ingest
	// still records the artifact instead of dropping it.
	apreq := fixtureAPREQ(fixtureTicket(18, cipher))
	hexBlob := hex.EncodeToString(apreq)
	line := convertKerberoastLine("svc_http@CORP.LOCAL\t" + hexBlob)
	if line != "svc_http@CORP.LOCAL:"+hexBlob {
		t.Fatalf("etype-18 must degrade to legacy UPN:hex, got %q", line)
	}
}

func TestConvertKerberoastLineMalformedFallsBack(t *testing.T) {
	// Truncated DER: ticket cipher unresolvable → legacy passthrough.
	blob := "61" + "82" + "0100" // garbage length
	line := convertKerberoastLine("svc_http@CORP.LOCAL\t" + blob)
	if !strings.HasPrefix(line, "svc_http@CORP.LOCAL:") {
		t.Fatalf("malformed DER must degrade to legacy, got %q", line)
	}
	// Cipher too short to hold checksum+stream.
	apreq := fixtureAPREQ(fixtureTicket(23, []byte{1, 2, 3, 4}))
	blob2 := hex.EncodeToString(apreq)
	line2 := convertKerberoastLine("svc_http@CORP.LOCAL\t" + blob2)
	if !strings.HasPrefix(line2, "svc_http@CORP.LOCAL:") {
		t.Fatalf("short cipher must degrade to legacy, got %q", line2)
	}
	// Non-hex and legacy lines pass through untouched.
	if got := convertKerberoastLine("old/spn@C.L:abcd"); got != "old/spn@C.L:abcd" {
		t.Fatalf("legacy line must pass through, got %q", got)
	}
	if got := convertKerberoastLine("svc_http@CORP.LOCAL\tnot-hex!"); got != "svc_http@CORP.LOCAL\tnot-hex!" {
		t.Fatalf("non-hex tab line must pass through, got %q", got)
	}
}

func TestConvertKerberoastResultMixedLines(t *testing.T) {
	cipher := make([]byte, 56)
	for i := range cipher {
		cipher[i] = byte(i)
	}
	rc4 := "$krb5tgs$23$*svc_http$CORP.LOCAL$svc_http@CORP.LOCAL*$" +
		hex.EncodeToString(cipher[:16]) + "$" + hex.EncodeToString(cipher[16:])
	raw := "svc_http@CORP.LOCAL\t" + hex.EncodeToString(fixtureAPREQ(fixtureTicket(23, cipher))) + "\n" +
		"old/spn@C.L:feedbeef\n" +
		"\n"
	out := convertKerberoastResult(raw)
	lines := strings.Split(out, "\n")
	if lines[0] != rc4 {
		t.Fatalf("first line not converted:\n got: %s\nwant: %s", lines[0], rc4)
	}
	if lines[1] != "old/spn@C.L:feedbeef" {
		t.Fatalf("legacy line must survive, got %q", lines[1])
	}
}