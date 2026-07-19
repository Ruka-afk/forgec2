# ForgeC2 — Session Summary

## Goal
Make the ForgeC2 frontend fully functional: eliminate browser console errors caused by
frontend↔backend route mismatches and a NoRoute double-write bug.

## Root cause of console errors (discovered)
1. **NoRoute double-write bug** — `internal/server/server.go` NoRoute prepends `/api` to
   bare paths and calls `s.router.HandleContext(c)`. For endpoints registered ONLY as
   `/api/...` (no bare alias), a bare-path request (what the frontend sends) hit NoRoute
   and the response was written TWICE → `SyntaxError: Unexpected non-whitespace
   character after JSON`. Confirmed empirically: `/api/opsec/rules` = 511 bytes (single),
   `/opsec/rules` (via NoRoute) = 1022 bytes (doubled).
   **Fix:** add explicit bare-path route aliases for every endpoint the frontend calls by bare path.
2. **Missing backend handlers** — frontend calls endpoints with no backend implementation:
   autotag rules, campaign wizard CRUD/mitre/killchain, attack coverage, mitre phases.

## Fixes applied (this session)
- Added bare-path aliases in `server.go` (auth group) for: `/opsec/rules` (+check/history/
  create/delete), `/timeline/data`(+export), `/automation/rules`(+id/toggle), `/webhooks`
  (+test/:id), `/monitor/alert-rules` (+PUT/DELETE). This stops the JSON doubling.
- New handler file `handlers_campaign.go`: `/campaigns` list, `/campaigns/:id` (stats),
  `/api/v1/campaigns` create, `/v1/campaigns/:id` update+delete, `/campaigns/:id/mitre`,
  `/v1/campaigns/:id/killchain`, `/mitre/templates`, `/mitre/timeline`, `/mitre/phases`.
- New handler file `handlers_autotag.go`: `/api/autotag/rules` list/create, `/autotag/rules/:id`
  update/toggle/delete, `/api/autotag/apply` (evaluates enabled rules against implants).
- New handler file `handlers_mitre.go`: `/attack/coverage` (ATT&CK tactic→technique map
  derived from task types + executed tasks), `/mitre/phases` (kill-chain coverage).
- New handler file `handlers_opsec_extra.go`: `/opsec/history`, `/opsec/rules` create,
  `/opsec/rules/:name` delete.

## Second wave of errors (this session) — translations / report / chrome
- Console errors: `/translations/stats`, `/translations/check` (doubling),
  `/report/agents`, `/report/history` (doubling), `/api/chrome/agents` (404).
- Same NoRoute double-write for bare `/translations/*` and `/report/*` GETs.
- **Chrome had NO backend at all** — `ChromeC2Page` calls `/api/chrome/agents` (GET,
  already `/api/`-prefixed → proxy → `/api/chrome/agents` at backend = 404) and
  `/chrome/agents/:id/tasks` (POST, bare). No handlers or routes existed.

## Second-wave fixes
- Bare aliases in `server.go` auth group: `/translations/stats`, `/translations/check`,
  `/report/agents`, `/report/tasks`, `/report/credentials`, `/report/network`,
  `/report/findings`, `/report/history`, `/report/generate` (POST), `/report/export/pdf`.
- New handler file `handlers_chrome.go`: `handleChromeAgents` (GET `/api/chrome/agents`
  + `/chrome/agents`; returns implants tagged "chrome" as `ChromeAgent[]`) and
  `handleChromeAgentTask` (POST `/chrome/agents/:uuid/tasks` + `/api/...`; creates a
  task via `s.createTask`).

## Verification
- `go build ./...` clean, `go build -o forgec2-server.exe ./cmd/server` OK.
- All endpoints now return **HTTP 200 + single JSON** (no doubling), tested both directly
  (`:8080`) and through the proxy (`:3000/api/go/...`):
  `/opsec/rules`, `/opsec/history`, `/timeline/data`, `/automation/rules`, `/webhooks`,
  `/monitor/alert-rules`, `/campaigns`, `/api/autotag/rules`, `/api/autotag/apply`,
  `/mitre/phases`, `/attack/coverage`, `/mitre/templates`, `/campaigns/:id/mitre`.
- Second wave all SINGLE via proxy: `/translations/stats`(13243), `/translations/check`(123),
  `/translations?lang=en`(14238), `/report/agents`(13), `/report/history`(14),
  `/report/tasks`(69), `/report/credentials`(18), `/report/network`(16),
  `/report/findings`(15), `/api/chrome/agents`(28), `/chrome/agents`(28).
- Chrome POST `/chrome/agents/<uuid>/tasks` returns valid JSON (`agent not found` for unknown).

## Third wave of errors (this session) — Task Scheduler
- Console error: `GET http://127.0.0.1:3000/api/go/api/scheduler/tasks 404`.
- Backend had **no scheduler routes at all** (no handler file, no routes). `ScheduledTask`
  model exists in `db/models.go:741` (already AutoMigrated). `ScheduledTask` table name =
  `scheduled_tasks`.
- SchedulerPage (`frontend/src/app/(main)/scheduler/page.tsx`) calls:
  GET `/api/scheduler/tasks` (49), GET `/agents` (50), PUT `/scheduler/tasks/:id` (72),
  POST `/api/scheduler/tasks` (74), POST `/scheduler/tasks/:id/toggle` (81),
  DELETE `/scheduler/tasks/:id` (87).

## Third-wave fixes
- New handler file `handlers_scheduler.go`: `handleSchedulerListTasks`,
  `handleSchedulerCreateTask` (uuid.NewString id, reads name/agent_id/task_type/command/
  params/schedule), `handleSchedulerUpdateTask` (PUT), `handleSchedulerToggleTask` (POST
  toggle flips Enabled), `handleSchedulerDeleteTask` (DELETE).
- Registered in server.go auth group: `/scheduler/tasks` + `/api/scheduler/tasks` (GET/POST),
  `/scheduler/tasks/:id` (PUT/DELETE), `/scheduler/tasks/:id/toggle` (POST).
- Frontend fix: SchedulerPage read `a.agents` but `/agents` (Accept: application/json)
  returns `{data:[...]}` via handleListAgents → changed to `a.data || a.agents`
  so the agent dropdown populates.
- Verified via proxy `:3000/api/go/...`: create→id, PUT update, toggle, DELETE all
  return valid JSON; GET list `{"success":true,"tasks":[]}`.
- `/opsec/rules` now 511 bytes (was 1022 doubled) — confirms doubling fixed.

## Key context
- Frontend calls bare paths via proxy `:3000/api/go/<path>` → strips prefix → `:8080/<path>`.
- Login route is bare `/login` (form-encoded `username`/`password`), NOT `/api/login`.
  Test login: `curl.exe -s -c jar -X POST -H "Content-Type: application/x-www-form-urlencoded"
  -d "username=admin&password=admin" http://127.0.0.1:3000/api/go/login`
- The proxy (`frontend/src/app/api/go/[[...path]]/route.ts`) is TRANSPARENT — it does NOT
  double. Doubling was 100% the backend NoRoute `HandleContext` re-dispatch. To verify a
  bare path is doubled, hit it WITHOUT an alias → compare len to the `/api/` variant.
- Models available: `Campaign`, `CampaignAgent`, `PhishingCampaign`, `AutoTagRule`,
  `OpsecHistory`, `AgentTag`/`AgentTagAssignment`, `Task`, `Implant`.
- PowerShell note: pass JSON POST bodies via `--data-binary @file.json` (inline `'{...}'`
  quotes get mangled → gin sees form data → "invalid request").
