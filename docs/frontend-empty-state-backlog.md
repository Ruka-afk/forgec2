# Frontend loading / empty / error state worklist

Scope: the live source tree is `frontend/src/pages` (the old `frontend/src/app`
tree no longer exists — any "src/app" path in older notes is dead). Rule of
record is `docs/frontend-style-conventions.md` §2: page/table empties use
`<EmptyState>` (compact in cells), charts may keep a single muted line of text,
errors use `ErrorState`/`DataError`, and **a failed load must NEVER be rendered
as "empty" or as a reassuring default**.

This is a DEFECT list, not a style grep. A hand-rolled muted `<p>` inside a chart
or a table cell is legitimate and intentionally absent here. Counts come from a
static audit on 2026-09-24 — re-verify a file before editing it.

## Already fixed (do not re-open)

- BOF repo tab: endless spinner in the empty branch → `<EmptyState>`
- BOF: phantom "Library" tab (no backend) removed; per-item import/rating UI
  removed; the URL-import contract now sends the required `filename`
- Scripting console: templates rewritten for the Goja engine, response
  unwrapping fixed, run history moved to the audit log
- Destructive actions: report/integration/profile/token delete now confirm; the
  last native `alert()` is gone (enforced by `check:native-dialogs`)
- Credential vault + Builds: a WebSocket refresh no longer replaces loaded rows
  with skeletons, and a failed refresh no longer blanks them
- Settings: an unread kill-switch status is no longer shown as "Safe", and a
  failed TOTP status query is no longer shown as "2FA disabled"
- campaign: the detail panel no longer spins forever when the stats request fails
- Mimikatz module availability is a tri-state (`true | false | null` = unknown,
  see `ModuleAvailability` in `lib/cred-quality.ts`). A failed module-list query
  used to render as "module missing" and disable the credential actions; it now
  renders as "status unknown" and stays disabled without asserting a falsehood
- listener detail + agent persistence: a failed fetch no longer renders as
  "listener not found" / "no such agent"
- agent config: a failed fetch no longer leaves `defaultConfig` rendered as the
  agent's real configuration
- Empty-state flash before first load: privesc, scanner, macros, ntlm, pivoting,
  chain, cloud, toolkit, settings/ModulesSection, profiles, dashboard
  ActiveMissions + AnalyticsView, ai session sidebar + playbook picker, automation
  RuleDialog, agent interact dock, password-spray vault

### Replacement writes — an unread payload must never unlock a POST

These are the dangerous ones: the form is prefilled from the server, so a failed
read leaves plausible-looking defaults and an enabled Save (or a whole-array
replace) that silently destroys real configuration.

- **Settings**: `/settings` is the source for the server/agent/malleable forms.
  `useSettingsData` now exposes `loaded` (true only after the first successful
  read) and `SettingsPage` blocks the save buttons and gates the
  profile/database/about/certificates tabs on it. A later failed refresh shows a
  "showing last known values" banner instead of blanking the page.
- **Malleable response headers (real data-loss bug, two sites)**: the backend
  returns `malleable_headers` from `/settings`, but both `useSettingsData` and
  `useProfilesData` hard-coded `headers_text: ""`, so the textarea always looked
  empty and every save of that form wiped the configured response headers. Both
  now read the server value; regression tests in `useSettingsData.test.ts` and
  `useProfilesData.test.ts`.
- **Notification targets**: "Save all" POSTs the whole array, so a failed target
  read followed by "add one + save" deleted every existing target. Add/Save are
  now disabled while the list is unread; `NotificationRoutesCard` got a
  `DataError`.
- **Profiles active config + the page's own malleable form**: unread config no
  longer renders as "disabled / 0s / 200" and the save is blocked.
- **Domain fronting**: add/remove/toggle all POST the whole domain list, so a
  failed status read plus "toggle auto-failover" would POST `domains: []`.
  `configUnread` now blocks every mutator and the active-domain card shows
  Unknown.
- **Circuit breaker**: the page never consumed `useApiResource.error`, so any of
  the three queries failing rendered zero listeners and zero events — now a
  full-page `DataError`. Thresholds are prefilled, so both the page modal and
  `listeners/components/ListenerBreakerConfigDialog` block Save until loaded.
- **AI config**: a failed read left `enabled=false` / `hasApiKey=false` / default
  provider, so a working setup read as disabled and saving would clobber the
  execute policy. `useAIConfig` now exposes `configLoaded`/`configError` and
  refuses to save.

### False-safe status — never report unread state as safe

- **Listener health**: a failed circuit-breaker detail query used to leave
  `healthByTarget={}` so the listeners page said "0 burned" and the dashboard
  said "All healthy". `useListenersData` now exposes
  `healthLoading`/`healthError`; the burned tile shows "…", the table's
  agent-count column shows Unknown, and `OpsHome` reports per-source
  `partialFailures` instead of silently substituting empty arrays for the five
  feeds that failed.
