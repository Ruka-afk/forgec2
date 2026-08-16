package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", os.Args[1])
	if err != nil {
		fmt.Println("open:", err)
		os.Exit(1)
	}
	defer db.Close()
	res, err := db.Exec(os.Args[2])
	if err != nil {
		fmt.Println("exec:", err)
		os.Exit(1)
	}
	n, _ := res.RowsAffected()
	fmt.Printf("ok, rows affected: %d\n", n)
}
