# Architecture

ForgeC2 is a single-binary C2 framework with an embedded Next.js web console. This document describes the system architecture, component interactions, and data flows.

## System Overview

```
┌─────────────────────────────────────────────────────────┐
│                    ForgeC2 Server                        │
│                  (Go + Gin + SQLite)                     │
│                                                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────┐  │
│  │ Auth     │  │ Agent    │  │ Task     │  │ Plugin │  │
│  │ (JWT/    │  │ Manager  │  │ Queue    │  │ Runtime│  │
│  │ TOTP/    │  │          │  │          │  │ (goja) │  │
│  │ RBAC)    │  │          │  │          │  │        │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └───┬────┘  │
│       │              │              │            │       │
│  ┌────┴──────────────┴──────────────┴────────────┴────┐  │
│  │                   Gin Router                        │  │
│  │         (300+ endpoints, middleware stack)          │  │
│  └────────────────────────┬───────────────────────────┘  │
│                           │                              │
│  ┌────────────────────────┴───────────────────────────┐  │
│  │                  SQLite (GORM)                      │  │
│  │     agents, tasks, credentials, users, roles,       │  │
│  │     listeners, audit_logs, cache, plugins           │  │
│  └────────────────────────────────────────────────────┘  │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ WebSocket    │  │ AI Assistant │  │ Scripting    │  │
│  │ Hub          │  │ (SSE)        │  │ Engine       │  │
│  │ (live push)  │  │              │  │ (goja)       │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
│                                                         │
│  ┌────────────────────────────────────────────────────┐  │
│  │              Embedded Frontend (SPA)                │  │
│  │         //go:embed all:dist (Next.js static)        │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────┬──────────────────────────────┘
                           │
              ┌────────────┴────────────┐
              │     Transport Layer     │
              │                         │
              │  HTTP(S)  TCP  DNS      │
              │  ICMP     gRPC  SSH     │
              │  SMB/TCP (P2P)          │
              │  Discord/Slack (ExtC2)  │
              └────────────┬────────────┘
                           │
              ┌────────────┴────────────┐
              │       Agent Implants    │
              │                         │
              │  Windows (x64/x86/ARM)  │
              │  Linux   (x64/ARM)      │
              │  macOS   (x64/ARM)      │
              │  PowerShell             │
              └─────────────────────────┘
```

## Component Interactions

### 1. Agent Check-in Flow

```
Agent ──── HTTP POST /beacon ────→ RateLimiter ──→ Auth ──→ TaskQueue
  │                                                         │
  │←──── JSON response (encrypted tasks) ──────────────────-┘
  │
  ├── On success: update last_seen, broadcast to WebSocket hub
  └── On failure: increment error count, trigger circuit breaker
```

### 2. Task Dispatch Flow

```
Web UI ── POST /agents/:id/command ──→ Handler ──→ OPSEC Guard ──→ DB (tasks)
  │                                                      │
  │                                                      ↓
  │←── WebSocket push (task_created) ────────────── Task Queue
  │
  └── Agent polls tasks on next beacon check-in
```

### 3. WebSocket Live Push

```
Server ──── broadcastToClients() ──→ wsClientConn.channel
  │                                      │
  │                                      ├── wsWriter goroutine
  │                                      └── WebSocket connection
  │
  └── Events: agent_online, agent_offline, task_update, credential_found
```

### 4. AI Assistant Flow

```
User ── POST /ai/chat ──→ AIHandler ──→ Provider API (SSE)
  │                                      │
  │←──── SSE stream (markdown chunks) ──┘
  │
  └── Tool calls: execute_command → TaskQueue → Agent
```

## Data Model

### Core Entities

| Entity | Table | Key Relationships |
|--------|-------|-------------------|
| Agent | `agents` | has many Tasks, Credentials, Tokens |
| Task | `tasks` | belongs to Agent, has status/result |
| Credential | `credentials` | belongs to Agent (optional) |
| User | `users` | has Role, can lock/unlock Agents |
| Listener | `listeners` | serves Agents, has profile config |
| AuditLog | `audit_logs` | tracks all user operations |
| Plugin | `plugins` | has Manifest, Reviews, Ratings |
| Campaign | `campaigns` | has MITRE mappings, tasks |
| Workflow | `workflows` | has steps, executions |

### Indexes

| Table | Index | Purpose |
|-------|-------|---------|
| `tasks` | `idx_tasks_agent_created_status` | Dashboard queries, task history |
| `tasks` | `idx_tasks_agent_id` | Agent task listing |
| `audit_logs` | `idx_audit_logs_created_at` | Audit log queries |
| `credentials` | `idx_credentials_agent_id` | Agent credential listing |

## Middleware Stack

Request flows through these middleware in order:

1. **gin.Recovery** — panic recovery
2. **InFlightTracker** — graceful shutdown tracking
3. **RequestID** — X-Request-ID generation/validation
4. **SecurityHeaders** — CSP, X-Frame-Options, etc.
5. **NoCache** — cache-control for API responses
6. **ErrorHandler** — unified error handling
7. **metricsMiddleware** — Prometheus request metrics
8. **AuthRequired** (auth group only) — JWT validation + cookie check
9. **CSRFProtect** (auth group only) — double-submit cookie pattern
10. **RateLimiter** (per-route) — IP-based rate limiting
11. **RequirePermission** (per-route) — RBAC permission check

## Security Architecture

### Authentication

1. User submits credentials to `POST /login`
2. Server validates against bcrypt hash
3. IP-based lockout check (5 attempts / 15min window)
4. JWT token generated with user ID, role, expiry
5. Session cookie set (`HttpOnly`, `Secure`, `SameSite=Lax`)
6. CSRF cookie set (`forgec2_csrf`)
7. All subsequent requests validated via `AuthRequired` middleware

### Agent Encryption

- ECDH key exchange during agent registration
- Per-session AES-256-GCM encryption
- Session key rotation after configurable message count
- Loot encrypted at rest with server-derived key

## Deployment Architecture

### Single Binary

```
forgec2-server.exe (or ./forgec2-server)
├── config.yaml (mounted or embedded)
├── data/
│   ├── forgec2.db (SQLite)
│   ├── server.crt / server.key (TLS)
│   └── plugins/ (plugin data)
└── logs/ (audit, application logs)
```

### Docker

```yaml
services:
  forgec2:
    build: .
    ports: ["8000:8000"]
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./data:/app/data
```

3-stage build: Node 20 (frontend) → Go 1.22 (backend with embedded FS) → Alpine (runtime, ~20 MB)
