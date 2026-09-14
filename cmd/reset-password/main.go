// Command reset-password updates a user's password in the ForgeC2 database.
// Usage: go run ./cmd/reset-password -db data/db/forgec2.db -user labtest -pass newpassword
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", filepath.Join("data", "db", "forgec2.db"), "sqlite database file")
	username := flag.String("user", "", "username to reset password for (required)")
	password := flag.String("pass", "", "new password (required)")
	flag.Parse()

	if *username == "" || *password == "" {
		fmt.Fprintf(os.Stderr, "Usage: reset-password -db <path> -user <username> -pass <password>\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if _, err := os.Stat(*dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "db not found: %s\n", *dbPath)
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bcrypt hash failed: %v\n", err)
		os.Exit(1)
	}

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	res, err := db.Exec("UPDATE users SET password_hash = ? WHERE username = ?", string(hash), *username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
		os.Exit(1)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rows affected: %v\n", err)
		os.Exit(1)
	}
	if rows == 0 {
		fmt.Fprintf(os.Stderr, "user not found: %s\n", *username)
		os.Exit(1)
	}

	fmt.Printf("Password updated for user: %s (rows affected: %d)\n", *username, rows)
}
