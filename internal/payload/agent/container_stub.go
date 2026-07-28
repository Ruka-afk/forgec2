//go:build !linux

package main

func DetectContainer() (string, bool) {
	return "", false
}

func CheckDockerSocket() bool {
	return false
}

func CheckK8sServiceAccount() (bool, string) {
	return false, ""
}

func GetK8sNamespace() string {
	return ""
}

func GetContainerID() string {
	return ""
}
