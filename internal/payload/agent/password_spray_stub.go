//go:build !windows

package main

import "fmt"

func trySprayAuth(domain, user, password, dc string) (string, string) {
	return "error", fmt.Sprintf("password spray is Windows-only (logon user %s\\%s)", domain, user)
}
