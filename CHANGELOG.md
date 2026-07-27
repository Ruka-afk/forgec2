# Changelog

All notable changes to ForgeC2 will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [2.4.1] - 2026-07-25

### Security

- `/debug/pprof` and `/metrics` disabled by default; enable via `server.enable_pprof` / `server.enable_metrics` (auth required)
- SOCKS / rportfwd default bind host `127.0.0.1` (`server.socks_listen_host`)
- Mimikatz path no longer downloads via remote IEX; requires local script

### Changed

- i18n: English + Chinese only (removed ja/ko/ar)
- Docker: Go 1.25 image; compose wires PostgreSQL via `FORGEC2_DB_DRIVER` / `FORGEC2_DB_DSN`
- Scripts persisted to SQLite/Postgres (`scripts` table)
- Sidebar: experimental modules under collapsible **Lab** section
- Roles UI: full permission groups aligned with backend
- Autotag API paths unified under `/api/autotag/*`
- DNS start accepts domain/addr configuration body
- SSH lateral tasks available on Linux/macOS (not Windows-only)
- `process_tree` mapped to agent process list handler (alias of `ps`, not a true tree view)
- OpenAPI: default port 8000, roles admin/user

### Added

- `config.example.yaml` template without secrets
- Integrations CRUD (save/toggle/delete) via WebhookConfig persistence
- Generate page: Beacon Transport selector (WSS/gRPC/SSH/DNS/TCP/ICMP/mTLS/h2c) wired into implant ldflags
- DNS DoH/DoT and SSH credential fields on Generate for advanced transports
- Remote Desktop experimental banner (screen stream + input, not full RDP)
- Phishing campaigns: real SMTP bulk send with per-target tokens, open tracking, and public landing capture (`/phishing/l/:token`)
- OpenAPI: phishing + integrations + modules + DNS paths; version 2.4.1
- Modules store (`data/modules/`): upload Invoke-Mimikatz.ps1 in Settings; mimikatz tasks auto-attach module (no remote IEX)
- Deploy module to agent: `POST /agents/:id/modules/deploy`
- UI: silent empty-catch toasts on major list pages (scripting, notifications, files, tasks, loot, automation, bloodhound, …)
- Docs: `docs/CAPABILITY_MATRIX.md` (tasks × platform × quality)
- CI: OpenAPI check is a hard gate (core paths + min coverage); checker parses quoted Gin routes
- Phase A: lateral/opsec/cb/builds load error toasts; Chrome experimental banner; chrome_* extension-only; process_tree honesty; OpenAPI core allowlist 37; CI min-coverage raised
- Phase B: OpenAPI coverage ~34% (CI ≥30%); Container task result polling; Cloud steal result polling; Modules mimikatz readiness banner; `docs/TRANSPORT_E2E.md`
- Phase C: Settings Modules **Deploy to agent** UI; modules deploy accepts JSON body + path fix; gRPC `grpcs://` TLS (agent + server when TLS certs enabled); SSH host-key pin via ldflag/auto server key (fallback lab ignore); modules/gRPC unit tests
- Phase D: Generate transport security notes + SSH host-key pin field; Lab honesty banners (NTLM/Cloud/Container); `registerStubRoutes` → `registerExtendedRoutes`; capability matrix updated for gRPC/SSH/Modules/Lab
- Phase E: Attack/Prank/Circuit-breaker honesty banners; Lab sidebar badge; OpenAPI batch for cloud/NTLM/container/CB/chat/extc2/workflows/chain
- Phase F: Privesc/Lateral/Scanner honesty banners; OpenAPI ~49% (lateral/privesc/scan/tokens/scheduler/campaigns/…); `scripts/api-smoke.ps1`
- Phase G: API auth returns JSON 401 for `/api/*` (no SPA HTML redirect); SPA skip `/admin`/`/ready`/XHR; sidebar slim + intel/lab collapsed by default; OpenAPI ~64% (CI min-coverage 50%)
- Phase H: `api-smoke.ps1` authenticated suite (login/CSRF/agents/modules/listeners/attack); OpenAPI ~79% (agent cmd + infra/collab/ws paths); CI min-coverage 65%
- Phase I: `checkopenapi` resolves Gin `Group()` prefixes (agent cmds = `/agents/{id}/…`); OpenAPI ~94%; CI min-coverage 80% + unauthenticated health/401 smoke
- Phase J: prune Phase-H relative OpenAPI orphans (`/mimikatz` → keep `/agents/{id}/mimikatz`); `-list-stale` / `-dump-backend`; smoke adds dashboard/task-types/profiles read-only
- Phase K: empty-path Group root extraction (`GET ""` → `/api/plugins`); remove wrong collab/chat/rate-limit OpenAPI; SPA shells documented; stale=0; CI min-coverage 90% + stale gate
- Phase L: core OpenAPI schemas (Health/Modules/AgentList/Dashboard/Deploy/TaskResponse/Error); honest info.description; smoke asserts modules envelope
- FE-1: Login page en/zh + `POST /login` + version from `/health`; Generate SharedSettings i18n; 401 debounce + `?next=`/`expired=1`; generate profile load toast

