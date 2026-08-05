# ⚒️ ForgeC2

> Command and control, forged for the modern red team.

[![CI](https://github.com/Ruka-afk/forgec2/actions/workflows/ci.yml/badge.svg)](https://github.com/Ruka-afk/forgec2/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Ruka-afk/forgec2)](https://github.com/Ruka-afk/forgec2/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**English** · [中文](README.zh.md)

ForgeC2 is a self-hosted, single-binary C2 platform written in pure Go. One executable ships a hardened implant build pipeline, multi-protocol beaconing, an AI-assisted operations console, and a full Next.js web UI — no frontend server, no database engine, no dependencies to babysit.

---

## What you get

| | |
|---|---|
| 🚀 **One binary, everything inside** | Next.js console, REST API, beacon endpoints and SQLite — all served from a single port. Deploy with one file. |
| 🧬 **On-demand payload factory** | EXE / DLL / PowerShell / ELF / macOS implants, XOR stagers, shellcode, Donut, and one-liners — generated in-browser, cross-compiled server-side. |
| 📡 **Ten transports** | HTTP(S), WSS, gRPC, mTLS, H2C, TCP, DNS, ICMP, SSH — plus SMB/TCP P2P chaining and Discord/Slack external C2. |
| 🤖 **AI copilot built in** | DeepSeek, OpenAI, Claude, or any OpenAI-compatible model — drive your engagement from chat with tool calling. |
| 🛡️ **OPSEC as a feature** | Pre-flight rule engine, malleable C2 profiles, AMSI/ETW evasion, sleep masks, and a payload pipeline hardened against sloppy defaults. |
| 🧩 **Extensible by design** | 40+ drop-in plugins, a JavaScript scripting engine, workflow automation, and a full OpenAPI surface. |

---

## Quick start

**Linux**

```bash
chmod +x forgec2-server-linux-amd64
./forgec2-server-linux-amd64 -config config.yaml
```

**Windows**

```powershell
.\forgec2-server.exe -config config.yaml
```

Open `http://localhost:8000` — the server prints a freshly generated admin password to the console on first run.

### Build it yourself

```bash
git clone https://github.com/Ruka-afk/forgec2.git && cd forgec2

# requires Go 1.25+ and Node.js 20+
powershell -File scripts/build-embedded.ps1   # frontend → embedded → binary

# ...or containerized
docker compose up -d
```

---

## The payload generator

The centerpiece of ForgeC2 is a workspace-style generator that treats payload creation like a proper build pipeline:

- **Sticky connection panel** — listener, C2 URLs, transport, malleable profile, beacon timing, and keys stay in view while you build
- **Build status** — every artifact reports Ready / Compiling / Done / Failed with inline results
- **Artifact families** — agent binaries (EXE, DLL, PS1, ELF, macOS), stagers, shellcode/Donut, one-liners, and one-click quick presets
- **Transport-aware fields** — picking WSS, gRPC, SSH, DNS, ICMP, mTLS, or H2C reveals only the fields that transport actually needs
- **Everything is i18n'd** — English and Chinese, with key coverage enforced in CI

## Implant capabilities

50+ task types across the standard ops playbook:

**Access** — shell, PowerShell, execute-assembly, BOF, PowerPick, PE/CLR loading, token steal/make/revert, credentials, mimikatz, kerberoast, DCSync
**Lateral** — WMI, WinRM, PsExec, Pass-the-Hash, Pass-the-Ticket, SMB/TCP relay, SOCKS5, port forward, NTLM relay
**Persistence** — registry, scheduled tasks, startup, WMI, services, COM hijack, IFEO
**Evasion** — AMSI/ETW bypass, VEH unhook, hardware breakpoints, sleep masks, sandbox detection
**Surveillance** — screenshot, live screen, window-titled keylogging, recording, clipboard, remote input
**Recon** — cookie export, VPN/WiFi credentials, portscan, process tree, OS/domain discovery

Full per-task, per-OS capability matrix: [docs/CAPABILITY_MATRIX.md](docs/CAPABILITY_MATRIX.md)

---

## Operations console

- **60+ pages** — dashboard with live charts (heatmaps, OS distribution, task Gantt, geo, attack paths), agent fleet management, file browser, terminal, token lab, traffic profiles
- **Multi-operator** — RBAC roles, agent locking, task claiming, audit trail
- **Automation** — workflow engine, cron scheduler, auto-tagging, scheduled PDF reports
- **Teammate tools** — campaigns, phishing (SMTP + tracking), BloodHound ingestion, domain fronting, infrastructure redirectors
- **Resilience** — circuit breaker for listener health, AES-GCM encrypted DB backups, graceful failover

## Security posture

- Auto-generated admin password, JWT secret, and TLS material on first boot — no default credentials anywhere
- JWT + bcrypt sessions, TOTP 2FA, CSRF double-submit, SameSite cookies, strict security headers
- Rate limiting and IP lockout on auth, body-size caps, path-traversal guards, audit logging
- Payload pipeline: crypto/rand entropy, randomized PE section names, in-place benign import injection, AMSI-aware macro generation

---

## Architecture at a glance

```
                    ┌────────────────────────────────────────────┐
   Operators ─────▶ │  ForgeC2 (single binary, :8000)            │
                    │  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
                    │  │  Web UI  │  │   API    │  │  Beacon  │  │
                    │  │ Next.js  │  │ Gin REST │  │ endpoints│  │
                    │  │ (embedded)│ │ + WS + AI│  │          │  │
                    │  └──────────┘  └──────────┘  └──────────┘  │
                    │  SQLite · Plugins · Scripting · OPSEC      │
                    │  Build queue → cross-compiled implants     │
                    └───────────┬────────────────────────────────┘
                                │ HTTP(S)/WSS/gRPC/mTLS/H2C/TCP/DNS/ICMP/SSH
                    ┌───────────▼────────────────────────────────┐
                    │  Windows / Linux / macOS implants (P2P)     │
                    └────────────────────────────────────────────┘
```

Deep dive: [ARCHITECTURE.md](ARCHITECTURE.md)

---

## Configuration

Everything lives in one YAML file ([config.example.yaml](config.example.yaml) is the reference). Highlights:

| Key | Purpose |
|---|---|
| `server.port` / `server.tls_enabled` | Listen address and TLS termination |
| `server.allowed_origins` / `cookie_domain` | Cross-domain deployment |
| `implant.default_interval` / `default_jitter` | Beacon cadence defaults |
| `ai.provider` / `api_key` / `model` | AI assistant backend |
| `rate_limit.login.*` | Auth brute-force protection |

## Development

```bash
go build ./...            # backend
go test ./internal/...    # tests (run with -count=1)
cd frontend && npm run dev  # UI hot-reload on :3000
```

Repository hygiene is enforced by checks: `go vet`, OpenAPI validation (`cmd/checkopenapi`), and frontend CSS/i18n/path gates (`npm run check`).

## Docs & versioning

- [CHANGELOG.md](CHANGELOG.md) — full release history (currently **v2.5.0**)
- [docs/](docs/) — transport E2E labs, capability matrix, design docs
- [CONTRIBUTING.md](CONTRIBUTING.md) — how to build, test, and ship code
- [SECURITY.md](SECURITY.md) — vulnerability disclosure

---

## Legal

ForgeC2 is for **authorized security testing only**. You must have explicit written permission from the owner before using it against any system. See [LICENSE](LICENSE).

---

*Forge your access. Control your narrative.*
