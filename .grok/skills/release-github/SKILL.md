---
name: release-github
description: Push ForgeC2 to GitHub, create git tag, publish GitHub Release with server binary. Use for release, tag, push to GitHub, or /release-github.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: deploy
---

## When to use

Ship a version to GitHub with tag + Release asset (`forgec2-server.exe`).

## Pre-release checklist

| Step | Command / action |
|------|------------------|
| 1 | `go test ./...` |
| 2 | `powershell -File build_js.ps1` (if JS/CSS changed) |
| 3 | `go build -o forgec2-server.exe ./cmd/server/` |
| 4 | Update `README.md` / `README.zh.md` version line if needed |

## Git push + tag

```powershell
git add -A
git commit -m "vX.Y.Z: <summary>"
git push origin main

git tag -a vX.Y.Z -m "vX.Y.Z: <summary>"
git push origin vX.Y.Z
```

## GitHub Release (gh CLI)

```powershell
$env:Path += ";C:\Program Files\GitHub CLI"
gh auth login   # once, if not authenticated

gh release create vX.Y.Z `
  --title "vX.Y.Z" `
  --notes "## Highlights`n- ..." `
  forgec2-server.exe
```

## GitHub Release (API fallback)

If `gh` unavailable, use GitHub API `POST /repos/Ruka-afk/forgec2/releases` then upload asset to `uploads.github.com`.

## Release notes template

```markdown
## vX.Y.Z

### UI
- ...

### Implant / C2
- ...

### Install
go build -o forgec2-server.exe ./cmd/server/
.\forgec2-server.exe -config config.yaml
```

## Verify

- Tag visible at https://github.com/Ruka-afk/forgec2/tags
- Release page has `forgec2-server.exe` download
- CI badge green on `main`