- **Agent AV detection**: a failed hostinfo probe used to render "no security
  products detected". `AgentHeader` now takes `avStatus`
  (`"unknown" | "empty" | "products"`) and only makes the no-AV claim when the
  probe succeeded.
- **Cookie proxy + SOCKS relay**: a failed status query used to look like
  "stopped" (and offered to start a second relay). Both are now unknown states
  with the start/stop control disabled.
- **Agent OS detection**: a failed or unrecognised OS report defaulted to
  Windows, which sent `C:\` paths and Windows path semantics to Linux agents
  through the file browser (delete/rename/move/upload all join paths with it).
  `detectOs` now returns `null` and the browser is blocked; `shell-ui` gained
  `isKnownOs()` and an unknown-OS prompt/interpreter path (see
  `shell-ui.test.ts`).
- **AI pending approvals**: `PendingAIIntents` / `PendingAITasks` no longer
  render "nothing awaiting approval" when the approval query fails.

### False-zero figures

- **Attack coverage**: a failed read rendered a real-looking `0/0` and `0%`. The
  summary now shows "—", with a retryable `DataError` and an unknown agent-list
  banner.
- **Report**: overview failure used to yield a clean-looking all-zero assessment,
  and the IOC tab's failed extraction read as "No indicators extracted". The hook
  now exposes `overviewError`/`previewError`/`historyError` and `stats` is `null`
  when unread; `IOCTab` distinguishes a failed scan from an empty one.
- **Lateral / timeline**: four and three sources respectively became empty arrays
  on failure, so the stat cards and source chips reported real-looking zeros.
  Both now track per-source `partialFailures`; timeline marks a failed source's
  chip with "?" instead of 0.
- **Topology**: a failed *first* load left all three graph sources `null` and
  rendered three authoritative-looking empty graphs. Transient poll failures
  still keep the last good graph (preserving zoom/pan); only a first-load
  failure shows a retryable `DataError`.

### Silently empty lists and pickers

- **Agent list (shared `useAgentList`)**: 7 consumers discarded its `error`, so
  target selectors and command-palette agent entries vanished as if no agents
  existed. New `components/AgentLoadError.tsx` is wired into chain, cloud,
  bloodhound, container, tasks and the command palette.
- **tokens / roles / toolkit / stager / packer / settings ApiKeysSection**:
  option and library lists now surface a retryable error. Stager disables token
  registration while the listener list is unread (an empty picker there is not
  "no listeners"), and the packer's whole artifact form is disabled rather than
  presenting empty dropdowns that look authoritative.

## Class 1 — a loading branch that can never resolve

(none outstanding — the campaign case is fixed)

## Class 2 — empty state shown before the first load resolves (flash)

(none outstanding — see the fixed list above; re-audit before adding more)

## Class 3 — a failed fetch presented as empty / as a safe default

The "lies to the operator" half is done. What remains is MEDIUM: these make a
list look empty or a picker look bare, but the surrounding page is not making a
safety claim and no destructive write is unlocked by the wrong value.

- `settings/{SIEMRulesSection, ExtC2Section, BackupSection, ModulesSection}`
- `generate/components/BuildHistorySection` + `usePayloadGenerator` (presets
  fall back to hard-coded defaults after a failed profile read)
- campaign (list + the MITRE/template `Promise.all`), cloud results, bloodhound
  (results and status are tracked together), autotag
- macros library/history, scripting (agent/library/history), bof
  library/results
- agent detail token page + `TimelineSection`, `agents/components/useAgentData`
  (lock + tag lists), `AgentDockShot`, topbar notification dropdown
- `builds/components/EffectivenessCard`, `useSavedViews`, automation
  `ScheduledRulesCard` + `OneShotTasksCard`, `ai/AIContextPanel`
- opsec is a separate page worth a look; the audit did not confirm a site there
- `plugins/**` is notes-only (re-signing constraint)

Suggested order: the settings panels (they sit next to code already fixed in
this file), then the agent-detail token/timeline, then the rest. Prefer
migrating a page to `useApiResource` (it already retains data on refresh errors
and keeps background refreshes silent) over hand-rolling another
`useState`/`useEffect` loader. For a page that already uses `useApiResource`, the
fix is usually just consuming its `error` and rendering `DataError`.

## Two cross-cutting traps found while fixing this list — check for them first

1. **A FAILED read being cached as a valid answer** (see `lib/ai-status.ts`,
   fixed). Look for any module-level cache that writes its error fallback into
   the success slot.
2. **A fetch or a state reset called DURING render.** Three components did
   `if (loading === null) { void fetch(); return null; }`. It works by accident
   and turns any failure into a render loop. See `useAIStatus.ts` and
   `ExecutionHistoryDialog`.
