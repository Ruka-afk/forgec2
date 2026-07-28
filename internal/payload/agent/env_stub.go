//go:build !windows

package main

func detectEnvironment() (string, *OpsProfile) {
	return "", nil
}
