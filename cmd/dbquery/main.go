package main

import (
	"fmt"
	"os"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	dbPath := "data/db/forgec2.db"

	if len(os.Args) < 2 {
		// No args: list users
		listUsers(dbPath)
		return
	}

	arg1 := os.Args[1]
	if arg1 == "--help" || arg1 == "-h" {
		fmt.Println("Usage:")
		fmt.Println("  go run ./cmd/dbquery/                    - list all users")
		fmt.Println("  go run ./cmd/dbquery/ <db_path>          - list users in specific DB")
		fmt.Println("  go run ./cmd/dbquery/ <username> <pass>  - reset password")
		return
	}

	if len(os.Args) == 2 {
		// One arg: could be dbPath or username without password
		// If it ends with .db, treat as dbPath
		if len(arg1) > 3 && arg1[len(arg1)-3:] == ".db" {
			listUsers(arg1)
			return
		}
		// Otherwise it's a username - show error
		fmt.Fprintf(os.Stderr, "Missing password. Usage: go run ./cmd/dbquery/ <username> <new_password>\n")
		os.Exit(1)
	}

	// Two or more args: username + password
	username := os.Args[1]
	newPassword := os.Args[2]

	if len(newPassword) < 4 {
		fmt.Fprintln(os.Stderr, "Password must be at least 4 characters")
		os.Exit(1)
	}

	resetPassword(dbPath, username, newPassword)
}

func listUsers(dbPath string) {
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "DB open error: %v\n", err)
		os.Exit(1)
	}

	var users []db.User
	database.Find(&users)

	if len(users) == 0 {
		fmt.Println("No users in database!")
		return
	}

	fmt.Printf("Users in %s:\n", dbPath)
	for _, u := range users {
		hashStatus := "SET"
		if u.PasswordHash == "" {
			hashStatus = "EMPTY (first password becomes the password)"
		}
		fmt.Printf("  ID=%d  Username=%-20s  Role=%-10s  Active=%v  Password=%s\n",
			u.ID, u.Username, u.Role, u.IsActive, hashStatus)
	}
}

func resetPassword(dbPath, username, newPassword string) {
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "DB open error: %v\n", err)
		os.Exit(1)
	}

	var user db.User
	result := database.Where("username = ?", username).First(&user)
	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "User '%s' not found in database\n", username)
		os.Exit(1)
	}

	hash, err := middleware.HashPassword(newPassword)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hash error: %v\n", err)
		os.Exit(1)
	}

	database.Model(&user).Update("password_hash", hash)
	fmt.Printf("Password for '%s' has been reset successfully!\n", username)
	fmt.Printf("You can now log in with username='%s' and your new password.\n", username)
}
