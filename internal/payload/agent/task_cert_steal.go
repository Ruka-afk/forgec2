//go:build linux || windows || darwin

package main

import "runtime"

func handleCertStoreList(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	handleCertStoreListImpl(task, res)
}
