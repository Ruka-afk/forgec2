# ForgeC2 Capability Matrix

> Status of implant tasks / transports as of v2.4.1.  
> Quality: **Core** (production) · **Hardened** (usable OPSEC) · **Scripted** (PS/external) · **Experimental** · **Stub** (Windows-only or incomplete).

## Transports

| Transport | Generate UI | Server listener | Implant | Quality |
|-----------|-------------|-----------------|---------|---------|
| HTTP(S) | Yes | Yes | Yes | Core |
| TCP | Yes | Yes | Yes | Core |
| DNS | Yes (+ DoH/DoT fields) | Yes | Yes | Hardened |
| WSS | Yes (Beacon Transport) | Yes | Yes | Hardened — persistent WS + binary frames + ping |
| gRPC | Yes (Beacon Transport) | Yes | Yes | Experimental — `grpcs://` TLS when server certs on; `grpc://` lab insecure |
| SSH | Yes (+ creds + host-key pin) | Yes | Yes | Experimental — host key pin via ldflag/auto server key; empty pin = lab ignore |
| ICMP | Yes | Yes (FC2I fragments) | Win/Linux | Hardened — envelopes split across Echo payloads |
| UDP | Yes (experimental) | Yes (`udp_enabled`) | Yes | Experimental — one datagram per beacon |
| QUIC | Yes (experimental) | Yes (`quic_enabled`, TLS 1.3 ALPN h3/fc2) | Yes | Experimental — stream-framed v2 envelope |
| mTLS / h2c | Selector only | Partial | Code present | Experimental |
| SMB P2P | P2P mode | Win pipe / Unix socket | Yes | Hardened |

## Core post-ex (all GOOS unless noted)

| Task | Windows | Linux | Darwin | Quality |
|------|---------|-------|--------|---------|
| shell / interactive shell | Yes | Yes | Yes | Core |
| ls / read / write / upload / download / find | Yes | Yes | Yes | Core |
| file_hunt | Yes | Yes | Yes | Hardened — listing-first, count/size/depth caps; optional small-file download |
| screen_trigger_start / stop | Yes | Yes (xdotool) | Yes (osascript) | Hardened — title match + cooldown; frames saved as screenshots |
| usb_enum | Yes (GetDriveType) | Yes (/sys/block) | Yes (diskutil) | Hardened — discovery only |
| usb_drop | Yes | Yes | Yes | Hardened — explicit source path required; dest must be removable; never copies the implant exe |
| browser_history | Yes | Yes | Yes | Hardened — copy-then-query SQLite, 200 rows |
| session_recon | Yes | Yes | Yes | Hardened — sessions/idle/active window |
| ps / killproc / suspend / resume | Yes | Yes | Limited | Core |
| screenshot | Yes | Tools-based | Tools-based | Hardened |
| set_sleep / config_push / profile_rotate | Yes | Yes | Yes | Core |
| process_tree | Yes (PPID tree) | Yes (/proc) | Yes (ps -axo) | Core |
| socks / rportfwd | Yes | Limited | Limited | Hardened |
| tun_start / tun_stop | Stub | Yes (/dev/net/tun) | Stub | Linux TUN + teamserver UDP helper; Windows needs Wintun (not bundled) |

## Credentials & AD (Windows primary)

| Task | Quality | Notes |
|------|---------|-------|
| creds | Scripted | Disk artifacts (SAM/minidump paths) |
| mimikatz / dcsync / tickets | Scripted | Requires **Modules** store `Invoke-Mimikatz.ps1` (no remote IEX) |
| kerberoast / asreproast | Scripted | Large PS payloads |
| ADCS ESC1–8 | Experimental | PS + certreq |
| browser / cookie / wifi / vpn | Hardened | Platform-specific; Chrome 127+ v20 App-Bound (DPAPI + honest IElevator miss) |
| cookie isolated proxy | Hardened | 127.0.0.1 HTTP inject; HTTPS CONNECT is tunnel-only (import Netscape jar) |
| SCCM recon | Scripted | Windows registry + WMI; discovery only |
| Entra PRT recon | Scripted | dsregcmd / CloudAP join state — not a PRT dump |
| Entra device-code / consent | Hardened | Server-side; lab client_id required; tokens not persisted |

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

## Malleable C2 profiles (v2)

| Feature | Status | Notes |
|---------|--------|-------|
| Multi-URI rotation (round-robin) | Hardened | `beacon_uris`; primary stays `beacon_uri` for old agents; server NoRoute accepts custom URIs even with no global preset |
| Request placements (cookie/query/header cover copies) | Hardened | `placements: [{target, chain}]`; canonical body always sent; server scans all query/cookie values so param-name rotation works |
| Transform chains (base64/url, netbios(u), xor, mask, strrep, case, urlencode, uri_append) | Hardened | Agent/server engines aligned (full-key xor, `key;offset` mask); Validate endpoint round-trips each chain |
| Header order determinism + UA pool | Hardened | Fixed browser order, sorted custom keys, per-beacon UA rotation (profile pool + built-ins) |
| URI jitter + param-name rotation + work window | Hardened | Junk query per beacon; query names rotate through pool; `work_start/end/tz` gates beacons |
| CS `.profile` import + Validate (dry-run) | Hardened | `POST /api/generate/profile/import-text`, `/api/generate/profile/validate` |
| Response output chains | Experimental | `server_output` encodes only via matching global preset; per-file chains preview-only until global preset matches |

Rebuild implants after changing placement/header/timing fields — old agents keep the old shape.

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
