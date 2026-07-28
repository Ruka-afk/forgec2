//go:build linux

package main

import (
	"os"
	"strings"
)

// DetectContainer returns the container runtime if inside a container
func DetectContainer() (string, bool) {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker", true
	}
	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil {
		content := string(data)
		if strings.Contains(content, "docker") {
			return "docker", true
		}
		if strings.Contains(content, "kubepods") {
			return "kubernetes", true
		}
		if strings.Contains(content, "containerd") {
			return "containerd", true
		}
		if strings.Contains(content, "lxc") {
			return "lxc", true
		}
	}
	sched, err := os.ReadFile("/proc/1/sched")
	if err == nil && strings.Contains(string(sched), "container") {
		return "container", true
	}
	return "", false
}

func CheckDockerSocket() bool {
	_, err := os.Stat("/var/run/docker.sock")
	return err == nil
}

func CheckK8sServiceAccount() (bool, string) {
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return false, ""
	}
	return true, string(data)
}

func GetK8sNamespace() string {
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func GetContainerID() string {
	data, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.Split(line, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			if len(lastPart) == 64 {
				isHex := true
				for _, c := range lastPart {
					if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
						isHex = false
						break
					}
				}
				if isHex {
					return lastPart
				}
			}
		}
	}
	return ""
}
