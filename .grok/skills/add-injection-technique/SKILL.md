---
name: add-injection-technique
description: Add ForgeC2 process injection techniques (EarlyBird, Threadless, NtCreateThreadEx). Use for injection, shellcode inject, or /add-injection-technique.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add a new Windows injection method for shellcode or PE loading.

## Key files

| File | Role |
|------|------|
| `agent/task_injection.go` | Task dispatcher for inject types |
| `agent/injection_earlybird_windows.go` | EarlyBird APC |
| `agent/injection_threadless_windows.go` | Threadless inject |
| `agent/injection_ntcreatethreadex.go` | Classic remote thread |
| `agent/peloader.go` | PE reflective load |
| `internal/payload/shellcode.go` | Shellcode helpers |

## Checklist

1. Implement `injectYourMethod(shellcode []byte, pid uint32) error` with `//go:build windows`.
2. Register task type in `task_registry.go`.
3. Server route + handler queues task with pid + payload ref.
4. Stub on Linux/macOS returning unsupported message.
5. Document required privileges (SeDebugPrivilege) in task output errors.

## Safety / evasion

- Pair with `edr-evasion` skill for AMSI/ETW stubs if loading .NET or scripts.
- Syscall stubs: `syscall_stubs_windows.go` for indirect syscalls.

## Verify

- Inject into test process (notepad) on Windows agent
- Task result reports success/failure with Win32 error code
- Non-Windows agents return clean unsupported JSON