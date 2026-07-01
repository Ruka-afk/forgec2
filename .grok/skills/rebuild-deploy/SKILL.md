---
name: rebuild-deploy
description: Rebuild ForgeC2 frontend bundles and Go server, then restart the server on Windows
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: deploy
---

## When to use

Use after changing Go handlers, HTML templates, or JS/CSS under `internal/server/templates/static/`.

## Windows rebuild & restart

```powershell
# 1. Rebundle JS only (skip CSS when unchanged)
powershell -ExecutionPolicy Bypass -File build_js.ps1 -SkipCSS

# 2. Build server binary
go build -o server.exe ./cmd/server/

# 3. Stop existing server (if running)
Get-Process server,forgec2-server -ErrorAction SilentlyContinue | Stop-Process -Force

# 4. Start server
.\server.exe -config config.yaml
```

## Makefile shortcuts

```bash
make build-js    # runs build_js.ps1
make build       # go build ./cmd/server/
make build-all   # bundle + build
make dev         # FORGEC2_DEV=1 go run (unbundled JS)
```

## Notes

- Production pages load `*.bundle.js` from `pageScriptMap` in `internal/server/page_assets.go`.
- Dev mode uses individual `.js` files when `FORGEC2_DEV=1` (or `FORGEC2_DEV_MODE=1` for bundle toggle).
- After JS changes in production mode, always rerun `build_js.ps1` before testing in the browser.

## Verify

- `go build ./cmd/server/` succeeds
- Server starts without errors in `server.log` / console
- Changed page loads and actions respond