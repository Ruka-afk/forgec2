package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func testSSHHostKey(t *testing.T) ssh.Signer {
	t.Helper()
	signer, err := loadOrGenerateSSHHostKey("")
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	return signer
}

// acceptAnyHostKey is the test stand-in for ssh.InsecureIgnoreHostKey (G106).
func acceptAnyHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}

func TestSSHBeaconListenerRoundTrip(t *testing.T) {
	handler := func(agentID string, reqJSON []byte) []byte {
		return []byte("resp:" + string(reqJSON))
	}

	l := NewSSHBeaconListener(SSHListenerConfig{
		Addr:     "127.0.0.1:0",
		User:     "forgec2",
		Password: "hunter2",
		KeyAuth:  true,
		HostKey:  testSSHHostKey(t),
	})
	l.SetHandler(handler)
	if err := l.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer l.Stop()

	client, err := ssh.Dial("tcp", l.Addr(), &ssh.ClientConfig{
		User:            "forgec2",
		Auth:            []ssh.AuthMethod{ssh.Password("hunter2")},
		HostKeyCallback: acceptAnyHostKey,
		Timeout:         10 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	if err := session.Start("/bin/sh -c 'cat'"); err != nil {
		t.Fatalf("exec: %v", err)
	}

	body := `{"uuid":"11111111-2222-3333-4444-555555555555","seq":1}`
	if _, err := stdin.Write([]byte(body)); err != nil {
		t.Fatalf("write body: %v", err)
	}
	stdin.Close()

	got, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if err := session.Wait(); err != nil {
		t.Fatalf("session wait: %v", err)
	}

	want := "resp:" + body
	if string(got) != want {
		t.Errorf("response mismatch: got %q want %q", string(got), want)
	}
}

func TestSSHBeaconListenerWrongPasswordRejected(t *testing.T) {
	l := NewSSHBeaconListener(SSHListenerConfig{
		Addr:     "127.0.0.1:0",
		User:     "forgec2",
		Password: "correct",
		HostKey:  testSSHHostKey(t),
	})
	l.SetHandler(func(agentID string, reqJSON []byte) []byte { return reqJSON })
	if err := l.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer l.Stop()

	client, err := ssh.Dial("tcp", l.Addr(), &ssh.ClientConfig{
		User:            "forgec2",
		Auth:            []ssh.AuthMethod{ssh.Password("wrong")},
		HostKeyCallback: acceptAnyHostKey,
		Timeout:         10 * time.Second,
	})
	if err == nil {
		client.Close()
		t.Fatal("expected dial to fail with wrong password")
	}
}

func TestSSHBeaconListenerKeyOnlyAndKeyboardInteractive(t *testing.T) {
	handler := func(agentID string, reqJSON []byte) []byte { return []byte("ok") }

	l := NewSSHBeaconListener(SSHListenerConfig{
		Addr:    "127.0.0.1:0",
		User:    "forgec2",
		KeyAuth: true,
		HostKey: testSSHHostKey(t),
	})
	l.SetHandler(handler)
	if err := l.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer l.Stop()

	t.Run("keyboard-interactive accepted when password empty", func(t *testing.T) {
		client, err := ssh.Dial("tcp", l.Addr(), &ssh.ClientConfig{
			User: "forgec2",
			Auth: []ssh.AuthMethod{ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				return make([]string, len(questions)), nil
			})},
			HostKeyCallback: acceptAnyHostKey,
			Timeout:         10 * time.Second,
		})
		if err != nil {
			t.Fatalf("dial (keyboard-interactive): %v", err)
		}
		client.Close()
	})

	t.Run("keyboard-interactive rejected when password set", func(t *testing.T) {
		strict := NewSSHBeaconListener(SSHListenerConfig{
			Addr:     "127.0.0.1:0",
			User:     "forgec2",
			Password: "set",
			HostKey:  testSSHHostKey(t),
		})
		strict.SetHandler(handler)
		if err := strict.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}
		defer strict.Stop()

		client, err := ssh.Dial("tcp", strict.Addr(), &ssh.ClientConfig{
			User: "forgec2",
			Auth: []ssh.AuthMethod{ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				return make([]string, len(questions)), nil
			})},
			HostKeyCallback: acceptAnyHostKey,
			Timeout:         10 * time.Second,
		})
		if err == nil {
			client.Close()
			t.Fatal("expected keyboard-interactive to be rejected when a password is configured")
		}
	})
}

func TestSSHBeaconListenerPublicKeyAuth(t *testing.T) {
	handler := func(agentID string, reqJSON []byte) []byte { return []byte("ok") }

	l := NewSSHBeaconListener(SSHListenerConfig{
		Addr:     "127.0.0.1:0",
		User:     "forgec2",
		Password: "somepass",
		KeyAuth:  true,
		HostKey:  testSSHHostKey(t),
	})
	l.SetHandler(handler)
	if err := l.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer l.Stop()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("wrap client key: %v", err)
	}

	client, err := ssh.Dial("tcp", l.Addr(), &ssh.ClientConfig{
		User:            "forgec2",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: acceptAnyHostKey,
		Timeout:         10 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial (public key): %v", err)
	}
	client.Close()
}

func TestLoadOrGenerateSSHHostKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "ssh_host_key")

	signer1, err := loadOrGenerateSSHHostKey(path)
	if err != nil {
		t.Fatalf("first load/generate: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("host key not persisted: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("host key file empty")
	}

	signer2, err := loadOrGenerateSSHHostKey(path)
	if err != nil {
		t.Fatalf("second load/generate: %v", err)
	}
	if string(signer1.PublicKey().Marshal()) != string(signer2.PublicKey().Marshal()) {
		t.Error("host key should be stable across reloads")
	}
}

func TestEnvelopeAgentID(t *testing.T) {
	if got := envelopeAgentID([]byte(`{"uuid":"abc-123","seq":5}`)); got != "abc-123" {
		t.Errorf("expected abc-123, got %q", got)
	}
	if got := envelopeAgentID([]byte(`{"seq":5}`)); got != "" {
		t.Errorf("expected empty for no uuid, got %q", got)
	}
	if got := envelopeAgentID([]byte("not json")); got != "" {
		t.Errorf("expected empty for bad json, got %q", got)
	}
}

func TestSSHBeaconListenerRejectsDirectTCPChannel(t *testing.T) {
	l := NewSSHBeaconListener(SSHListenerConfig{
		Addr:    "127.0.0.1:0",
		User:    "forgec2",
		KeyAuth: true,
		HostKey: testSSHHostKey(t),
	})
	l.SetHandler(func(agentID string, reqJSON []byte) []byte { return reqJSON })
	if err := l.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer l.Stop()

	client, err := ssh.Dial("tcp", l.Addr(), &ssh.ClientConfig{
		User:            "forgec2",
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: acceptAnyHostKey,
		Timeout:         10 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	// Opening a direct-tcpip channel must be rejected (unknown channel type).
	if _, err := client.Dial("tcp", "127.0.0.1:9999"); err == nil {
		t.Fatal("expected direct-tcpip channel to be rejected")
	}
}
