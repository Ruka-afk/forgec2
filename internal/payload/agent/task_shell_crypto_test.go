package main

import (
	"runtime"
	"testing"
)

func TestShellSessionChaCha20RoundTrip(t *testing.T) {
	server, err := newECDSession()
	if err != nil {
		t.Fatalf("server session: %v", err)
	}
	client, err := newECDSession()
	if err != nil {
		t.Fatalf("client session: %v", err)
	}
	server.setSuite("chacha20")
	client.setSuite("chacha20")
	if err := client.establishFromServerKey(server.publicKeyB64()); err != nil {
		t.Fatalf("client establish: %v", err)
	}
	if err := server.establishFromServerKey(client.publicKeyB64()); err != nil {
		t.Fatalf("server establish: %v", err)
	}
	aad := []byte("agent-x\x0042")
	ct, err := server.encryptAESGCMWithAAD([]byte("chacha-whoami"), aad)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	pt, err := client.decryptAESGCMWithAAD(ct, aad)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(pt) != "chacha-whoami" {
		t.Fatalf("plaintext = %q", pt)
	}
}

func TestShellSessionSuiteMismatchFails(t *testing.T) {
	server, err := newECDSession()
	if err != nil {
		t.Fatalf("server session: %v", err)
	}
	client, err := newECDSession()
	if err != nil {
		t.Fatalf("client session: %v", err)
	}
	server.setSuite("chacha20")
	client.setSuite("")
	if err := client.establishFromServerKey(server.publicKeyB64()); err != nil {
		t.Fatalf("client establish: %v", err)
	}
	if err := server.establishFromServerKey(client.publicKeyB64()); err != nil {
		t.Fatalf("server establish: %v", err)
	}
	aad := []byte("agent-x\x001")
	ct, err := server.encryptAESGCMWithAAD([]byte("mismatch"), aad)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := client.decryptAESGCMWithAAD(ct, aad); err == nil {
		t.Fatal("expected decrypt failure under suite mismatch")
	}
}

// TestExecuteTaskDecryptsShell proves token_make's password (carried in the
// Shell field) is decrypted in executeTask like Command/Data. A decrypt
// failure surfaces as "task payload decryption failed"; reaching the handler
// (with its Windows-only guard on non-Windows) proves the field opened.
func TestExecuteTaskDecryptsShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("token_make executes LogonUser on Windows; covered on CI runners")
	}
	prevUUID := agentUUID
	prevSess := ecdhSess
	defer func() {
		agentUUID = prevUUID
		ecdhSess = prevSess
	}()

	agentUUID = "agent-shell-crypto"
	client, err := newECDSession()
	if err != nil {
		t.Fatalf("newECDSession: %v", err)
	}
	server, err := newECDSession()
	if err != nil {
		t.Fatalf("newECDSession: %v", err)
	}
	if err := client.establishFromServerKey(server.publicKeyB64()); err != nil {
		t.Fatalf("client establish: %v", err)
	}
	if err := server.establishFromServerKey(client.publicKeyB64()); err != nil {
		t.Fatalf("server establish: %v", err)
	}
	ecdhSess = client

	aad := []byte(agentUUID + "\x00" + "7")
	enc := func(pt string) string {
		ct, err := server.encryptAESGCMWithAAD([]byte(pt), aad)
		if err != nil {
			t.Fatalf("encrypt: %v", err)
		}
		return ct
	}

	task := Task{
		ID:        7,
		Type:      "token_make",
		Encrypted: true,
		Command:   enc("CORP\\jsmith"),
		Shell:     enc("S3cretP@ss!"),
		Path:      "2",
	}
	res := executeTask(task)
	if res.Error == "task payload decryption failed" {
		t.Fatalf("Shell payload was not decrypted: %v", res.Error)
	}
	if res.Error != "token ops only on Windows" {
		t.Fatalf("unexpected executeTask result: %q", res.Error)
	}
}

// TestExecuteTaskDecryptsShellFailure rejects a tampered Shell with the same
// failure the Command/Data paths produce.
func TestExecuteTaskDecryptsShellFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows runners execute the real handler; use non-windows CI")
	}
	prevUUID := agentUUID
	prevSess := ecdhSess
	defer func() {
		agentUUID = prevUUID
		ecdhSess = prevSess
	}()

	agentUUID = "agent-shell-crypto-bad"
	client, err := newECDSession()
	if err != nil {
		t.Fatalf("newECDSession: %v", err)
	}
	server, err := newECDSession()
	if err != nil {
		t.Fatalf("newECDSession: %v", err)
	}
	if err := client.establishFromServerKey(server.publicKeyB64()); err != nil {
		t.Fatalf("client establish: %v", err)
	}
	if err := server.establishFromServerKey(client.publicKeyB64()); err != nil {
		t.Fatalf("server establish: %v", err)
	}
	ecdhSess = client

	aad := []byte(agentUUID + "\x00" + "7")
	ct, err := server.encryptAESGCMWithAAD([]byte("real-password"), aad)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	task := Task{ID: 7, Type: "token_make", Encrypted: true, Command: ct, Shell: ct + "tampered"}
	if res := executeTask(task); res.Error != "task payload decryption failed" {
		t.Fatalf("tampered Shell must fail decryption, got %q", res.Error)
	}
}

// TestExecuteTaskPlainShellInterpreter allows Encrypted shell tasks whose
// interpreter was left in the clear (the historical teamserver behaviour).
func TestExecuteTaskPlainShellInterpreter(t *testing.T) {
	prevUUID := agentUUID
	prevSess := ecdhSess
	defer func() {
		agentUUID = prevUUID
		ecdhSess = prevSess
	}()

	agentUUID = "agent-shell-plain-interp"
	client, err := newECDSession()
	if err != nil {
		t.Fatalf("newECDSession: %v", err)
	}
	server, err := newECDSession()
	if err != nil {
		t.Fatalf("newECDSession: %v", err)
	}
	if err := client.establishFromServerKey(server.publicKeyB64()); err != nil {
		t.Fatalf("client establish: %v", err)
	}
	if err := server.establishFromServerKey(client.publicKeyB64()); err != nil {
		t.Fatalf("server establish: %v", err)
	}
	ecdhSess = client

	aad := []byte(agentUUID + "\x00" + "9")
	ct, err := server.encryptAESGCMWithAAD([]byte("whoami"), aad)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	res := executeTask(Task{
		ID:        9,
		Type:      "shell",
		Encrypted: true,
		Command:   ct,
		Shell:     "cmd.exe",
	})
	if res.Error == "task payload decryption failed" {
		t.Fatal("plaintext shell interpreter must not fail payload decrypt")
	}
}
