//go:build !windows
// +build !windows

package main

// The following operations are Windows-only (Kerberos delegation attacks,
// certificate-store enumeration, manual PE loading). On Linux/macOS they
// are compiled out and report "unsupported on this platform" without
// crashing the shared task dispatchers.

import "errors"

var errWindowsOnly = errors.New("operation not supported on this platform")

func findUnconstrainedDelegation() ([]string, error) {
	return nil, errWindowsOnly
}

func abuseConstrainedDelegation(userPrincipal, targetSPN string) ([]byte, error) {
	return nil, errWindowsOnly
}

func abuseRBCD(targetComputer, attackerComputer, domainAdmin string) error {
	return errWindowsOnly
}

func bronzeBitAttack(targetSPN, userPrincipal string) ([]byte, error) {
	return nil, errWindowsOnly
}

func modifyAdminSDHolder(attackerSID string) error {
	return errWindowsOnly
}

func dcsyncMachineAccount(domain, targetUser, dcIP string) ([]byte, error) {
	return nil, errWindowsOnly
}

func handleCertStoreListImpl(task Task, res *TaskResult) {
	res.Error = errWindowsOnly.Error()
}

func loadManualPE(path string) (string, error) {
	return "", errWindowsOnly
}

func handleADCSESC1Impl(task Task, res *TaskResult) { res.Error = "Windows only"; _ = task }
func handleADCSESC2Impl(task Task, res *TaskResult) { res.Error = "Windows only"; _ = task }
func handleADCSESC3Impl(task Task, res *TaskResult) { res.Error = "Windows only"; _ = task }
func handleADCSESC4Impl(task Task, res *TaskResult) { res.Error = "Windows only"; _ = task }
func handleADCSESC5Impl(task Task, res *TaskResult) { res.Error = "Windows only"; _ = task }
func handleADCSESC6Impl(task Task, res *TaskResult) { res.Error = "Windows only"; _ = task }
func handleADCSESC7Impl(task Task, res *TaskResult) { res.Error = "Windows only"; _ = task }
func handleADCSESC8Impl(task Task, res *TaskResult) { res.Error = "Windows only"; _ = task }
func handleADCSFullAuditImpl(task Task, res *TaskResult) {
	res.Error = "Windows only"
	_ = task
}

// Privilege-escalation and recon handlers registered from windows-tagged files.
func handleJuicyPotato(task Task, res *TaskResult)          { res.Error = "Windows only"; _ = task }
func handleNamedPipeImpersonate(task Task, res *TaskResult) { res.Error = "Windows only"; _ = task }
func handleSharpHound(task Task, res *TaskResult)           { res.Error = "Windows only"; _ = task }

func sccmRecon() (string, error) {
	return "", errWindowsOnly
}

func entraPRTRecon() (string, error) {
	return "", errWindowsOnly
}
