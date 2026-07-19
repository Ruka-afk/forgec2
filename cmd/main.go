package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	path := `..\..\internal\server\server.go`
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	// Find sub-group names
	groupRe := regexp.MustCompile(`^\t\t(\w+) := auth\.Group\("/"\)`)
	var groups []string
	for _, line := range lines {
		if m := groupRe.FindStringSubmatch(line); m != nil {
			groups = append(groups, m[1])
		}
	}

	// Build set of existing lines for dedup
	existing := make(map[string]bool)
	for _, line := range lines {
		existing[line] = true
	}

	// Collect insertions: (insertAfterIndex, newLine)
	type insert struct {
		after int
		line  string
	}
	var inserts []insert

	// Auth routes (2 tabs)
	authRe := regexp.MustCompile(`^\t\tauth\.(GET|POST|PUT|DELETE)\("/api/(.+?)", (.+?)\)$`)
	for i, line := range lines {
		if m := authRe.FindStringSubmatch(line); m != nil {
			method, apiPath, handler := m[1], m[2], m[3]
			nonApiLine := fmt.Sprintf("\t\tauth.%s(\"/%s\", %s)", method, apiPath, handler)
			if !existing[nonApiLine] {
				inserts = append(inserts, insert{after: i, line: nonApiLine})
			}
		}
	}

	// Sub-group routes (3 tabs)
	for _, g := range groups {
		subRe := regexp.MustCompile(fmt.Sprintf(`^\t\t\t%s\.(GET|POST|PUT|DELETE)\("/api/(.+?)", (.+?)\)$`, regexp.QuoteMeta(g)))
		for i, line := range lines {
			if m := subRe.FindStringSubmatch(line); m != nil {
				method, apiPath, handler := m[1], m[2], m[3]
				nonApiLine := fmt.Sprintf("\t\t\t%s.%s(\"/%s\", %s)", g, method, apiPath, handler)
				if !existing[nonApiLine] {
					inserts = append(inserts, insert{after: i, line: nonApiLine})
				}
			}
		}
	}

	// Sort by index descending
	for i := 0; i < len(inserts); i++ {
		for j := i + 1; j < len(inserts); j++ {
			if inserts[j].after > inserts[i].after {
				inserts[i], inserts[j] = inserts[j], inserts[i]
			}
		}
	}

	// Build new output
	var out []string
	nextIns := 0
	for i, line := range lines {
		out = append(out, line)
		for nextIns < len(inserts) && inserts[nextIns].after == i {
			out = append(out, inserts[nextIns].line)
			nextIns++
		}
	}

	result := strings.Join(out, "\n")
	result = strings.ReplaceAll(result, "\n", "\r\n")
	if err := os.WriteFile(path, []byte(result), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Inserted %d missing non-/api/ route duplicates\n", len(inserts))
}
