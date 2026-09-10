// Command db-reset prepares a release database: it snapshots the sqlite file,
// clears operational tables (tasks, implants/sessions, audit logs, alerts,
// notifications, AI runs/intents) while preserving configuration
// (listeners, users, roles, webhook/notification routes, profiles, macros),
// resets autoincrement sequences and vacuums.
//
// Usage: go run ./cmd/db-reset -db data/db/forgec2.db [-backup-dir data/db/backup] [-no-backup] [-yes]
//
// The server must be stopped first: an open writer holds locks and in-memory
// state would repopulate the UI until restart.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// clearedTables are wiped on reset. Configuration tables (users, roles,
// listeners, webhook_configs, notification_routes, server_configs,
// ai_provider_profiles, command_macros, workflows, scripts, plugins) stay.
var clearedTables = []string{
	"tasks",
	"implants",
	"audit_logs",
	"alerts",
	"notifications",
	"ai_chat_runs",
	"ai_chat_run_events",
	"ai_chat_messages",
	"ai_execution_intents",
}

func main() {
	dbPath := flag.String("db", filepath.Join("data", "db", "forgec2.db"), "sqlite database file")
	backupDir := flag.String("backup-dir", "", "backup directory (default <db-dir>/backup)")
	noBackup := flag.Bool("no-backup", false, "skip pre-reset backup (not recommended)")
	yes := flag.Bool("yes", false, "skip confirmation prompt")
	flag.Parse()

	if _, err := os.Stat(*dbPath); err != nil {
		fmt.Println("db not found:", *dbPath)
		os.Exit(1)
	}
	dir := *backupDir
	if dir == "" {
		dir = filepath.Join(filepath.Dir(*dbPath), "backup")
	}

	if !*yes {
		fmt.Printf("Reset %s? Operational data will be cleared (config preserved). Type YES to continue: ", *dbPath)
		var answer string
		if _, err := fmt.Scanln(&answer); err != nil || answer != "YES" {
			fmt.Println("aborted")
			return
		}
	}

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		fmt.Println("open:", err)
		os.Exit(1)
	}
	defer db.Close()

	if !*noBackup {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			fmt.Println("backup dir:", err)
			os.Exit(1)
		}
		snap := filepath.Join(dir, "forgec2-pre-reset-"+time.Now().Format("20060102-150405")+".db")
		if _, err := db.Exec(fmt.Sprintf("VACUUM INTO '%s'", snap)); err != nil {
			fmt.Println("backup failed (is the server still running?):", err)
			os.Exit(1)
		}
		fmt.Println("backup:", snap)
		pruneBackups(dir, 20)
	}

	tx, err := db.Begin()
	if err != nil {
		fmt.Println("begin:", err)
		os.Exit(1)
	}
	defer tx.Rollback()
	for _, table := range clearedTables {
		var name string
		if err := tx.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			continue // table absent in this schema version
		}
		if _, err := tx.Exec(fmt.Sprintf("DELETE FROM %q", table)); err != nil {
			fmt.Printf("clear %s: %v\n", table, err)
			os.Exit(1)
		}
		if _, err := tx.Exec("DELETE FROM sqlite_sequence WHERE name=?", table); err != nil {
			fmt.Printf("reset seq %s: %v\n", table, err)
			os.Exit(1)
		}
		fmt.Println("cleared:", table)
	}
	if err := tx.Commit(); err != nil {
		fmt.Println("commit:", err)
		os.Exit(1)
	}
	if _, err := db.Exec("VACUUM"); err != nil {
		fmt.Println("vacuum:", err)
		os.Exit(1)
	}
	fmt.Println("reset complete:", *dbPath)
}

// pruneBackups keeps the newest keep snapshots, deleting older ones.
func pruneBackups(dir string, keep int) {
	entries, err := filepath.Glob(filepath.Join(dir, "forgec2-pre-reset-*.db"))
	if err != nil || len(entries) <= keep {
		return
	}
	for _, old := range entries[:len(entries)-keep] {
		_ = os.Remove(old)
	}
}
