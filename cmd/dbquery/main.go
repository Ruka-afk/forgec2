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
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		fmt.Println("query tables:", err)
		os.Exit(1)
	}
	var tables []string
	for rows.Next() {
		var t string
		rows.Scan(&t)
		tables = append(tables, t)
	}
	rows.Close()
	fmt.Println("tables:", tables)

	cols, err := db.Query(`PRAGMA table_info(listeners)`)
	if err != nil {
		fmt.Println("pragma:", err)
		return
	}
	var names []string
	for cols.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		cols.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		names = append(names, name)
	}
	cols.Close()
	fmt.Println("listener cols:", names)

	rows2, err := db.Query(`SELECT * FROM listeners`)
	if err != nil {
		fmt.Println("query listeners:", err)
		return
	}
	dest := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range dest {
		ptrs[i] = &dest[i]
	}
	for rows2.Next() {
		if err := rows2.Scan(ptrs...); err != nil {
			fmt.Println("scan:", err)
			return
		}
		fmt.Println("---")
		for i, n := range names {
			fmt.Printf("  %s = %v\n", n, dest[i])
		}
	}
	rows2.Close()

	dump := func(table, cols string) {
		fmt.Printf("=== %s (%s) ===\n", table, cols)
		rs, err := db.Query(`SELECT ` + cols + ` FROM ` + table)
		if err != nil {
			fmt.Println("  err:", err)
			return
		}
		defer rs.Close()
		cnt := 0
		for rs.Next() {
			var id int
			var rest string
			if err := rs.Scan(&id, &rest); err != nil {
				fmt.Println("  scan:", err)
				return
			}
			fmt.Printf("  %d: %s\n", id, rest)
			cnt++
		}
		if cnt == 0 {
			fmt.Println("  (empty)")
		}
	}
	dump("users", "id, username")
	dump("api_keys", "id, name")
	dump("user_sessions", "id, user_id")

	cols2, err := db.Query(`PRAGMA table_info(reg_secrets)`)
	if err == nil {
		var names2 []string
		for cols2.Next() {
			var cid int
			var name, ctype string
			var notnull int
			var dflt sql.NullString
			var pk int
			cols2.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
			names2 = append(names2, name)
		}
		cols2.Close()
		fmt.Printf("reg_secrets cols: %v\n", names2)
		rs, err := db.Query(`SELECT * FROM reg_secrets`)
		if err == nil {
			dest := make([]any, len(names2))
			ptrs := make([]any, len(names2))
			for i := range dest {
				ptrs[i] = &dest[i]
			}
			for rs.Next() {
				if err := rs.Scan(ptrs...); err != nil {
					fmt.Println("  scan:", err)
					break
				}
				for i, n := range names2 {
					fmt.Printf("  %s = %v\n", n, dest[i])
				}
				fmt.Println("  ---")
			}
			rs.Close()
		}
	}
}
