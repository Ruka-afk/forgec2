//go:build !linux

package main

import "errors"

// container escape internals are Linux-only (they target the Docker Unix
// socket and the Kubernetes service-account API). On other platforms they
// report an honest unsupported error; DetectContainer already returns
// "not inside a container" for non-Linux, so these are defensive fallbacks.

func escapeDockerSocket(payload string) (string, error) {
	return "", errors.New("docker socket escape is only supported on Linux")
}

func probeKubernetesAPI(token, ns string) (string, error) {
	return "", errors.New("kubernetes API probe is only supported on Linux")
}