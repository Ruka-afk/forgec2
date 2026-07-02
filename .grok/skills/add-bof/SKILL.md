---
name: add-bof
description: Extend ForgeC2 BOF management (upload, run, edit, repo import). Use for BOF features, beacon object files, or /add-bof.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add BOF upload/run workflow, BOF repo integration, or agent-side BOF execution changes.

## Key files

| Layer | File |
|-------|------|
| Server handlers | `internal/server/handlers_bof.go` |
| BOF repo | `handlers_automation.go` — `handleBOFRepoIndex`, `handleBOFRepoImport` |
| UI | `templates/bof.html`, `templates/bof_repo.html` |
| JS | `static/js/bof.js`, `bof_repo.js` (in `plugins.bundle.js` / `admin.bundle.js`) |
| Agent execution | `internal/payload/agent/bof.go` |
| DB | `db.BOF` model |

## UI patterns

- Cards: `ui-card` (must be in `layout.css` — see `fix-ui-style`)
- Modals: `ui-modal-backdrop` + `ui-modal` (`upload-modal`, `run-modal`, `edit-modal`)
- Actions: `data-action="run-bof"`, `show-upload-modal`, etc. in `bof.js`

## Add BOF API endpoint

1. Route in `server.go` under auth group.
2. Handler in `handlers_bof.go` — validate `.o` file, store in DB/disk.
3. Run flow: create task type `bof` → agent `bof.go` loads COFF, executes, returns output.
4. Audit: `s.logAction(c, "bof_run", ...)`.

## Agent-side BOF run

- Task args: BOF ID, argument string matching COFF definition.
- Windows only for full COFF loader; stub on Linux/macOS if needed.

## Verify

- Upload `.o` via UI modal
- Run against online agent → task completes with output
- Download / edit / delete work without console errors