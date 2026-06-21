# ForgeC2

[English](./README.md) | [中文](./README.zh.md)

**Professional Command & Control Framework for Authorized Red Team Operations**

ForgeC2 is a modern, single-binary, operator-friendly C2 framework built in pure Go. Features P2P beacon chaining, DNS beaconing, Artifact Kit, credential harvesting, 50+ task types, live screen streaming, a powerful **AI Assistant**, and a clean web interface.

**v1.3.0** — AI Assistant (DeepSeek/OpenAI/Claude/Qianwen), Implant terminology, 3-state agent status, 33 externalized JS files, comprehensive hardening.

## 🤖 AI Assistant (NEW)

- **Multi-LLM**: DeepSeek, OpenAI (ChatGPT), Claude, Qianwen (通义千问), Custom (OpenAI-compatible)
- **Function Calling**: List Implants, execute commands, view credentials, manage listeners — via natural language
- **Streaming Chat**: Real-time SSE streaming with markdown (tables, code blocks, lists)
- **Reasoning Display**: DeepSeek R1 thinking process visualization
- **Conversation Persistence**: localStorage save, context window (20 pairs), Markdown export
- **Safety**: Content length cap, tool deduplication, consecutive call limits

## 🏗️ Core C2

- HTTP(S), TCP, DNS, ICMP transport layers
- P2P Beacon Chaining: SMB Named Pipes / TCP relay, parent-child topology
- 15+ Malleable C2 Profiles: bing, google, office365, teams, slack, zoom, etc.
- Multi-listener support with independent configs
- Configurable sleep/jitter, on-the-fly `set_sleep`

## 🧠 Implant Capabilities (50+ Tasks)

| Category | Tasks |
|----------|-------|
| Shell & System | `shell`, `ps`, `killproc`, `suspend`, `resume`, `reboot`, `shutdown` |
| Credentials | `creds`, `mimikatz`, `kerberoast`, `dcsync`, auto-vault |
| Lateral Movement | WMI, WinRM, PsExec, Pass-the-Hash, Pass-the-Ticket |
| Token Ops | steal, make, revert, whoami |
| Execution | execute-assembly, BOF, PowerPick, PE Loader, Fork&Run |
| Persistence | Registry, schtasks, Startup, WMI, Service, COM hijack |
| Surveillance | screenshot, keylogger, live screen stream |
| Network | SOCKS5 relay, portscan, rportfwd |

## 🖥️ Web UI

- **Dashboard**: Implant counts (online/stale/offline), task stats
- **Implant Detail**: 3-state status, public IP + GeoIP map, lock/viewer indicators
- **Shell**: Compact toolbar with dropdowns, command history, auto-complete
- **Post-Exploitation Toolkit**: 40+ categorized commands with quick actions
- **Generate**: Shared listener selector, cross-platform (EXE/PS1/ELF/macOS/Stager/OneLiner)
- **Audit Log**: Full operator action logging, CSV export
- **Credential Vault**: Masked passwords, API-based copy
- **Sidebar**: Collapsible groups, online users panel

## 🔒 Security

- JWT + bcrypt, auto-generated JWT secret, HttpOnly CSRF tokens
- Rate limiting, audit logging, path traversal prevention
- Plaintext passwords never in HTML DOM
- XSS hardening: DOM textContent escape, innerHTML sanitized
- 33 JS files externalized from templates

## 🧩 Tech Stack

- **Backend**: Go 1.25, Gin, GORM, SQLite (glebarez), Gorilla WebSocket
- **Frontend**: Tailwind CSS, Font Awesome, vanilla JS (no framework)
- **Database**: SQLite with WAL mode, indexed FK, batch inserts

## Quick Start

```bash
git clone https://github.com/Ruka-afk/forgec2.git
cd forgec2
go mod tidy
go run ./cmd/server
```

Server starts on `http://0.0.0.0:8080`. Default login: `admin` / `admin`.

## Roadmap

- [x] HTTP/HTTPS/TCP/DNS transport
- [x] P2P beacon chaining (SMB/TCP)
- [x] Artifact Kit (stager/stage)
- [x] Credential auto-import + vault
- [x] Malleable C2 profiles (15+)
- [x] Multi-user RBAC + collaboration
- [x] Post-Exploitation Toolkit
- [x] Live screen streaming
- [x] SOCKS5 relay
- [x] Security audit + hardening
- [x] AI Assistant (DeepSeek/OpenAI/Claude/Qianwen)
- [x] JS externalization (33 files)
- [x] Agent→Implant terminology
- [x] 3-state status (online/stale/offline)
- [x] Public IP + GeoIP mapping
- [ ] macOS agent
- [ ] EDR evasion modules

## Legal

**For authorized security testing, red team exercises, and educational purposes only.** Explicit written authorization required before deployment.

## License

Custom license. See `LICENSE` or contact for commercial licensing.

---

*ForgeC2 — Forge your access. Control your narrative.*
