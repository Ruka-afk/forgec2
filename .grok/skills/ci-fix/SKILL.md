---
name: ci-fix
description: Fix ForgeC2 GitHub Actions CI failures (go test, go build, go mod tidy). Use when CI fails, GitHub Actions red, or /ci-fix.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: debug
---

## CI workflow

**File**: `.github/workflows/ci.yml`

On push/PR to `main`:
1. `go mod tidy`
2. Verify `go.mod` / `go.sum` unchanged
3. `go test ./...`
4. `go build ./cmd/server/`

## Local reproduce

```bash
go mod tidy
git diff go.mod go.sum   # should be empty after tidy
go test ./...
go build ./cmd/server/
```

## Common failures

| Error | Fix |
|-------|-----|
| `+build lines do not match //go:build` | Use `// +build linux windows darwin` (spaces not commas) |
| Test timeout in plugin | Check `executor_test.go` timeout expectations |
| Missing go.sum entry | Run `go mod tidy`, commit go.sum |
| Agent package on Linux CI | Agent is `package main` — only cross-compile via generator |
| SQLite CGO | Use `modernc.org/sqlite` (pure Go) in agent go.mod |

## Fix workflow

1. Read failed job log on GitHub Actions
2. Reproduce locally with same Go version (1.25+)
3. Fix, run full test suite
4. Commit with `fix(ci): ...`
5. Push — badge should turn green

## Badge

README: `![CI](https://github.com/Ruka-afk/forgec2/actions/workflows/ci.yml/badge.svg)`