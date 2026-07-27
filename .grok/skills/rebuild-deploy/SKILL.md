---
name: rebuild-deploy
description: Rebuild ForgeC2 single binary (embedded frontend + Go server) and restart
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: deploy
---

## When to use

Use after changing Go handlers/API, Next.js pages (`frontend/src/`), or embedded frontend assets.

## Architecture

Single binary — all-in-one process:

| Component | Port | Build |
|-----------|------|-------|
| Go server (with embedded Next.js UI) | 8000 | `powershell -File scripts/build-embedded.ps1` |

## Windows — full restart (recommended)

```powershell
powershell -File scripts\build-embedded.ps1
```

`build-embedded.ps1` builds Next.js frontend → copies to embed directory → compiles Go binary → restarts server → runs health check.

## Manual steps

```powershell
# 1. Build frontend
cd frontend
npm install
npm run build

# 2. Copy to embed directory
Remove-Item -Recurse -Force ..\internal\webdist\dist -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path ..\internal\webdist\dist | Out-Null
Copy-Item -Recurse -Path "out\*" -Destination "..\internal\webdist\dist\"

# 3. Build Go binary
cd ..
go build -o forgec2-server.exe ./cmd/server

# 4. Stop existing process
taskkill /f /im forgec2-server.exe 2>$null

# 5. Start server
Start-Process .\forgec2-server.exe -ArgumentList "-config config.yaml" -WindowStyle Hidden

# 6. Health check
Start-Sleep -Seconds 3
Invoke-RestMethod http://127.0.0.1:8000/health
```

## Development mode

```powershell
# Terminal 1 — API with live reload
go run ./cmd/server -config config.yaml

# Terminal 2 — Next.js hot reload (proxies API to Go on :8000)
cd frontend
npm run dev
```

Open **http://localhost:3000** (Next.js hot-reload) or **http://localhost:8000** (single binary).

## Backend-only rebuild (no frontend changes)

```powershell
powershell -File scripts\dev-backend.ps1
```

## Verify

- `http://127.0.0.1:8000/health` returns `{"status":"ok"}`
- `http://127.0.0.1:8000/login` returns 200 (SPA HTML)
- Dashboard charts load; WebSocket shows Live in top bar