## [2.4.0] - 2025-07-24

### Added

- Single-binary deployment: frontend embedded via `//go:embed`
- Docker 3-stage build (Node → Go → Alpine, ~20 MB image)
- SPA fallback middleware for client-side routing
- `NEXT_PUBLIC_API_BASE` and `NEXT_PUBLIC_WS_URL` for cross-domain development

### Changed

- Frontend no longer requires a separate server in production
- All API, WebSocket, and beacon endpoints served from a single port (8000)

### Fixed

- WebSocket hub race condition with `sync.Once` initialization
- Rate limiter goroutine leak with context-cancellable cleanup
- Circuit breaker goroutine leak with `Stop()` method
- SOCKS relay DB thrashing with in-memory byte accumulation
- Agent broadcast race with synchronous `broadcastAgentOnline`
- Login lockout memory leak with periodic cleanup goroutine
- Unbounded query cache replaced with bounded `TTLCache` (1000 entries, 5min TTL)

## [2.3.0] - 2025-06-15

### Added

- Frontend-backend separation with cross-domain deployment support
- CSRF protection (double-submit cookie pattern)
- SameSite=Lax cookies for session and CSRF tokens
- Configurable CORS via `allowed_origins`
- Cookie domain configuration for cross-origin sharing
- Request body limits (2MB max JSON)
- Campaign management (create, update, kill-chain tracking)
- Phishing module (templates, campaigns, credential capture)
- BloodHound integration (SharpHound collection, JSON upload/analysis)
- NTLM relay module
- Container escape module (Docker, Kubernetes detection)
- Task scheduler with cron-based scheduling
- Workflow engine with step execution
- Auto-tagging rules for agents
- Scheduled reports (daily/weekly/monthly PDF generation)
- Custom RBAC roles with per-resource permission gates
- Collaboration features (agent lock/unlock, task claiming)
- Prometheus metrics endpoint (`/metrics`)
- Log rotation with configurable retention
- Traffic profiling and monitoring
- Agent health plugins
- 40+ plugins (recon, hooks, reports, credential analysis)
- Circuit breaker for listener health monitoring
- Scripting engine (goja JavaScript runtime)
- External C2 channels (Discord, Slack)
- MITRE ATT&CK mapping (templates, timeline, phases, coverage)

### Changed

- Replaced `sync.Map` with bounded `TTLCache` for query caching
- Login lockout now uses IP-based tracking with periodic cleanup

## [2.2.0] - 2025-04-01

### Added

- HTTP/HTTPS/TCP/DNS/ICMP/gRPC/SSH transport
- P2P chaining (SMB named pipes / TCP relay)
- Malleable C2 profiles (15+ presets)
- Multi-listener support (independent host/port/profile per listener)
- Sleep + jitter per implant (0s real-time mode)
- AI Assistant (DeepSeek, OpenAI, Claude, Qianwen, custom OpenAI-compatible)
- Function calling for AI assistant
- SSE streaming with markdown rendering
- Chat history persistence
- Real-time shell terminal
- File browser with upload/download
- Screenshot capture and live screen streaming
- Keylogger with window titles
- Cookie export (Chrome/Edge SQLite)
- VPN and WiFi credential extraction
- SOCKS5 relay
- Reverse port forwarding
- Token operations (steal/make/revert/impersonate)
- Execute assembly, BOF, PowerPick
- Persistence (registry, schtasks, startup, WMI, service, COM hijack, IFEO)
- Privilege escalation checks
- AMSI/ETW bypass, VEH unhook, hardware breakpoints
- Sleep mask and sandbox detection
- Web console with 60+ pages
- Theme support (light/dark/system)
- i18n (en/zh)
- Ctrl+K search
- OpenAPI documentation
- TOTP two-factor authentication
- Database backup/restore
- Audit logging

## [2.1.0] - 2025-02-01

### Added

- Initial public release
- Basic C2 functionality (HTTP transport, task dispatch)
- SQLite database with GORM
- Web console with agent management
- JWT authentication
