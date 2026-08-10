# ForgeC2 Capability Matrix

> Status of implant tasks / transports as of v2.4.1.  
> Quality: **Core** (production) · **Hardened** (usable OPSEC) · **Scripted** (PS/external) · **Experimental** · **Stub** (Windows-only or incomplete).

## Transports

| Transport | Generate UI | Server listener | Implant | Quality |
|-----------|-------------|-----------------|---------|---------|
| HTTP(S) | Yes | Yes | Yes | Core |
| TCP | Yes | Yes | Yes | Core |
| DNS | Yes (+ DoH/DoT fields) | Yes | Yes | Hardened |
| WSS | Yes (Beacon Transport) | Partial | Yes | Hardened — see `docs/TRANSPORT_E2E.md` |
| gRPC | Yes (Beacon Transport) | Yes | Yes | Experimental — `grpcs://` TLS when server certs on; `grpc://` lab insecure |
| SSH | Yes (+ creds + host-key pin) | Yes | Yes | Experimental — host key pin via ldflag/auto server key; empty pin = lab ignore |
| ICMP | Yes | Yes | Win/Linux | Experimental |
| mTLS / h2c | Selector only | Partial | Code present | Experimental |
| SMB P2P | P2P mode | Win pipe / Unix socket | Yes | Hardened |

## Core post-ex (all GOOS unless noted)

| Task | Windows | Linux | Darwin | Quality |
|------|---------|-------|--------|---------|
| shell / interactive shell | Yes | Yes | Yes | Core |
| ls / read / write / upload / download / find | Yes | Yes | Yes | Core |
| ps / killproc / suspend / resume | Yes | Yes | Limited | Core |
| screenshot | Yes | Tools-based | Tools-based | Hardened |
| set_sleep / config_push / profile_rotate | Yes | Yes | Yes | Core |
| process_tree | Alias → ps | Alias → ps | Alias → ps | Core* (*not a true tree) |
| socks / rportfwd | Yes | Limited | Limited | Hardened |

## Credentials & AD (Windows primary)

| Task | Quality | Notes |
|------|---------|-------|
| creds | Scripted | Disk artifacts (SAM/minidump paths) |
| mimikatz / dcsync / tickets | Scripted | Requires **Modules** store `Invoke-Mimikatz.ps1` (no remote IEX) |
| kerberoast / asreproast | Scripted | Large PS payloads |
| ADCS ESC1–8 | Experimental | PS + certreq |
| browser / cookie / wifi / vpn | Hardened | Platform-specific |

## Lateral & privesc

| Task | Windows | Linux/macOS | Quality |
|------|---------|-------------|---------|
| lateral_wmi / winrm / psexec / dcom | Yes | Stub | Scripted / Hardened |
| ssh_lateral / scp / ssh_tunnel | Yes | Yes | Hardened (Go SSH) |
| token_* / UAC / potato | Yes | Stub | Hardened (Win) |
| container_detect | Detect | Linux real | Experimental escape |

## Execution / inject

| Task | Quality | Notes |
|------|---------|-------|
| execute_assembly / powerpick / peloader | Hardened | Windows |
| bof | Hardened | Windows |
| inject / shinject / threadless… | Hardened | Windows-only suite |

## Server product modules

| Feature | Quality | Notes |
|---------|---------|-------|
| Agents / Listeners / Generate | Core | |
| AI assistant | Hardened | Needs API key |
| Automation / webhooks | Core | |
| Phishing SMTP + landing | Hardened | Real send; authorized use only |
| Integrations CRUD | Hardened | Persisted webhooks |
| Modules store | Core | Settings → Modules; deploy to agent via upload task |
| Scripting | Hardened | DB-persisted scripts |
| Remote desktop | Experimental | Screenshot stream + input |
| Chrome C2 page | Experimental | Extension agent only; standard implant has no chrome_* handlers |
| NTLM coerce/relay UI | Experimental | Lab section; Windows-centric |
| Cloud steal UI | Experimental | Lab; result polling |
| Container UI | Experimental | Detect better on Linux; escape often stub elsewhere |
| OpenAPI | Hardened | ~98% inventory; core schemas for health/agents/modules/dashboard; stale=0; CI ≥90% |
| API smoke | Hardened | `scripts/api-smoke.ps1` (+ `-TryDefaultAdmin`); modules JSON envelope check |
| Lateral UI | Hardened / Scripted | Method-dependent; Win-primary for many paths |
| Privesc UI | Hardened / Scripted | Checks ≠ guaranteed elevation |
| Scanner UI | Hardened | Agent-side port scan; not full Nmap |
| ATT&CK coverage UI | Hardened | Task-type mapping only — not proof of compromise |
| Circuit breaker UI | Hardened | Listener probe/failover; not full multi-C2 mesh |

## Operator rules

1. Prefer **Core/Hardened** for real engagements.  
2. **Scripted** tasks need modules/tools and leave more telemetry.  
3. **Experimental** must be validated per target; may return unknown/stub.  
4. After implant code changes, **regenerate** payloads (sleep/config alone is not enough).  
5. Upload `Invoke-Mimikatz.ps1` under **Settings → Modules** before credential dumps.

## Regenerating this matrix

When adding task types, update:

- `pkg/protocol/tasks.go`
- `internal/payload/agent/task_registry.go`
- `internal/server/tasktypes.go`
- This document
