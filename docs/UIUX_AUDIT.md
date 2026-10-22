# Frontend UI/UX audit (2026-10)

Evidence base for the UI/UX program agreed in 2026-10. Scope: the operator
console in `frontend/` (~54 routes, ~200 page components) audited as **polish
within the existing design language** — consistency, accessibility, feedback,
motion and narrow-screen quality. Out of scope by decision (do not re-open):
information-architecture changes, page re-layouts, a new component system.

Defect clusters below carry an owner batch (U1–U6) and the evidence that
reproduces them. Both scanners are hard gates inside `npm run check` now —
`check:contrast` since U1b, `check:aria` since U2 closed K1–K5.

## Method

- **Static scan** — `frontend/scripts/check-contrast.mjs`: parses the token
  blocks in `globals.css`, builds every pair the UI renders (solid fills,
  8–20 % tinted surfaces over background/card, text-on-surface, charts, rings)
  and computes WCAG ratios; `--selftest` pins the color math (white/black = 21,
  `#777` cases, alpha compositing). Audit-time reading: 288 pairs across
  light+dark, **114 text pairs below 4.5** (≈10 root causes with many usage
  variants), 2 non-text below 3.0, 6 informational (input/border boundaries).
  After the U1 batches: 304 pairs, 0 text failures.
- **Browser cross-check** (temporary harness, not committed): the exact
  Tailwind class combinations were rendered in Chromium against the built CSS
  and measured from canvas pixels — the browser's own oklch/color-mix
  compositing — to confirm the static math. Where the two disagree the browser
  is authoritative; U1a switched the script to compositing alpha tints in the
  same gamma-encoded sRGB space browsers blend in (agreement within ~0.1).
- **Runtime probe** (temporary Playwright spec, not committed): tab-order walk
  and screenshots for dashboard / agents / listeners / settings at 390 and 1280,
  light+dark, against the static export with mocked APIs (same `prepare()`
  pattern as `e2e/layout.spec.ts`).
- **Inventories**: ripgrep counts over `frontend/src` (excluding tests) for the
  feedback / loading / dialog / keyboard / motion primitives; numbers inline.

## Already done — do not re-open

Empty/loading/error trilogy closed (Class 1–3, `docs/frontend-empty-state-backlog.md`);
design-token gates (icon sizes, spacing, raw hex, radii, shadows); i18n key parity gate
(4,714 keys); native-dialog gate; `focus-visible` global ring; skip link; global
`prefers-reduced-motion` kill switch; route-level Suspense + hover prefetch; overflow
assertions on 8 routes × 4 viewports; dual-theme × dual-platform visual baselines;
`useMutation`/`useErrorToast`/`useConfirm` patterns established (adoption gaps are
tracked below as defects, not regressions).

## D1 — Contrast (owner: U1)

> **U1a landed (2026-10-18):** C1, C2, C5, C6, C8 and C9 are fixed at the
> token/primitive layer, and the component side of C3, C4 and C7 moved to the
> new `-ink` / `-solid` roles (badge/banner/alert/button/dropdown variants, the
> shell components, `statusStyles`, task renderers).
>
> **U1b landed (2026-10-18):** the page-level migration is done — 300 bare
> `text-X` call sites in 104 files became `text-X-ink`, 38 `-foreground`-on-tint
> chips moved to ink, the light `primary/20` text chips/hovers became `/15`,
> and `check:contrast --strict` is now a member of `npm run check` (304 pairs,
> **0 text failures**). The single deliberate exception is the
> `ScreenVideoPlayer` overlay, which paints the bright base hues on a dark
> video surface.

### C1 · `text-destructive-foreground` is a phantom utility (6 sites)

No `--destructive-foreground` token exists (checked `globals.css` + `@theme`;
the built CSS has no `.text-destructive-foreground` rule), so the class is
silently dropped and the label inherits `foreground` on solid `bg-destructive`.
Browser-measured: **light 3.69:1, dark 3.01:1**; switching to `text-white`
still fails dark (3.70:1 — the dark destructive fill L 0.64 is too bright for
white). Call sites: `TrafficPage.tsx:151`, `AgentBulkBar.tsx:47/48/60`,
`InfrastructurePageContent.tsx:165`, `ProfilesPageContent.tsx:526`.
Fix: define the token (and/or retune the dark fill) in U1.

### C2 · Light success/info badges are white-on-pale-tint (≈1.2:1) — worst finding

`Badge` variants `success`/`info` render `bg-X/15 text-X-foreground` in light
mode, and the light `X-foreground` tokens are near-white (oklch 0.985):
browser-measured **1.20:1 / 1.20:1** — the label is effectively invisible.
Used across chain/domain-fronting/stager/generate/builds/pivoting/plugins/tokens/ai
pages (`variant="success|info"` grep: 20+ sites). The light `X-foreground`
tokens are tuned as "text on bright fill" while the badge uses them as "text on
pale tint" — the token system needs either text-safe light status values or a
separate ink token (U1 design decision).

### C3 · Light status colors as text on tints and page (2.2–3.6:1)

`text-warning` 2.19–2.54, `text-success` 2.83–3.51, `text-info` 2.91–3.60 in
light mode (on background/card and on their own 8–20 % tints). Call sites:
`TrafficPage.tsx:119–123` (method pills), `UsersPage.tsx:57/163/168/175/180`,
`banner.tsx:8–11`, `taskRenderers.tsx:109–111`, `AgentStatusBanner.tsx:49/77/81`,
`Sidebar.tsx:183`. Dark mode is fine for the same pattern (7.3–8.8:1) — only
light needs the retune.

### C4 · Dark badges (`bg-X/20 text-X`) sit at 3.3–4.2:1

The `dark:` half of the badge variants uses the status token as text on its own
20 % tint: info ≈ 3.3, success ≈ 3.4, warning ≈ 3.5–4.2 — below 4.5 for 12 px
labels. The dark token set is close; a small lift (or /15 instead of /20) fixes it.

### C5 · Dark solid warning button at 1.75:1

`bg-warning text-warning-foreground` with the dark palette = light-on-light.
Call sites: `CampaignPageContent.tsx:399` (progress chips);
`InfrastructurePageContent.tsx` config buttons. Light mode is fine (4.86:1).

### C6 · Light solid success/info buttons at 3.36/3.44:1

Same buttons' light half: white ink on L 0.61/0.62 fills. Call sites:
`InfrastructurePageContent.tsx:159` (nginx), campaign progress chips.

### C7 · Destructive tinted text — light 3.75, dark /20 variant ~2.8–3.3

`bg-destructive/10 text-destructive` (alert, error-state, badge, button,
dropdown destructive item, `AgentStatusBanner.tsx:43`, `TopBar.tsx:88/93`,
`UsersPage.tsx:183/186/549`): browser-light 3.75:1; the dark `/20` variant is
worse (static ~2.8–3.3 — remeasure in U1 with the browser harness). Also
`text-destructive` on plain background is light 4.47 — a nudge, not a redesign.

### C8 · Active nav item light 4.4:1 (borderline)

`Sidebar.tsx:154` `bg-primary/12 text-primary` — 12 px label just under 4.5 in
light; dark 5.84 OK.

### C9 · Avatar initials 4.16–4.24 (both themes)

`avatar-4` / `avatar-7` foreground pairs. One L nudge closes it.

### C10 · Informational (no fix required unless strict)

`chart-4` light 2.39–2.55 vs card/background (series distinguishability);
`border`/`input` boundaries 1.2–1.35 vs surfaces (industry-common; only elevate
if we adopt strict WCAG 1.4.11 for control boundaries).

## D2 — Keyboard & accessible names (owner: U2)

> **U2 landed (2026-10-18):** K1–K5 are fixed and `check:aria --strict` is a
> member of `npm run check` (184 triggers scanned; zero unnamed, zero
> title-only). K1/K2 got `aria-label`s (new `agents.screen_play` /
> `screen_pause` / `screen_fullscreen` EN+ZH keys; the phase chip now carries
> the full phase name), K3 names the icon-only breadcrumb home link from its
> item label, K4 deletes the static `index.html` skip link (the localized
> React one stays as the single pre-content stop), and K5 gives the `Table`
> primitive an optional `label` that turns the scroll container into a named
> region (`role="region"` + `aria-label` + explicit `tabIndex=0`, so the
> horizontal scroll is keyboard-reachable beyond Chromium) — the listeners
> table passes `t("listeners.title")`.

- **K1 · 2 icon-only buttons with no name** — `ScreenVideoPlayer.tsx:69`
  (play/pause) and `:78` (fullscreen). `npm run check:aria` scans 184
  icon-sized triggers: these 2 missing, 1 title-only, 2 named by `sr-only`
  (dialog/sheet closers ✓).
- **K2 · Title-only trigger** — `CampaignPageContent.tsx:388` (phase chip):
  `title` is not a dependable accessible name.
- **K3 · Breadcrumb home link has an empty name** (WCAG 2.4.4) —
  `breadcrumb.tsx:70` renders an `aria-hidden` Home icon with no text and no
  `aria-label`; found by the tab walk on every listed route. Screen readers
  announce "link".
- **K4 · Two skip links** — `index.html:36` ("Skip to main content", pre-hydration)
  and `AppLayout.tsx:96` ("Skip to content", React) both survive hydration,
  costing two tab stops before content. Keep one (or hide the static one after
  mount).
- **K5 · Scrollable table container has no label** — listeners' table region is
  correctly focusable (`tabIndex=0` wrapper) but its accessible name computes
  from all cell text; a short `aria-label`/`aria-labelledby` would do.
- Context: 576 `aria-label`s, 31 `onKeyDown`s, `aria-sort` in `data-table.tsx`
  + `AgentsTable` + `TasksPage` — the primitive layer is right; the findings
  above are the uncovered tail.

## D3 — Feedback loop (owner: U3)

> **U3 landed — wave 1, the settings tree (2026-10-18):** the mutation and error
> conventions are no longer single-sample. All 22 hand-rolled mutation handlers
> in `pages/settings/**` moved onto `useMutation` — 8 in `SettingsPage.tsx`
> (agent / server / malleable saves, password change, JWT regeneration, vacuum,
> backup download, purge), plus `useTOTP` (×3), `CertificatesSection` (×2),
> `EmergencySection` (×1), `BackupSection` (×2), `ModulesSection` (×3),
> `NotificationsSection` (×1), `SIEMRulesSection` (×1) and `ApiKeysSection`
> (×1) — replacing ten hand-rolled boolean flags with per-action `isPending`
> and the hook's stale-completion / unmount guards. The sections' shared
> `saving` prop stays (derived as the OR of every settings mutation plus
> `totpSaving`), so the disabled-state behaviour is unchanged; the hand-rolled
> 10-second `withSaveTimeout` wrapper and its `settings.toast.save_timeout`
> key are retired (the API layer's own 30 s timeout plus `toastError` cover the
> stuck-button case). Errors route through `useErrorToast`, extended with an
> optional `fallbackParams` argument (the `{ action }` emergency template
> interpolates through it): the last four raw `err.message` toasts in the tree
> are gone, the three settings saves that previously failed **silently** (the
> old wrapper let the rejection escape as an unhandled rejection, no toast)
> now report, SIEM toggle/delete no longer swallow failures, and the SIEM save
> failure shows its own key instead of the ExtC2 copy-paste
> (`channel_config_failed`). Counts: `useMutation` call sites 6 → 28,
> `useErrorToast` importing files 2 → 12, settings-tree raw-message error
> toasts 4 → 0. Per-row identity spinners (`restoring` / `downloading` /
> `savingId` / `testingId` / `testingIdx`) intentionally keep local state —
> the hook tracks pending-ness, not which row was clicked. Remaining clusters
> (agents detail, users, persistence, phishing, infrastructure, …) migrate in
> later waves; the conventions are unchanged.

> **U3 landed — wave 2, the agent-detail console (2026-10-18):** the three
> action hooks behind `/agents/:id` moved onto `useMutation` +
> `useErrorToast`. `useAgentNotes` loses its hand-rolled `saving` flag and
> `mountedRef` (the hook's stale-completion and unmount guards replace both;
> save failures resolve through `normalizeError` instead of a pre-translated
> string). `useAgentQuickShell` keeps its Enter-key `sendingRef` guard — the
> hook supersedes stale callbacks but does not drop concurrent calls, so the
> ref is what stops a double command — while `sending` becomes `isPending`.
> `useAgentDangerActions` replaces the identity `busy` string with a boolean
> OR of five per-action pendings (kill / uninstall / set / clear kill date /
> migrate); the ten-string `messages` object is gone (successes resolve from
> `t()` inside the hook) and each failure key routes through `toastError`,
> keeping the `await run() → keep-the-dialog-open-on-failure` contract. In
> `AgentDetailPage`, Diagnose and Apply-sleep are mutations too, and the
> header quick-action handler's generic failure routes through
> `toastError(..., { label })` (its 409 warning and `logger.error` stay).
> Route-id semantics sharpen: both per-agent hooks call `reset()` from their
> id effect, so a completion that lands after the operator switched agents is
> discarded — it can no longer close the new agent's notes editor, push the
> old command's result into the new agent's history, toast "sleep updated"
> with the wrong hostname, or leave the old send's guard blocking the new
> agent. The header's per-button spinner and the persistence page's
> per-method spinner keep local identity state by design. Coverage: new
> `useAgentNotes.test.ts` and `useAgentQuickShell.test.ts` plus the extended
> `useAgentDangerActions.test.ts` (12 hook tests; the two abandon cases were
> verified red against the pre-reset form). Counts: `useMutation` call sites
> 28 → 37, `useErrorToast` importing files 12 → 15, hand-rolled error toasts
> in the console's action paths 9 → 0. `check-i18n` now also recognises
> `toastError(err, "key", params)` — the optional third argument wave 1
> introduced (two settings call sites were previously exempt only via the
> `settings.` dynamic prefix). The route-id reset rule is written into
> `docs/FRONTEND_CONVENTIONS.md`; remaining clusters (users, persistence,
> phishing, infrastructure, the collect/stealth sections) migrate in later
> waves.

> **U3 landed — wave 3, the users page (2026-10-18):** the operator
> management surface moved onto `useMutation` + `useErrorToast`. Nine
> mutations replace the page's hand-rolled paths — create / update /
> password submit (which also gain the missing pending guards: a second
> click lands on a disabled button), force-logout, kick, session revoke,
> revoke-all, and the row-scoped toggle/delete calls. The toggle/delete
> identity spinner (`actionLoading = uid + "_toggle" / uid + "_delete"`)
> stays in local state by design — the hook tracks pending-ness, not which
> row was clicked — while those calls still route failures through the
> normalizer. The sessions dialog's `revokeLoading` string is retired:
> "one revoke at a time" is `revokingSession || revokingAll`, so a
> completion that lands after a newer revoke cannot re-enable the buttons
> early, and the promise-chain row handler becomes a plain mutation call.
> All ten raw `toast.error(t(...))` sites on the page (including the
> secondary roles fetch) now surface the normalized message instead of the
> generic copy. Coverage: new `UsersPage.test.tsx` (3 component tests —
> the double-submit guard, a 409 create surfacing "username already
> exists", a failed toggle reporting its message while the row spinner
> stays local; all three were verified red against the pre-migration
> page). Counts: `useMutation` call sites 37 → 46, `useErrorToast`
> importing files 15 → 16, users-page raw error toasts 10 → 0.

> **U3 landed — wave 4, the persistence page (2026-10-18):** the
> `/agents/:id/persistence` surface moves onto `useMutation` +
> `useErrorToast`. The three action paths — list, install, remove — become
> mutations; the per-method spinner (`actionLoading = method` /
> `` `remove_${method}` ``) keeps local identity state by design, while the
> calls route failures through the normalizer, so a rejected install now
> shows the backend's message (the old `formatThrownError` path rendered
> `[object Object]` for a status-carrying rejection) and the three
> `data.success` else-branches drop out: `api.post` already throws an
> `ApiError` carrying the envelope's `error` text, which `toastError`
> surfaces as the message. The list poller trades its shared
> `listCancelledRef` boolean for a monotonic run id: a boolean has to be
> reset for the new run, which *un-cancels* the old one — its 10×1.5 s loop
> kept polling the previous agent's tasks and could paint that output onto
> the new agent's page after a route-id switch (pinned by the fourth test).
> The refresh control shows a spinner while a run is in flight without
> disabling (a re-click supersedes the run instead of stacking a second
> poll), and the route-id effect now resets the whole page identity — agent,
> `loadError`, queued methods, detection output, spinner and all three
> mutation guards — because `/agents/A/persistence →
> /agents/B/persistence` reuses the instance (no per-id key on the route)
> and B briefly rendered A's queue. The duplicate load-failure toast is
> retired: the inline `DataError` + retry is the surface (the config/traffic
> page loads do not toast either), so the one translated `toast.error` left
> on the page is the version-blocked validation, which stays direct by
> convention. Coverage: new `PersistencePage.test.tsx` (6 component tests —
> the mount list plus its polled output, the confirmed install with its task
> id, a rejected install surfacing "agent offline" while the local spinner
> clears, the confirmed remove, the route-id reset and the superseded-run
> output guard; the last three were verified red against the pre-migration
> page). Counts: `useMutation` call sites 46 → 49, `useErrorToast` importing
> files 16 → 17, persistence-page raw error toasts 7 → 0.

> **U3 landed — wave 5, the phishing page (2026-10-18):** the whole
> `/phishing` surface moved onto `useMutation` + `useErrorToast` — the
> page's template save/delete, campaign save/launch/stop/delete and the
> Entra identity panel's four actions (device-code start/poll, consent
> start/exchange) became ten mutations. Every rejection now surfaces the
> backend's message through the normalizer: the page's six API-failure
> toasts route through it (the launch path's old
> `e instanceof Error ? e.message` fallback printed the generic key for a
> status-carrying rejection, which is what the API layer actually throws),
> and the panel's four `entra_failed` toasts, which swallowed every
> reason, surface it too. The panel's shared `busy` boolean became the OR
> of four per-action pendings: two actions leaving in one commit window
> (both buttons enabled before either pending flushed) previously let the
> first completion re-enable all four buttons while the second request was
> still in flight — the OR keeps them disabled until every request settles
> (pinned by test, verified red against the old boolean). The load-failure
> surface is real now: the old path toasted once (throttled) and rendered
> empty tables that read as "nothing configured"; the page shows the
> inline `DataError` + retry the config/traffic loads use, and
> `POLL.toastThrottle` is no longer imported. Stop failures gain their own
> copy (`phishing.toast.stop_failed`, EN+ZH) instead of reporting "Failed
> to save campaign". The launch identity spinner (`launchingId`) stays
> local by design and the template-required validation toast stays direct
> by convention. Coverage: new `PhishingPageContent.test.tsx` (7 component
> tests — the inline load error + retry, a rejected template save
> surfacing the backend message with the dialog left open, the successful
> save close+refresh, the launch double-click guard with the rejected
> launch's message, a rejected stop, the confirmed delete + refresh, the
> direct validation toast) and new `EntraIdentityPanel.test.tsx` (3 tests
> — the rejected device-code start message, the two-actions-in-flight OR
> race, the polled token JSON render); the load-error, save-rejection,
> launch-rejection, stop-rejection and both panel cases were verified red
> against the pre-migration code. Counts: `useMutation` call sites 49 → 59,
> `useErrorToast` importing files 17 → 19, phishing raw API-failure toasts
> 10 → 0.

> **U3 landed — wave 6, the infrastructure page (2026-10-18):** the whole
> `/infrastructure` cluster moved onto `useMutation` + `useErrorToast` — the
> page's five redirector actions (save / delete / test-ssh /
> generate-then-save / deploy-ssh) and the config hook's three (proxy config
> generate / ACME provision / profile export) became eight mutations, retiring
> the hand-rolled `try/catch`/`finally` flags and the four pending booleans
> (`saving`, `deploying`, `testingSSH`, `generatingModal`) they carried. The
> row-scoped delete spinner (`deleting === rd.id`) keeps its identity in local
> state by design and the clipboard-copy toast stays direct (a local
> capability failure, not an API error). Rejections surface the backend's
> message: the save/delete/test-ssh/generate-modal/provision/export toasts
> route through the normalizer, and two stale-data paths close — two
> overlapping config generations can no longer let the first response
> overwrite the newer output (the mutation guard keeps only the latest;
> pinned red), and a failed generation no longer passes silently as a server
> response (the local fallback template still renders, but the failure is
> reported alongside it). The SSH probe's `data.success` else-branch was
> dead — `api.postJson` already throws an `ApiError` for a `success:false`
> envelope — so it is removed with its now-unused
> `infrastructure.toast.ssh_failed` key; the success toast had been reading a
> nonexistent `stdout` field and rendered "SSH OK: undefined" for a reachable
> endpoint (`message` now interpolates). The load failure stops doubling as a
> toast (the inline `DataError` + retry already carried the text), and the
> deploy dialog's failure banner renders a normalized message instead of the
> raw `e.message`. Coverage: new `InfrastructurePageContent.test.tsx` (9
> component tests — the inline load error with no duplicate toast, the
> rejected save surfacing "invalid host" with the button re-enabled, the
> confirmed delete surfacing the backend message, the rejected/successful SSH
> probe, the overlapping-generation race, the failed generation + fallback
> template, ACME failure and the provisioned-path banner, the normalized
> deploy banner; all nine verified red against the pre-migration code).
> Counts: `useMutation` call sites 59 → 67, `useErrorToast` importing files
> 19 → 21, infrastructure raw API-failure toasts 9 → 0.

> **U3 landed — wave 7, the collect and stealth sections (2026-10-18):** the
> shared dispatcher behind the recon/collect tabs moved onto `useMutation` +
> `useErrorToast`. `useCollectTask` — the hook that powers nine sections
> (Recon, Registry, Window, ScreenTrigger, BrowserHistory, WeChat history,
> Keylogger, Clipboard, Wallpaper) — loses its hand-rolled `try/catch` and
> `mountedRef`; `collect` and `fire` route failures through the normalizer,
> and the callers' pre-translated `errorText` becomes an i18n `errorKey`
> fallback (12 call sites), so a mapped status localizes and a 4xx surfaces
> the backend's own message instead of context copy that overrode every
> reason; the key is the last resort for a message-less failure. `fire` gains
> an options object (`body` / `successText` / `errorKey`), and a new
> `onSettled` clears the local `busy` identity on both paths. A switch
> `/agents/A → /agents/B` reuses the section instances (the route carries no
> per-id key), so the hook clears the busy identity and the stored dump and
> retires the in-flight generation, and a stale completion resolves
> `null`/`false` for its caller instead of handing the old agent's output to
> the new section (pinned red); the sections that keep agent-bound local
> state reset it (clipboard draft + history, registry drafts + confirm,
> screen-trigger match/watching, window/browser results + filters, keylogger
> badge, recon label). The four remaining raw-`e.message` sites in the same
> tabs moved onto the normalizer with their context keys — `EvasionSection`
> (stealth: the technique travels as a mutation argument, and the armed
> confirmation is dropped on a switch), `InjectSection`, `WebcamMicSection`
> (an empty capture keeps the direct local-capability toast) and
> `ProcessSection` — each retiring its hand-rolled `sending` flag.
> `ScreenTriggerSection` gains the `agents.trigger_failed` fallback key
> (EN+ZH), and `check-i18n` recognises `errorKey:` data. Counts:
> `useMutation` call sites 67 → 73, `useErrorToast` importing files 21 → 26,
> collect/stealth raw API-failure toasts 7 → 0. Coverage:
> `useCollectTask.test.ts` 7 → 14 tests (mapped-status localization, 4xx
> backend text, the message-less fallback, the switch discard, the first
> `fire` pins), plus four new suites verified red against the pre-wave code:
> `EvasionSection.test.tsx` (5), `InjectSection.test.tsx` (5),
> `WebcamMicSection.test.tsx` (5), `ProcessSection.test.tsx` (5).

> **U3 landed — wave 8, the agent sub-pages and the right-rail AI card
> (2026-10-18):** the tail named at wave 7 closed. `AISuggestCard`'s load and
> run become mutations — the 409 approval notice keeps the header
> quick-action's `toast.warning(err.message)` treatment, every other
> rejection routes through the normalizer with the `ai.toast.assist_failed`
> fallback, and an agent switch clears suggestions plus an in-flight load.
> `/agents/:id/config` pushes through a mutation (the dead `res.success`
> else-branch retires) and its load failure routes through the normalizer but
> keeps its toast: the inline `DataError` only renders before the first
> success, so a post-push refresh failure has no other surface (the wave-4
> note that the config load did not toast described the traffic page only).
> The route-id effect resets the form and the in-flight push.
> `/agents/:id/traffic` adapts and auto-adapts through mutations, retires the
> duplicate load toast into the full-page `ErrorState` it doubled, deletes
> the dead adapt else-branch with the now-unused `agents.traffic_adapt_error`
> key, disables the checkbox while its toggle is in flight, and clears the
> report on a switch. `/agents/:id/token` migrates its seven actions (steal /
> make / revert / drop / impersonate / whoami / note) with `activeAction`
> keeping the row identity locally; the whoami banner gains an explicit
> `whoamiError` flag, because `startsWith("Error")` read backend text to
> decide the styling — a localized failure rendered as success — and now
> shows the normalized message; the switch clears the PID / user / domain /
> password drafts, the note drafts and the banner. `useScreenMonitor` routes
> capture / start / stop / title-trigger failures through the normalizer
> (new `screen.capture_failed` key, EN+ZH), keeping the busy/abort
> machinery, the match validation and the WebSocket `screen_stream_error`
> event text; `useRemoteDesktop` does the same for start / stop /
> remote-click, keeping the version gate and the silent background poll. Two
> files were audited and left alone with reasons recorded: `ShellPage` (its
> shell failures already render the inline banner; the silent branch is a
> quota guard) and `useAgentFiles` (the raw site is a stream `error` event
> payload, not a promise rejection). Counts: `useMutation` call sites
> 73 → 85, `useErrorToast` importing files 26 → 32, the tail's hand-rolled
> API-failure toasts 24 → 0 (the three direct `toast.error` sites left in the
> six files are two validations and one event text). Coverage: five new
> suites — `AISuggestCard.test.tsx` (4), `ConfigPage.test.tsx` (4),
> `TrafficPage.test.tsx` (5), `TokenPage.test.tsx` (4),
> `useScreenMonitor.test.ts` (4) — plus `useRemoteDesktop.test.ts` 3 → 5;
> all verified red against the pre-wave code (21 of the 26 suite tests
> failed without the change; the three pre-existing remote-desktop tests and
> the two unchanged success paths stayed green).

> **U3 follow-up — wave 9, the agents list dock/menu cluster (2026-10-19):**
> the wave-8 tail sentence closed: the helpers it recorded as never on the
> U3 map left their `e.message ?? generic` fallbacks. `AgentDockCommands`
> queues ps/screenshot/sleep and starts/stops the SOCKS relay through three
> mutations; the `busy` row identity stays local in `try/finally` so the row
> still disables as one, and a rejected relay start/stop surfaces the
> backend's own message under the `agents.dock_socks_failed` fallback.
> `AgentDockFiles` normalizes its load/pull/push failures; the per-row
> transfer fan-out stays hand-rolled by design — a single mutation's
> stale-completion guard would drop a sibling transfer's completion — so only
> the error routing changed. `AgentInteractDock` routes approve/reject/cancel
> through `toastError` and replaces the silent `setTasks([])` load failure
> with an inline `ErrorState` + retry (silent WS-down polling keeps the last
> list, so a reconnect cannot flash an error over a working dock).
> `useAgentMenuAction` normalizes screenshot/beacon-now (the clipboard
> failure stays direct — a local capability, not an API error).
> `useAgentBulkOps` routes its nine failure catches into the action banner
> through `errorText` and the CSV toast through `toastError` (the abort
> branch keeps its info toast); the page's bulk-results catch follows.
> The rule is single-sourced: `resolveErrorText` backs `useErrorToast` and
> the new inline `errorText(t, e, () => key)` (lazy fallback key), retiring
> the infrastructure page's local copy. Counts: `useMutation` call sites
> 85 → 88, `useErrorToast` importing files 32 → 37, the cluster's raw
> `e.message ?? generic` sites 6 → 0 and its reason-swallowing failure copy
> 15 → 0 (plus the task list's silent first-load failure, now reported).
> Coverage: `AgentDockCommands.test.tsx` (4), `AgentDockFiles.test.tsx` (4),
> `AgentInteractDock.test.tsx` (3), `useAgentMenuAction.test.ts` (4),
> `useAgentBulkOps.test.ts` 3 → 8, `errors.test.ts` 9 → 12; all new-behavior
> tests verified red against the pre-migration code (17 red / 18 green across
> the six suites, 35 tests).

> **U3 follow-up — wave 10, the last `formatThrownError` action surfaces
> (2026-10-19):** the tail recorded at wave 4 closes — `formatThrownError`
> renders a non-`Error` rejection as `[object Object]`, and every page still
> using it now routes through the normalizer. `NtlmPage` moves
> coercion / relay-start / relay-stop onto three mutations behind one derived
> pending (`loading` = the OR of the three `isPending`s — the retired shared
> bool's job — so a second request cannot start mid-flight); the `data.success`
> else-branches stay because a success-less envelope is a real shape
> (`unwrapBody` unwraps only `success === true` bodies), so the branch is not
> provably dead. `PivotingPageContent` normalizes the direct-SOCKS start
> rejection (new `pivoting.toast.direct_socks_failed`) but keeps its per-row
> busy identity hand-rolled in `try/finally`: rows fire in parallel, so a
> single mutation's stale-completion guard would drop a sibling row's
> completion — the wave-9 dock-files rule applied verbatim.
> `usePivotingData`'s six failure paths (five swallowed the reason whole)
> report through the normalizer; only the RPort start gained a new fallback
> (`pivoting.toast.rport_start_failed`), the other five keys stay.
> `ToolkitPage` normalizes quick-action dispatch and the template `onRun`
> path (new `templates.toast.run_failed`); both no-agent guards stay direct
> validation toasts and the success-less envelope branch stays.
> `TemplatesPanel` moves save onto a mutation (`saving` retires, the dialog
> keeps its pending guard) with the new `templates.toast.save_failed`, routes
> delete through `templates.toast.delete_failed`, and gives the silent load
> failure the wave-9 shape: inline `ErrorState` + retry under
> `common.load_failed` — the old `setTemplates([])` made a dead endpoint read
> as "no templates". Counts: `useMutation` call sites 88 → 92,
> `useErrorToast` importing files 37 → 42, `formatThrownError` action-surface
> call sites 11 → 1 (the survivor is `ShellTerminal`'s deliberate 409
> branch), and 5 new keys land in both locales (4721 → 4726 used keys).
> Coverage: `NtlmPage.test.tsx` (4), `usePivotingData.test.ts` (4),
> `PivotingPageContent.test.tsx` (2), `TemplatesPanel.test.tsx` (6),
> `ToolkitPage.test.tsx` (5); 11 of the 21 verified red against the
> pre-migration code (all five suites redden without the change). Test note:
> the existing `fireEvent.mouseDown` + `click` recipe opens a base-ui `Select`
> popup but never commits the item (the TokenPage helper never actually
> committed a selection and its assertions do not depend on it) — the
> committed recipe is the `pointerdown` → `pointerup` → `click` pair, which
> the new suites use.

> **U3 follow-up — wave 11, the `/ai` conversation workspace
> (2026-10-19):** the cluster's raw-message error sinks all route through
> the normalizer now — 18 sites across six files. `AIPageContent` normalizes
> six action surfaces: session rename and creation (both swallowed the reason
> whole), the fire-and-forget run-cancel (its `.catch(() => ...)` becomes
> `.catch((e) => toastError(...))`), branch creation, pin and archive, so a
> rejected pin surfaces the backend's reason or the mapped status copy under
> `ai.pin_session_failed` instead of the raw text. `AIAssistBar` routes the
> run-review rejection through `toastError` and normalizes the macro-load
> inline error through `errorText` (display-inert today — the picker only
> disables — but the raw read retires). `AIContextPanel` resolves its load
> failure once and re-uses the string for both the inline banner and the
> toast, and normalizes the upload / remove / policy-update toasts (the
> policy toggle still rolls back before reporting). `PendingAIIntents`
> normalizes the approve/reject failure. `useAIConfig` and `useAISessions`
> follow the same split as `AIContextPanel`: the config and session-list
> loads resolve once through `errorText` for the inline `configError` /
> `sessionsError` banner and re-use that string for the toast, while save,
> message-load and delete go through `toastError`. Counts: `useErrorToast`
> importing files 42 → 48, the cluster's raw `e.message` sinks 18 → 0
> (grep-verified); no new keys (4726 used keys unchanged). Repo-wide, the
> `x.message`-ternary tail now stands at 76 sites (12 toast-level, 33 inline
> `set*Error`, 31 helper) — the named candidates for later waves. Coverage:
> `AIPageContent.test.tsx` (4), `AIAssistBar.test.tsx` (2),
> `AIContextPanel.test.tsx` (4), `PendingAIIntents.test.tsx` (2),
> `useAIConfig.test.ts` 4 → 6, `useAISessions.test.ts` 3 → 6; 15 of the 17
> new tests verified red against the pre-migration code (all six suites
> redden without the change; the two contrast pins — backend 4xx text
> passthrough — stay green both ways). Test note: the page suite polyfills
> `Element.prototype.scrollTo` and `window.matchMedia` (jsdom ships neither;
> the message viewport effect calls both, the latter from deferred animation
> frames).

> **U3 follow-up — wave 12, the tasks and macros workspaces
> (2026-10-19):** the cluster's 13 failure sinks all route through the
> normalizer now — 11 raw-message ternaries plus two reason-swallowing
> catches, across five files. `TasksPageContent`'s task-list load resolves
> its failure once through `errorText` and re-uses the string for both the
> inline `DataError` and the toast, and the local `mutationError` helper
> behind its six task actions (cancel/rerun/approve/reject/claim/release)
> now goes through the same rule, so a rejected cancel surfaces the
> backend's 409/404 text or the mapped status copy; the two bulk fan-outs
> keep their deliberate per-item failure counting (the wave-9 fan-out
> rule). `AIAnalysisPanel` routes the analyze-result rejection through
> `toastError` while the success-but-empty-payload toast stays direct (not
> a thrown error). `NLQueryDialog` normalizes its ask rejection the same
> way. `AIPlaybookDialog` normalizes the generate and save rejections (the
> no-steps toast stays direct). `MacrosPageContent` normalizes its three
> load failures through `errorText` — the inline "library/runs/agents
> unreadable" banners keep their `{message}` template, with the existing
> `common.load_failed` fallback and no new keys — plus its save/run toasts,
> and the delete/stop paths that previously swallowed the reason whole
> (`catch { toast.error(t(...)) }` becomes `catch (e) { toastError(...) }`).
> Counts: `useErrorToast` importing files 48 → 52, the cluster's raw
> `e.message` sinks 13 → 0 (grep-verified); no new keys (4726 used keys
> unchanged). Repo-wide, the `x.message`-ternary tail now stands at 65
> sites (6 toast-level, 29 inline `set*Error`, 30 helper) — the named
> candidates for later waves. Coverage: `TasksPageContent.test.tsx` (3),
> `AIAnalysisPanel.test.tsx` (3), `NLQueryDialog.test.tsx` (2),
> `MacrosPageContent.test.tsx` (4), `AIPlaybookDialog.test.tsx` (3); 10 of
> the 15 new tests verified red against the pre-migration code (the five
> contrast pins — backend 4xx/409 text passthrough and the success-but-
> empty toast — stay green both ways). Test note: the two dialog suites
> wrap their trigger clicks in `act` so the already-rejected request's
> continuation (with its loading-state update) settles inside an act
> window, which keeps stderr free of React act warnings; the page suites
> polyfill `matchMedia` and `Element.prototype.scrollTo` (jsdom ships
> neither).

> **U3 follow-up — wave 13, the agent console cluster
> (2026-10-19):** the deliberate 409-approval-warning trio and the
> cluster's five inline raw-message reads now share one mechanism.
> `useErrorToast` gains `warnError`, the warning-toned twin of
> `toastError`: same normalization and message rules, different tone
> (and aborted requests stay silent in both). The three 409 branches —
> `AISuggestCard`'s run command, `useAgentQuickShell`'s quick shell and
> `AgentDetailPage`'s header quick-action — route through it, so the
> backend's approval text keeps its warning tone through the shared
> resolver, and a message-less 409 falls back to the surface's copy
> instead of toasting an empty string; the quick-shell history row
> resolves the same conflict reason through `errorText` with the toast's
> fallback key, so the row and the warning cannot disagree.
> `HostInfoCard` resolves its sweep failure through `errorText` — the
> cluster's one *displayed* string, and a plain rejection object used to
> render the generic copy for every failure, so a 5xx now shows the
> mapped status copy and a backend 4xx its own text. The display-inert
> raw reads retire through the same resolver: `TimelineSection`'s
> timeline load, `TokenPage`'s token-list and process-list loads, and
> `useAgentData`'s lock table (whose plain-rejection form used to store
> `[object Object]`), each state keeping only its presence-flag role.
> Counts: `useErrorToast` importing files 52 → 53; the 8 touched files'
> error-surface `.message` reads 9 → 0 (grep-verified; the three
> remaining `.message` reads are deliberate — the quick-shell response
> data field, the whoami display text and the hook's own doc comment);
> the repo-wide `x.message`-ternary tail now stands at 56 sites
> (3 toast-level, 23 inline `set*Error`, 30 helper) — the 9 retired
> sinks were part of the previous 65. No new keys (4726 used keys
> unchanged). Coverage: `useErrorToast.test.ts` 5 → 9,
> `AISuggestCard.test.tsx` 4 → 5, `useAgentQuickShell.test.ts` 3 → 6,
> `TokenPage.test.tsx` 4 → 5, `useAgentData.locks.test.ts` 3 → 4, new
> `HostInfoCard.test.tsx` (2) and `TimelineSection.test.tsx` (2); 9 of
> the 14 new tests verified red against the pre-migration code (the five
> contrast pins — the 409-with-text warning, the non-conflict history
> text, the token banner and the two timeline guards — stay green both
> ways). Test notes: `useAgentQuickShell.test.ts` gained the sonner and
> i18n mocks its 409 paths need; the `HostInfoCard` suite uses fake
> timers (the card auto-collects 800 ms after mount) and expands the
> SectionCard collapsible before asserting, because the error line lives
> in the collapsed body; its stderr carries the pre-existing
> nested-button dev warning from SectionCard's header markup, which this
> wave does not touch.

> **U3 follow-up — wave 14, the agent files workspace
> (2026-10-19):** `useAgentFiles` — the hook behind both
> `/agents/:id/files` and the dock's file panel — retires its 17 raw
> failure sinks, the largest single-file cluster on the U3 map: 15
> `String(err)` coercion sites plus the two `.message` ternaries behind
> the listing retry card and the OS-detection blocker. Twelve
> un-prefixed toasts route through `toastError` (listing, exfil,
> preview read, delete, mkdir, rename, chmod, upload push, drives, USB,
> find/hunt), the three batch fan-outs (batch delete's dispatch and
> poll phases, batch pull) resolve through `errorText` under their
> filename prefix, and both displayed states resolve through
> `errorText` with the same fallback keys as their toasts, so a toast
> and its persistent card cannot disagree. The drive surface gains its
> own failure copy (`agents.files_drives_failed`, EN + ZH — the
> operation previously had no failure key). Behavior: `Error: …`
> prefixes and object coercions retire; a status-carrying rejection
> shows the mapped status copy, a backend 4xx its own text, and the
> upload pump's rejected `AbortError` is silent. Counts: `useErrorToast`
> importing files 53 → 54; the touched file's raw sinks 17 → 0 and
> repo-wide `String(err)` sites 30 → 15 (grep-verified); the
> repo-wide `x.message`-ternary tail 56 → 53 (2 toast-level, 21
> inline, 30 helper); 4727 used keys (+1). Coverage: new
> `useAgentFiles.errors.test.ts` (8); all 8 new tests verified red
> against the pre-migration code, while the three existing suites
> (cache 8, upload 2, view 3) stay green both ways as controls. Test
> notes: the new suite mocks `getRateLimitRetryAfter` alongside
> `api`/`pollTask`, so the 429 path resolves through the same client
> hook the real app uses, and the abort-silence pin rides a rejected
> upload pump whose signal was never aborted.

> **U3 follow-up — wave 15, the settings tree's error surfaces
> (2026-10-19):** the family that opened the batch closes its residue —
> the eight list-bearing sections behind Administration retire their
> nine raw-message reads, which wave 1 left behind because they were
> display states, not toasts. The five single-surface loads (API keys,
> ExtC2 channels, notification routes, notifications, SIEM rules)
> resolve their unread warnings through `errorText`: mapped status copy
> first (401/403/429/5xx), backend text for other 4xx, the network copy
> for a browser `TypeError`, and only a message-less
> failure keeps the section's existing fallback key. The old code
> rendered `Failed to fetch` verbatim and, for an empty message, an
> empty string — which, being falsy, painted the empty state ("nothing
> configured") over an unread list. The three dual-surface sites
> (`BackupSection`, `ModulesSection`'s module list, `useSettingsData`)
> resolve once and reuse the string for both the inline warning and the
> toast, so the two cannot disagree; `ModulesSection`'s agent-list
> banner (the deploy selector's unread-list warning) follows the same
> rule. `ReloadStatusCard`'s probe stops swallowing the failure reason:
> the toast routes through `toastError` under the existing
> `settings.reload_status_failed` fallback while the card body keeps
> that copy for an unread matrix, so a dead backend is distinguishable
> from a failed poll. Verified and deliberately kept:
> `EmergencySection`'s kill-switch status and `useTOTP`'s 2FA status
> hold a translated presence flag whose string is never rendered (the
> UI gates on truthiness and renders its fixed unknown-state copy), so
> they are flags, not message surfaces; `SettingsPage`'s hash parse and
> localStorage reads stay silent by design. Counts: `useErrorToast`
> importing files 54 → 55; the touched files' raw `set*Error` reads
> 9 → 0 (grep-verified); the repo-wide `x.message`-ternary tail drops
> 53 → 44 (wave 14's ledger minus exactly the nine settings sites);
> 4727 used keys unchanged. Coverage: new `loadErrors.test.tsx` (13 — a
> five-section table plus three `ModulesSection` cases:
> dual-surface resolve-once, agent-list banner, message-less fallback on
> both surfaces), new `BackupSection.test.tsx` (2) and
> `ReloadStatusCard.test.tsx` (2 — the probe reason plus a success-render
> control), and `useSettingsData.test.ts` 4 → 8; 20 of the 21 new tests
> verified red against the pre-migration code, with the ReloadStatusCard
> success render as the control. Test notes: the section tests use a
> param-aware `t` mock (`key·{message}`) because the unread warnings
> embed the failure inside a template param, and the `ModulesSection`
> agent-list case must return one module — the deploy section (and its
> `AgentLoadError`) only renders once the module list is non-empty.

- **F1 · `useMutation` adoption: 6 uses vs 66 hand-rolled `setSaving`
  (15 files) + 8 `setDeleting` (3 files) + per-page `actionLoading` state.**
  The convention exists (`docs/FRONTEND_CONVENTIONS.md`) with one migrated
  sample (`ExtC2Section`); U3 migrates page by page (stale-completion and
  unmount guards are the concrete wins).
- **F2 · Error mapping**: `toast.error` 430 sites / 119 files vs `useErrorToast`
  6 sites — backend error-code → i18n-key mapping is not propagated. U3 decides
  per page where it pays.
- **F3 · Broad and healthy**: success toasts 202 (78 files), `useConfirm` 44
  (42 files), `Skeleton` 80 (29), `DataError` 59 (47), `EmptyState` 67 (53).

## D4 — Narrow screens & touch (owner: U4)

- **R1 · Sampled 390/1280 renders**: no horizontal overflow; empty states and
  toolbars wrap correctly; `Button` variants carry mobile `min-h-11` (44 px
  touch expansion) in the primitive. Baseline healthy.
- **R2 · Connection state on mobile is a dot only** — the TopBar status label
  is `hidden lg:inline`; below lg the state (live/offline/reconnecting) is not
  readable as text.
- **R3 · Overflow assertions cover the 8 baseline routes**; extend the list to
  the top-10 operator routes × 4 viewports in U4.

## D5 — Motion (owner: U5, landed 2026-10-18)

- **M1 · No route transitions** — `startViewTransition` has 0 call sites while
  `globals.css` already carries `::view-transition` rules *and* their
  reduced-motion override (lines ~480). Progressive-enhancement opportunity:
  add it where route chunks now swap instantly.
- Skeleton usage (80) and `animate-*` classes are consistent; the global
  reduced-motion kill switch covers third-party animation.

> **U5 landed (2026-10-18):** M1 is fixed in `frontend/src/router.tsx` — the
> module wraps `router.navigate`, the single funnel every navigation passes
> through (the `<Link>` sites and the programmatic `navigate()` calls stay
> untouched), defaulting `viewTransition` to `true` with
> `options?.viewTransition ?? true` (a `Link` forwards a present-but-undefined
> key that an unconditional spread clobbered — link navigations fired no
> transition), keeping an explicit `false` opt-out, and skipping the API under
> `prefers-reduced-motion: reduce` or where `document.startViewTransition` is
> missing. `globals.css` (the cross-fade rules and their reduced-motion
> override) is unchanged. Pinned by `src/router-transitions.test.ts` (the four
> wrapper decisions against a spied `createBrowserRouter`) and the Playwright
> `route transitions` spec (the real `/chat → /ai` client-side redirect on all
> four viewports: calls > 0; 0 under emulated reduced motion).

## D6 — Copy & i18n (owner: U6, per page)

- The i18n gate is green (4,714 keys, en/zh parity, literal-key check). No
  systemic defect found; terminology spot-checks ride along with the page
  batches.

### U6 evidence archived (2026-10-20)

The four U6 captures that were missing from the original review are archived
under `docs/design/u6/`. They predate the U7 vocabulary sweep, so they are
**before** evidence; the current copy is Beacon:

- [Payload Generator (laptop)](design/u6/generate-laptop.png): the capture
  still reads "Agent Binaries"; current `generate.agents_title` is
  "Payload Formats".
- [Post-Exploitation Toolkit (mobile)](design/u6/toolkit-mobile.png):
  "Target Agent:"; current `toolkit.target_agent` is "Target Beacon:".
- [Network Topology (mobile)](design/u6/topology-mobile.png): "Agents will
  appear here..."; current `topology.empty_hint` is "Beacons will appear here
  when they connect".
- [Privilege Escalation (mobile)](design/u6/privesc-mobile.png): no banned
  term; retained as visual evidence.

## D7 — Visual consistency (owner: U6 / U1)

- **V1 · Dashboard card headers**: the lower "Sessions" / "Listener health"
  cards carry a blue gradient header band while the cards above are neutral
  (see the laptop-light probe screenshot). Confirm intent; if decorative it
  reads as inconsistency next to Quick actions.
- **V2 · Dark theme keeps light `--code-text` / `--code-bg`**
  (`globals.css:40–41` are only defined in `:root`; dark overrides only
  `--code-accent`), so markdown code blocks render light-on-dark. Confirm
  intent (paper-style blocks) or fix in U1.

## D8 — Type system (owner: U1c, landed 2026-10-18)

The highest-impact defect in the console, and the one the U1–U3 batches
structurally could not see: the contrast gate measures colour, the aria gate
measures names, and neither reads a font size.

### T1 · `text-(--fs-*)` was never a font-size utility (360 call sites, 113 files)

`text-<value>` is Tailwind's *font-size* utility only when `<name>` resolves
from `--text-<name>`. The codebase used the arbitrary-value spelling
`text-(--fs-compact)`, which compiles to `color: var(--fs-compact)` — a length
in a colour slot, an invalid declaration the browser discards silently. Every
call site therefore inherited 14/15px instead of the documented ramp.
Browser-measured before/after on `/agents`:

| Element | Class | Before | After | Intended |
| --- | --- | --- | --- | --- |
| Page title (`main h1`) | `text-page-title` | 15px | **24px** | 24px |
| Table header (`th`) | `text-micro-sm` | 14px | **10px** | 10px |
| Badge / kbd chip | `text-micro` | 14px | **10px** | 9px |
| Card section title | `text-section` | 15px | **18px** | 18px |

Consequences beyond cosmetics: table headers were the same size as cell text,
so the header row lost its hierarchy; 12–13px metadata rendered at 14–15px and
inflated every dense table; and because the size never changed, the `density`
preference only ever moved two values that nothing consumed.

Fix: declare the ramp in `@theme inline` as `--text-micro` … `--text-prose`
(each pointing at its `--fs-*` so `[data-density="compact"]` still cascades),
and migrate the 360 sites with `scripts/migrate-fs-utilities.mjs`. New
`check-design-tokens.mjs` Rules 5/6 make both halves permanent: the broken
spelling is an error, and a missing `--text-*` declaration is an error.

### T2 · Empty-state message clipped inside table cells

`EmptyState` is routinely rendered inside a `TableCell`, which sets
`whitespace-nowrap` for data cells. The status message therefore refused to
wrap and was cut off mid-sentence ("Deploy your first implant to establish a…")
on every table-backed page (measured: `scrollWidth` 360 vs `clientWidth` 320 at
1280, and 371 vs 320 at 390). Fixed in the primitive with `whitespace-normal`
plus `text-balance`; the audit probe now reports zero clipped text nodes across
10 routes × 4 viewports.

### T3 · Focus mode occluded the page title

`[data-focus="on"] #main-content { padding-top: 0 !important }` assumed the
top bar was hidden, but the bar is `fixed` and stays. Measured: heading top
28px vs bar bottom 56px — the title sat entirely under the bar. Focus mode now
keeps the padding and reclaims the 224px sidebar column plus the breadcrumb
strip, which is the actual win, and only quiets the bar's border/shadow.

### T4 · Stat tiles stair-stepped

One- and two-word labels in the same grid wrapped differently, so values sat
35px apart (`BEACONS` one line, `QUEUED TASKS` two). The tile now reserves a
two-line label slot and pushes `trend` to the bottom with `mt-auto`; all five
dashboard values align on one baseline at both 1280 and 390.

### T5 · Dead density selectors

`[data-density="compact"] .agent-table …` targeted a class no component ever
rendered — compact mode changed the type ramp and nothing else. Re-keyed to the
table primitive's own `[data-slot="table-cell|table-head|table-row"]`, which
every table sets.

### T6 · No command-palette entry on touch devices

The palette trigger was `hidden … lg:flex` and the only other door was Ctrl+K.
On a phone the sidebar is a closed sheet, so global navigation had no entry
point at all. Added an icon-sized trigger below `lg`; the header at 390 now
exposes Menu / Search / Status / Notifications / User.

### T7 · Sub-32px touch targets

`SectionCard`'s "View all" link was a bare `text-xs` anchor (44×16px) on every
dashboard panel. Now a 32px-tall inline-flex hit area that keeps the header
height. The status pill was likewise 16px tall; it is a focusable button with
`min-h-8` and a visible focus ring. Measured on `/dashboard` at 390 with touch
emulation: small-target count 9 → 0.

### T8 · `-foreground/100` no-op modifiers

`text-muted-foreground/100` is identical to the unmodified class; 140
occurrences across 46 files were noise that made real opacity decisions harder
to spot. Removed mechanically.

### Visual baselines

The typography change moves every baseline by design, so both platform sets
were regenerated for `mobile` + `laptop` (win32 locally; linux via the
`visual-baselines.yml` workflow). The workflow now passes
`--update-snapshots=all`: the default `changed` mode only rewrites snapshots
whose diff exceeds the 1.5% pixel-ratio tolerance, so within-tolerance drift
would silently stay stale in the committed set. `/chat` was dropped from
`VISUAL_ROUTES`: it
is a client-side redirect to `/ai`, so it rendered the *same* surface under a
second name, and capturing it mid-redirect produced duplicated-content
baselines. `waitForPageSurface` also now waits for `networkidle` before the
height settle, so lazy chunks can no longer land between settle and capture
(this was the source of intermittent 3–4% diffs on `/ai`).

### T9 · Whole-page crashes from a non-total list fallback

`TokensPage` read its payload as
`tokenData.tokens || tokenData.data || tokenData` — when the response carried
neither key, the fallback yielded the **envelope object**, and the next
`.filter(...)` threw `TypeError: x.filter is not a function`, taking the entire
page down through the ErrorBoundary. `NtlmPage` had the same shape
(`agentsData?.agents || agentsData`) and would have crashed on `.map`. Both use
a total helper now (`firstArray` from `lib/envelope.ts`, which returns `[]`
rather than a mistyped value); the fallback chain keeps its key priority.

Found by the widened route sweep, not by the e2e suite: the mocked envelope in
`layout.spec.ts` happens to satisfy the old keys, so the crash never fired
under test.

### T10 · Unbounded strings clipped by fixed-width columns

`ToolkitPage` pinned task-type ids to `w-28`; `screen_trigger_start` needs
144px in a 112px cell and was silently cut. The column now sizes to content and
the description truncates instead. `ScriptingPage`'s API reference rendered
long signatures (`query('agents'|'tasks'|…)`, 374px) in an unwrapped block that
overflowed its card at both 1280 and 390; it wraps now.

## U7 — Professional naming & page emoji (wave 1, landed 2026-10-19)

The batch's copy debt closes in one vocabulary. Source of truth:
`docs/TERMINOLOGY.md` (core terms, nav-equals-title rule, emoji catalog, scope).
Program scope: **L1** operator-facing copy + **L2** frontend route URLs and page
directory/component names (wave 2); **L3** — HTTP API, sqlite schema, OpenAPI — is
deliberately untouched (`agents` stays the contract word; the UI translates).

1. **Terminology.** Chrome no longer mixes Session / Agent / Implant: it says
   **Beacon** — 45 nav labels × 42 page titles × EN/ZH, plus the dashboard tiles
   (Sessions → Beacons, "No sessions" → "No beacons"). Abbreviations were replaced
   with professional terms (Generate → Payload Generator, PrivEsc → Privilege
   Escalation, Exploit Toolkit → Post-Exploitation Toolkit, Network Map → Network
   Topology, Chain → Proxy Chains, Cloud → Cloud Credential Theft, NTLM → NTLM Relay
   & Coercion, User Mgmt → User Management, Asset organization → Asset Management,
   Agent Groups → Beacon Groups …). The ZH side mirrors it and repairs six
   mistranslations (链式攻击→代理链, 横向穿透→跳板, 截图收集→战利品, 事件流→事件,
   熔断器→断路器, 全局令牌库→令牌仓库).
2. **Emoji identity.** One emoji per page, identical across sidebar label, h1,
   breadcrumb and `document.title` (all read the same value). Decoration only:
   `aria-hidden` inside chips, Windows-safe single codepoints (no ZWJ sequences, no
   flags), never the sole carrier of meaning. Catalog lives in `TERMINOLOGY.md`.
3. **U6 residue closed.** DNS stat tile loses the stray `tone="primary"` (the
   six-tile listeners grid reads as one family); `CardHeaderRow` h2 `truncate` →
   `line-clamp-2` (the traffic card's "Beacon Com…" mobile truncation);
   `PrivescPage.CHECK_TYPES` carried eleven mojibake `"??"`/`"???"` icon literals →
   real emoji with `aria-hidden` and 2-line descriptions (the desktop ellipsis
   "Run all available privilege es…" retires); the dashboard geo legend's `"??"`
   fallback → 📍.
4. **Verified-resolved from U6 (no change).** V1: `SectionCard`'s default
   `headerAccent` is the app-wide accent language, not a per-page defect. V2:
   `globals.css` `.dark` overrides `--code-text/--code-bg`, so code blocks keep
   contrast. Probe noise ruled deliberate: sr-only labels, hidden select values, and
   single-line ellipsis.

**Method note.** The nav/title repaint fell **under** the baseline suite's
`maxDiffPixelRatio: 0.015`, and plain `--update-snapshots` only rewrites snapshots
whose diff exceeds that tolerance — the pass silently kept the old PNGs (it reported
green and wrote nothing). When the visual language itself changes, regenerate with
`--update-snapshots=all`, exactly what `visual-baselines.yml` documents for the linux
set; a sub-threshold diff is a pass by design, not evidence of staleness.

**Still open.** Wave 3+: body-copy sweep per page domain (descriptions, empty states,
toasts) still saying Agent/Session/Implant.

## U7 — Professional naming & page emoji (wave 2, landed 2026-10-19)

Wave 2 is the routing layer of the same program (the copy layer landed in wave 1
above; the source of truth stays `docs/TERMINOLOGY.md`).

1. **Six routes renamed, detail family included.** `/agents` → `/beacons`,
   `/timeline` → `/events`, `/generate` → `/payload-generator`, `/privesc` →
   `/privilege-escalation`, `/chain` → `/proxy-chains`, `/asset-organization` →
   `/asset-management`; the nine `/agents/:id/…` detail routes follow their list
   page to `/beacons/:id/…`. `navigation.ts` (six hrefs), `route-imports.ts` (six
   ids), `route-prefetch.ts` and `layout.ts`'s flush-path predicate moved with them,
   and every internal link (dashboard tiles, palette entries, status-banner filters,
   cross-page buttons, dock navigation, menu actions — 151 literals across 48 files)
   now points at the new slug. A post-change audit finds the old literals only in the
   redirect table and in references to backend endpoints (the dual-use
   `/agents/:id/…` API paths, `/toolkit/agents/…`, `/api/agents/…`), inline endpoint
   comments included — no stale client link survives.
2. **Old URLs keep working — through the router, not the server.** `router.tsx`
   gains a `LEGACY_REDIRECTS` table (15 entries: six top-level plus the detail
   family) rendered by one shared `LegacyPrefixRedirect` that swaps the first path
   segment and carries the id, sub-route, query string and hash through
   (`/agents?status=offline` → `/beacons?status=offline`). Because they are ordinary
   routes they are pinned by the same machinery as pages: the static-export lockstep
   test (`vite-plugins/staticExport.ts` lists both name sets; `agents.html` stays a
   shipped shell so a bookmark still boots the SPA) and `requiredPerms`. No backend
   edit was needed: gin's legacy page handlers are Next.js-era JSON renderers that
   browser GETs never reach (the static middleware answers navigations first), and
   the API / DB / OpenAPI layer is the program's L3 boundary.
3. **Directories and entry components follow.** `pages/agents` → `pages/beacons`
   (`BeaconsPageContent`), `timeline` → `events` (`EventsPageContent`), `generate` →
   `payload-generator` (`PayloadGeneratorPage`), `privesc` →
   `privilege-escalation` (`PrivilegeEscalationPage`), `chain` → `proxy-chains`
   (`ProxyChainsPage`), `asset-organization` → `asset-management`
   (`AssetManagementPage`), and `AgentDetailPage` → `BeaconDetailPage`: 175 files
   moved with `git mv`. The boundary is deliberate — identifiers that mirror the
   server contract (`agentId`, `lib/api-paths.ts`, the detail sub-components such as
   `AgentHeader`/`AgentStatusBar`) keep the agent vocabulary, because the API is the
   contract and the URL is the label.
4. **Baselines: renamed, not regenerated.** `agents-*` → `beacons-*` and
   `timeline-*` → `events-*` on both platforms (16 files, `git mv`, byte-identical —
   the same page at a new address), with the parity check still reporting 28 names
   per platform. The static e2e suite gained seven redirect cases (canonical URL +
   surviving query string) and the final run was 184 passed / 36 skipped. One
   `ai-dark` visual failure in the first run was re-run-verified as a pre-existing
   flake in the AI welcome block's vertical offset: no `pages/ai/**` file is touched
   by this wave.

**Deployment probe.** Against the live server (embedded bundle, unauthenticated
session) `/beacons`, `/beacons/abc/shell`, `/events` and `/payload-generator` all
resolve in the shipped SPA and hand off to the login guard with their own path in
`?next=`, while `/agents` is rewritten to `/beacons` *before* the guard fires — the
compatibility redirect is in the deployed bundle, not only in the test server. The
served entry chunk hashes identically to `internal/webdist/dist`
(`92C14B56…D36151CD`), so what shipped is what was tested.

**Method note (codemod).** The 151 link literals were rewritten by a one-shot Node
script whose dry-run report was reviewed line by line before applying; five matches
were excluded as false positives because they are backend endpoints or
module-relative imports (`lib/agents.test.ts`'s `from "./agents"`,
`lib/cred-quality.ts`'s `GET /agents/:id/` doc line, `InjectSection`'s dual-use
endpoint note, `useCollectTask.test.ts` and `useRemoteDesktop.test.ts`'s API-path
assertions). `router.tsx` and `vite-plugins/staticExport.ts` were edited by hand so
the legacy tables could not be rewritten into themselves.

## U7 — Professional naming & page emoji (wave 3, landed 2026-10-19)

Wave 3 is the body-copy layer of the program (wave 1 did nav, titles and tiles,
wave 2 the URLs; `docs/TERMINOLOGY.md` remains the contract). It runs **page domain
by page domain**, and this wave takes the Beacons domain — the page whose rename
started the program, and the one that still said "Agent" in its own toolbar.

1. **Inventory before editing.** A throwaway Node script parses both language
   blocks and flags every *value* (never a key) that still matches the pre-Beacon
   vocabulary: EN `\b(agents?|sessions?|implants?)\b`, ZH 代理/会话/植入/受控/木马
   plus the English loan words the Chinese block carries in Latin script
   (`implant 版本`, `已导出 {count} 个 agent`). The Beacons domain flags 100
   values — 97 `agents.*` and the 3 `status_banner.*` strings the global banner
   renders — and the codemod refuses to run until every one of them is classified
   either as a change or as a deliberate keep, so a value cannot be silently
   skipped.
2. **The rules** (now also in `TERMINOLOGY.md`): the *running* implant is a
   beacon ("Waiting for the implant to return this result" → "…for the beacon…");
   the *generated artifact* is a payload ("Generate Implant" → **Generate
   Payload**, the toolbar button the baseline diff shows, "Deploy your first
   implant" → "…your first payload"); the check-in *event* is "check-in" in EN and
   回连 in ZH ("Agent will exit on next beacon" → "The beacon will exit on its next
   check-in", cf. the pre-existing "下次心跳" values left alone); the per-host
   count reads beacons, not sessions (`{n} sessions` → `{n} beacons`); and the
   detail header's fallback title (`agents.detail_session`, used when a host has
   no hostname) collapses to `Beacon` beside the `Beacon ID` / `Copy beacon ID`
   pair.
3. **Kept on purpose (13 values).** The `User-Agent` HTTP header family
   (`agents.config.ua`, `config.reset_ua`, `config_user_agent`), `AMSI Session
   Bypass` (a Windows technique name), the recon category `Sessions`
   (`agents.recon_session` — host logon/network sessions, not beacons), the
   remote-desktop trio plus the media panel's console-session strings ("results
   stay in this session gallery"), the Windows logon-session token source
   (`Logon Create` / 创建登录会话), and the WeChat pair where 会话 means *chat*.
   "Session" is banned as a synonym for a beacon, not as a word.
4. **Verification.** eslint and `tsc --noEmit` clean; unit suite 172 files /
   883 tests; `npm run check` 11/11 (i18n parity 4727 keys — values only, so the
   parity set is unchanged; bundle 3.99 MB / 190 files, critical path 972 kB);
   e2e 184 passed / 36 skipped. The first e2e run failed the four beacons-list
   cases for a mechanical reason worth recording: the suite serves the *prebuilt*
   `out/` export (`node scripts/serve-static.mjs out 4173`), so a copy change needs
   `npm run build` before the suite can see it — the failure was the old placeholder
   text still shipping in the served bundle, not a bad locator.

**Baseline adjudication.** The win32 refresh is scoped by evidence, not by
convenience. `--update-snapshots=all` rewrote twelve files on the first pass and a
partly different fourteen on the second; only the four `beacons-*` ones
(`dark|light` × `laptop|mobile`) are adopted, on the strength of a cropped diff:
the committed baseline still reads `Generate Implant` where the new build says
`Generate Payload` (the toolbar band of the laptop shot; the footer band is the
second copy cluster — the row count and pagination copy). The other ten files were
restored, and the reasons are measurable rather than aesthetic: the status banner
the wave touches is mounted in `AppLayout` but returns `null` under the visual
suite's mocked stats (`if (!counts) return null`), no rewritten key renders on
dashboard/events/listeners/login at all, `login-light-laptop` and `ai-dark-laptop`
regenerated **byte-identical** on the triage pass (so the first pass's rewrite was
noise), and `events-dark-laptop`'s 29-pixel diff sat in the top-bar strip
(`(913,24)–(1191,41)`) where this wave changes nothing.

**Deployment probe.** The rebuilt server (old PID 22104 → new **36260**) answers
`/health` with `ok`, and the served entry chunk `assets/index-Ccwie1A9.js` hashes
identically to `internal/webdist/dist` (`6B060D03…A204DE`), so what shipped is what
was tested. The strings are verifiable in the served bytes: the EN chunk carries
`Search beacons`, `Generate Payload` and `next check-in` and no longer contains
`Search agents`, while the ZH chunk carries 搜索 Beacon / 终止 Beacon / 生成载荷 /
下次回连 / 复制 Beacon ID and no longer 复制会话 ID. An authenticated probe (real
lab session, `/beacons`) confirms the rendering side: the toolbar search input's
placeholder is `Search beacons… ( / )`, the primary button reads **Generate
Payload**, neither `Search agents` nor `Generate Implant` appears anywhere in the
page, and `document.title` is `🐝 Beacons — ForgeC2`.

**Linux set (landed).** The CI frontend job compares against the **linux** files,
which come from the dispatch-only `visual-baselines.yml` (the workflow renders and
uploads; committing the PNGs is manual). Run `39220123316` was dispatched from the
commit that carries the wave (`955c534f`) and its `visual-baselines-linux` artifact
was scoped by the same evidence rule as the win32 pass: of the 28 rendered files, 21
are byte-identical to the committed set and 7 differ — the four `beacons-*` shots are
taken (the page whose copy moved; the dark-laptop diff is 14,347 px, 1.401 % of the
canvas, spanning the toolbar band and the table's copy clusters), and three are
rejected as jitter: `ai-light-laptop` at 11 px (0.001 %, a `(463,69)–(536,123)` strip
in the streaming area) and the two `listeners-*-mobile` shots at 0.445 % / 0.109 % —
all far under the 1.5 % tolerance the committed baselines are compared with, and no
rewritten key renders on those pages. The baseline follow-up is closed on both
platforms.

## U7 — Professional naming & page emoji (wave 3, body copy 2: the payload domain, landed 2026-10-19)

The second domain of the body-copy sweep, same method as the first: an inventory over
both language blocks flags the domain's pre-Beacon values (`generate.*` 14,
`profiles.*` 10, `builds.*` 5, plus the neighbouring `oneshot.*` 4), and the codemod
refuses to run until every flagged value is classified, with the map recording a
reason per key.

1. **The domain is the Build & Deploy pipeline** — the payload generator
   (`generate.*`), the build-record page (`builds.*`) and the profiles page
   (`profiles.*`): three surfaces talking about one generated artifact.
2. **Rewrites (24 values, 46 value edits).** The format grid stops calling itself
   "Agent Binaries": it offers six *formats* (exe, dll, ps1, ELF, Mach-O, one-liner),
   so the tablist is "Payload Formats" / 载荷格式 (`generate.agents_title`, whose
   accessible name the e2e smoke case asserts — the locator follows). The generated
   artifact is a payload wherever it is named ("Implant build records" → **Payload
   build records**, "Generate a new implant to get started" → "Generate a new
   payload to get started", "Generated agents connect back to C2 automatically" →
   "…payloads…", "Agents already generated with it are unaffected" → "Payloads…").
   The running thing is a beacon ("The implant would dial a port…" → "The beacon
   would dial…", "…so the agent can authenticate against a listener…" → "…the
   beacon…", "Agent modules" → **Beacon modules** in the title, the hint *and* the
   picker's aria-label). The counts follow the entity: "Agent Version Distribution"
   → **Beacon Version Distribution** with the unit "{n} agents" → "{n} beacons", and
   the profile push target is a beacon ("Push to Agent" → **Push to Beacon**, "Load
   agents" → **Load beacons** — the Chinese button had said 加载主机, i.e. hosts,
   and now says 加载 Beacon, with its hint and both toasts in step).
3. **Two rules the domain settles** (recorded in `TERMINOLOGY.md` as rule 7). A
   *format descriptor* names the format, not the entity: "GUI implant" →
   **GUI executable** / GUI 可执行文件, in a tile family that already reads
   `sideload` / `script one-liner` / `ELF · amd64/arm`. And where the sentence is
   about the *code* or the *build*, a subjectless clause beats smuggling the banned
   noun back in: "After implant changes, regenerate the payload" → "Regenerate the
   payload after code changes", "Not available on macOS implants" → "…in macOS
   builds" (the ZH side of the ICMP note had carried a bare `implant` loan word).
4. **Kept on purpose (5).** `HTTP Proxy` / HTTP 代理 (a network proxy in the build
   form), the two `User-Agent` labels (`generate.user_agent`,
   `profiles.user_agent_label` — the HTTP header family) and the settings-reload
   warning's "new sessions" (login sessions, rule 6).
5. **Deferred (4).** `oneshot.*` reads like the one-shot payload, but the values
   render in the automation page's **One-Shot Scheduled Tasks** card, so they sweep
   with the automation domain rather than here.
6. **Verification.** eslint and `tsc --noEmit` clean; unit suite 172 files / 883
   tests; `npm run check` 11/11 (i18n parity 4727 keys, bundle 3.99 MB / 190 files,
   critical path 972 kB); the `out/` export rebuilt and the 337-file
   `internal/webdist/dist` re-synced; the full e2e suite 184 passed / 36 skipped.
7. **No baseline move on either platform.** None of the three pages is in
   `VISUAL_ROUTES` (`login`, `dashboard`, `beacons`, `listeners`, `events`,
   `settings`, `ai`), so the wave changes no screenshot the suites compare — unlike
   wave 1, there is nothing to adjudicate here, and `gh` needed no dispatch.

**Deployment probe.** The rebuilt binary (PID 36260 → **32364**, started 16:31:20)
answers `/health` with `ok` and serves the new bundle byte-for-byte: the entry chunk
`assets/index-CqJpFQtm.js` hashes `3611C26B…6AD5` and the ZH chunk
`assets/zh-g-9iSHmH.js` `73C788FB…3290`, both identical to `internal/webdist/dist` —
and `check:webdist` pins that tree to `frontend/out`, the export the e2e suite served
— so what shipped is what was tested. The served bytes were then read back and
asserted marker by marker (33 assertions, all as expected): the EN entry carries
`Payload Formats`, `Beacon modules` (title, hint and aria-label), `Payload build
records`, `Generated payloads connect back to C2 automatically`, `Push to Beacon`,
`Load beacons`, `GUI executable`, `Beacon Version Distribution`, `Regenerate the
payload after code changes` and `Not available in macOS builds`, while `Agent
Binaries`, `Agent Profiles`, `Implant build records`, `Agent modules`, `Push to
Agent`, `Load agents`, `GUI implant`, `Agent Version Distribution` and `After implant
changes` are gone; the ZH chunk carries 载荷格式 / 载荷构建记录 / 推送至 Beacon /
加载 Beacon / Beacon 版本分布 / 构建时嵌入 Beacon and no longer 代理模块 / 加载主机 /
推送至 Agent / Agent 配置文件 / GUI 植入体 / Agent 版本分布. No authenticated DOM
probe ran at the time, unlike wave 1: the lab credential used earlier was not
recorded in the repo or the scratch tree (the README's example `labtest`/`labtest`
returns 401). **The probe is now closed** (2026-10-20, credential supplied by the
operator; live server PID 17704, `/health` ok). A real `/login` session on
`/payload-generator` rendered the wave's copy with no page errors: the format grid's
tablist is `Payload Formats` / `载荷格式`, the exe tile is `GUI executable` /
`GUI 可执行文件`, the advanced section is `Beacon modules` / `Beacon 模块`; the
Beacon Profiles inner tab shows `Push to Beacon` / `推送至 Beacon`, its push dialog
shows `Load beacons` / `加载 Beacon` (the click round-trips without an error), and
the build records subtitle is `Payload build records` / `载荷构建记录`. None of
`Agent Binaries`, `Agent modules`, `GUI implant`, `Push to Agent`, `Load agents`,
`After implant changes`, `Agent Version Distribution` or their ZH twins appeared in
any of the three tabs. `Beacon Version Distribution` / `Beacon 版本分布` is data-gated
on a non-empty beacon list (the card derives `versionDist` from beacons, and this lab
server currently has zero), so its rendering is covered by the data-present unit/e2e
fixtures rather than this zero-beacon probe; the string remains present in the served
bundle.

## U7 — Professional naming & page emoji (wave 3, body copy 3: the automation domain, landed 2026-10-19)

Third domain of the sweep. The automation page cluster — the triggers/alert-rule
cards, the scheduled-tasks card, the one-shot card and the workflow workspace with
its editor and execution-history dialogs — is swept as **one page**, so the shared
pickers and the `workflows.*` strings the page hosts move together.

1. **Inventory & scope.** Same method as before, run over six namespaces because the
   page hosts cards whose keys live elsewhere: 27 values flag (`auto.*` 6,
   `scheduler.*` 4, `oneshot.*` 4, `workflows.*` 5, `macros.*` 5, `autotag.*` 3).
   The macros page (`pages/macros/MacrosPageContent.tsx`) and the auto-tag rules page
   (`pages/autotag/AutoTagPage.tsx`) are separate pages with their own domains, and
   the attribution is checked per key — the automation page itself touches only two
   unflagged keys from those namespaces (`macros.global_stop_on_error`,
   `autotag.create`/`autotag.update`), so their 8 flagged values are deferred with
   that reason.
2. **Rewrites (19 values, 38 value edits, 0 keeps).** This is the first domain with
   nothing to keep: no `User-Agent`, no login-session copy, no network proxy hides in
   it. The event and alert-rule labels follow the entity — "Agent offline" / "Agent
   online" → **Beacon offline** / **Beacon online** in `auto.event_*` and
   `auto.type_*` — and the enum ids behind them (`agent_offline`, `agent_online`,
   the `agent.checkin` / `agent.disconnect` event list) stay untouched, because those
   are wire contract rather than copy. The pickers converge: `scheduler.agent`
   ("Agent" / 代理) → **Beacon**, `scheduler.select_agent` ("Select agent…" /
   选择代理…) → "Select beacon…", `scheduler.a11y_agent` → "Select beacon",
   `scheduler.err_agent_required` → "A beacon is required", and the four
   `oneshot.*` strings deferred by wave 2 land here too ("Select a beacon…", the
   beacon-list warning).
3. **Workflows read beacons.** `workflows.scope_all` "All Agents" → **All Beacons**,
   `scope_selected` → **Selected Beacons**, the scope-id placeholder "…or agent
   ids" → "…or beacon ids" (it doubles as that input's a11y label), the workspace
   subtitle → "…for beacon operations", and the dispatch toast → "Executed:
   {task_count} tasks to {agents_count} **beacons**" / 发送至 {agents_count} 个
   Beacon.
4. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests;
   `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and the 337-file `internal/webdist/dist` re-synced; the
   full e2e suite 184 passed / 36 skipped. No locator needed updating: nothing in the
   e2e or unit suites asserts on these strings (the `agent offline` literals the grep
   turns up are mock rejection messages in the beacons suites, not copy).
5. **No baseline move.** The automation page is not in `VISUAL_ROUTES`; the visual
   sets stay as committed.

**Deployment probe (and the residue it surfaced).** The rebuilt binary (PID 32364 → **9044**, started 16:54:47) answers `/health` with `ok`, and the served chunks are byte-identical to `internal/webdist/dist`: `assets/index-Cs8UlUem.js` = `85741880…FDC4`, `assets/zh-B0G7T9DV.js` = `7100B46B…E261`. The served bytes were then asserted marker by marker (33 assertions; 32 as expected, one deliberately re-classified below): "Beacon offline" / "Beacon online", the beacon-flavoured macro hint, the scheduler and one-shot picker copy, "All Beacons" / "Selected Beacons" / "…or beacon ids", the workflow subtitle and the dispatch toast ("…to {agents_count} beacons") are all present, while the agent-side twins ("An agent is required", "Select agent…", "…or agent ids", "tasks to {agents_count} agents", the agent-list warning) are gone.

The one unexpected marker is worth more than the wave: **"Agent 离线" still ships**, and the source says why — `hostinfo.offline_hint` ("Agent is offline — collection unavailable." / "Agent 离线——无法采集。") renders on the **beacon detail** page, and wave 1 had scoped its inventory by *key prefix* (`agents.*` + `status_banner.*`), so sibling namespaces the beacon pages render were never flagged. Re-scanning every namespace the beacon pages consume (`t("<ns>.` over `pages/beacons/**`) widens the finding: `shell.*` 11, `cred.*` 14, `screen.*` 2, `hostinfo.*` 1 — **28 flagged values the "Beacons domain done" claim had missed**, living in the shell console, the screen viewer, the credentials/harvest card and the host-info card (`persistence.*`, `format.*`, `notes.*`, `files.*`, `quick_shell.*` are clean). The same scan over the domains already swept found one more shared namespace: `listeners.*` (6 flagged) is rendered by the payload generator as well as by the listeners page. Both are queued as the next waves — the residue pass first, then listeners — instead of being quietly folded into this one.

## U7 — Professional naming & page emoji (wave 3, body copy 4: the residue pass, landed 2026-10-19)

The wave that fixes a scope mistake rather than adding a domain. Wave 1 inventoried
the Beacons cluster by **key prefix** (`agents.*` + `status_banner.*`) and declared it
done; but the beacon *pages* also render `shell.*` (the shell console), `cred.*` (the
credentials page and the detail's credential surfaces), `screen.*` (the screen
viewer) and `hostinfo.*` (the host-info card). Nothing had flagged them, and the
wave-3 deployment marker check found one of them still shipping — "Agent 离线" on the
host-info card. So the cluster was never actually complete.

1. **Method fix, then the scan.** The inventory now starts from the pages
   (`t("<ns>.` over `pages/beacons/**`) and adjudicates *every* namespace they touch;
   the same scan was re-run over the domains already swept to catch other holes (the
   payload generator renders `listeners.*`, carried as its own wave).
2. **28 flagged, 21 rewritten (39 value edits), 7 kept, 0 deferred.** The keeps are
   the cookie-proxy family (`Cookie proxy failed` / Cookie 代理失败, start/stop,
   status unknown) — a real local HTTP proxy that injects harvested cookies, i.e.
   代理 = proxy, not an implant synonym. That is rule 6 working as intended.
3. **The shell console settles.** Each shell tab is keyed by beacon id
   (`closeTab(tab.agentId)`, the select calls `openAgentTab`), so "session" there was
   the banned sense: `shell.tabs_label` "Shell sessions" → **Beacon shells**,
   `new_tab` "New session tab" → "New shell tab", `close_tab` "Close session" →
   "Close tab", `switch_session` "Switch session" → **Switch beacon**,
   `back_to_session` "Back to agent" → **Back to beacon** (it links to the detail
   page), `tab_limit` → "Shell tab limit ({max}) reached". The status strings follow:
   "Agent is {status}" → **Beacon is {status}**, "…still running on the agent" →
   "…on the beacon", `waiting_output` "waiting for implant output…" → "waiting for
   beacon output…", and the ZH side retires two bare `implant` loan words
   (`offline_body` 等 implant 回连 → 等 Beacon 回连).
4. **The credential and screen surfaces.** "Target session" → **Target beacon**,
   "Harvest from session" → **Harvest from beacon**, "Select a session first" →
   "Select a beacon first", "…auto-parsed from agent tasks" → "…from beacon tasks",
   the harvest explainer's "implant tasks" → "beacon tasks", the skipped-entries
   toast "…(no password/agent or fuse)" → "…(no password/beacon or fuse)"; the
   screen viewer's "Waiting for the agent to return a fresh frame…" → "…for the
   beacon…" and its timeout copy; and `hostinfo.offline_hint` → **Beacon is
   offline — collection unavailable.** / **Beacon 离线——无法采集。**
5. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests;
   `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and `internal/webdist/dist` re-synced (337 files); full
   e2e 184 passed / 36 skipped. The `/beacons` visual baseline does not move: the only
   `cred.*` keys that route renders (`quality_scripted`, `missing_module`,
   `module_status_unknown`) are unflagged, and no other rewritten key renders on a
   `VISUAL_ROUTES` page.

**Deployment probe.** The rebuilt binary (PID 9044 → **27356**, started 17:11:14)
answers `/health` with `ok`, and the served chunks are byte-identical to
`internal/webdist/dist` (`assets/index-DuXXv_rO.js` = `CD2F159F…E8B8`,
`assets/zh-toP-6Bt8.js` = `81293D6D…00B9`). The served bytes were then asserted
marker by marker, 60 of 60 as expected: the whole residue is gone from the wire
("Agent is offline" / "Agent 离线——无法采集。", "Target session", "Select a session
first", "Harvest from session", "Back to agent", "Switch session", "Shell sessions",
"New session tab", "Close session", "Session tab limit", "waiting for implant
output…", "queue implant tasks", "password/agent or fuse", and the screen viewer's
pair), and the new copy is live ("Beacon is offline — collection unavailable." /
**Beacon 离线——无法采集。**, "Target beacon" / 目标 Beacon, "Harvest from beacon" /
从 Beacon 采集, "Select a beacon first" / 请先选择 Beacon, "Back to beacon" /
返回 Beacon, "Switch beacon" / 切换 Beacon, "Beacon shells" / **Beacon Shell**,
"New shell tab", "Close tab" / 关闭标签, "Shell tab limit ({max}) reached",
"waiting for beacon output…" / 等待 Beacon 返回输出, "…still running on the beacon",
and the shell offline body's 等 Beacon 回连后再执行). One marker needed pinning
down first: `切换会话` still exists in the AI domain (`ai.toggle_sessions` =
切换会话列表), so that assertion matches the exact value *with its closing quote*
rather than the bare substring — and the bundle was checked against the source for
every absence marker before the run (all zero, the AI one excluded by construction).
This closes the defect the previous probe surfaced: the same check failed on
"Agent 离线" against the previous bundle and now passes.

## U7 — Professional naming & page emoji (wave 3, body copy 5: the listeners domain, landed 2026-10-19)

The first wave to start from the pages instead of a prefix. `pages/listeners/**` is
scanned for every namespace it renders (`t("<ns>.`), which turns up `cb.*`,
`dashboard.*`, `generate.*`, `listener.*`, `listeners.*` and `status.*` — and the
*singular* `listener.*` is a namespace no prefix list had included, exactly the hole
that bit wave 1.

1. **12 flagged → 12 rewritten (24 value edits), 0 kept, 2 deferred.** The deferrals
   are attribution, not scope creep: `cb.honesty_desc` renders on the
   circuit-breaker page (`pages/circuit-breaker/CircuitBreakerPage.tsx`) and
   `dashboard.quick_generate` on the dashboard (`pages/dashboard/components/
   OpsHome.tsx`) — separate pages with their own waves, and the dashboard is a
   visual route, so its wave carries the baseline work. `status.*` has nothing
   flagged; `generate.*` was already swept in wave 2.
2. **The detail page.** Stats and headings follow the rows they count:
   `listener.agent_total` "Associated Agent Total" → **Associated Beacon Total**,
   `agents_using` → **Beacons Using This Listener**, `view_all_agents` → **View All
   Beacons** (the link goes to `/beacons`). The empty state stops calling the client
   an implant: `listener.empty` "No Implant uses this listener" → **"No beacons use
   this listener"**. The action follows rule 7: `listener.generate_implant`
   "Generate Implant for this Listener" → **"Generate Payload for this Listener"**
   (it links to `/payload-generator?listener_id=…`). The load-balancing hint
   (`lb_hint`) moves "Agents can randomly or by policy connect" → "Beacons can…",
   and the ZH side follows throughout (关联 Beacon 总数, 使用此监听器的 Beacon,
   暂无 Beacon 使用此监听器, 为此监听器生成载荷, 查看全部 Beacon).
3. **The list page.** `listeners.col_agent` "Agent" → **Beacon** and `col_agents`
   "Agents" → **Beacons** (the ZH stays 无复数), the two count warnings
   (`agent_count_failed`, `agent_counts_unavailable`) become beacon counts, and the
   hero subtitle and tip follow rule 7 twice over: "Configure C2 listeners to
   receive implant connections. Select a target listener when generating implants."
   → "…to receive **beacon connections**. Select a target listener when generating a
   **payload**." and "…when generating an implant to connect **agents**…" → "…when
   generating a **payload** to connect **beacons**…".
4. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests;
   `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and `internal/webdist/dist` re-synced (337 files); full
   e2e 184 passed / 36 skipped with **no visual failure** — the listeners copy stays
   under the 1.5 % pixel tolerance on the win32 set, so the committed win32 baselines
   are deliberately left alone (refreshing sub-tolerance frames is what the wave-1
   linux lesson warned against). The linux set was re-rendered from this commit by
   `visual-baselines.yml` and compared file by file; see the deployment probe below
   for the numbers and the adoption decision.

**Deployment probe.** The rebuilt binary (PID 27356 → **28200**, started 17:39:59; the first two launcher attempts hung without starting anything and the third, a `.NET Process`-based launcher, came up clean) answers `/health` with `ok`, and the served chunks are byte-identical to `internal/webdist/dist` (`assets/index-CPZuDcy1.js` = `FCA65CEE…D835`, `assets/zh-DLE38mbq.js` = `ACD0258B…6753`). The served bytes were then asserted marker by marker, 35 of 35 as expected: the listeners vocabulary is live ("Associated Beacon Total", "Beacons Using This Listener", "View All Beacons", "No beacons use this listener", "Generate Payload for this Listener", "Beacons can randomly or by policy connect to different listeners", the subtitle and tip with "beacon connections" / "generating a payload", and the two beacon-count warnings; ZH: 关联 Beacon 总数 / 使用此监听器的 Beacon / 查看全部 Beacon / 暂无 Beacon 使用此监听器 / 为此监听器生成载荷 / 加载监听器 Beacon 数量失败), and the twins from the old vocabulary are gone from the wire ("Associated Agent Total", "Agents Using This Listener", "View All Agents", "No Implant uses this listener", "Generate Implant for this Listener", "receive implant connections", "when generating implants", "generating an implant to connect agents", "Failed to load listener agent counts", "Per-listener agent counts" and their ZH counterparts). Every absence marker was checked against the source first (all zero) so the assertion cannot trip on another domain's copy.

**Linux baseline check — evidence says leave the set alone.** `visual-baselines.yml` run **39230315724** re-rendered the 28-file linux set from this commit. 20 files are byte-identical to the committed ones; 8 differ, and pixel triage splits them cleanly. The four `listeners-*` frames — the ones this wave changed — differ by 0.866 % (dark mobile), 0.928 % (light mobile), 1.087 % (light laptop) and 1.094 % (dark laptop): all **below** the 1.5 % tolerance, with the diffs spread over the content column (`bbox=(467,153)-(1105,752)` on laptop, `(124,141)-(373,760)` on mobile — the hero and table copy plus the small reflow it causes). The other four (`ai-light-laptop` 0.001 %, 11 px at the same `(463,69)-(536,123)` box wave 1 saw; `beacons-dark-mobile` 0.084 %; `beacons-light-mobile` 0.094 %; `events-dark-mobile` 0.011 %) are jitter on pages this wave never touched. So **nothing is adopted**: the committed win32 and linux sets stay exactly as they are, and the decision is recorded with its numbers instead of assumed — the local suite enforces the win32 set and passed unmodified, which is the same answer reached from the other direction.

## U7 — Professional naming & page emoji (wave 3, body copy 6: the macros & auto-tag domains, landed 2026-10-19)

Two small pages in one wave, both deferred here by earlier waves with their reasons
attached — the macro library (`pages/macros/**`, whose flagged keys were handed over
from the automation wave) and the auto-tag rules page (`pages/autotag/**`).

1. **Inventory & the one shared key.** The page-cluster scan returns `macros.*`,
   `autotag.*`, `common.*` and `ai.*`; nothing is flagged in `common.*`, and in `ai.*`
   exactly one key is: `ai.pb_agent`, the playbook builder's target selector
   ("Personalize for agent (optional)" / 针对 Agent 定制（可选）) — and the macros
   page renders it (`ai.pb_*` is the shared playbook component used by the AI page
   too). Attribution rule 6 from the residue pass applies: a key a page renders moves
   with that page, so it is rewritten here and the remaining 27 flagged `ai.*` values
   stay with the AI domain. The two automation-page keys that touch these namespaces
   (`macros.global_stop_on_error`, `autotag.create`/`autotag.update`) are unflagged and
   untouched, which is why the automation wave could hand the namespaces over
   wholesale.
2. **Nine flagged → nine rewritten (17 value edits), 0 kept, 0 deferred.** The macro
   flow: `macros.no_agents` "No agents available" → **"No beacons available"**,
   `select_agents_hint` "Select the agents to run this macro on. Each agent executes
   the steps independently and sequentially." → "Select the **beacons**… Each
   **beacon**…", `toast.dispatched` "Macro dispatched to {count} agents" → "…**beacons**"
   / 已下发至 {count} 个 Beacon, `empty_message` "…replay it on any agent with one
   click." → "…any beacon…" (EN-only: the ZH value already said Beacon), and
   `agents_unreadable` "The agent list could not be read - this is not a report that no
   agents exist." → "The **beacon** list…". The auto-tag page: `applied` "Applied
   auto-tags to {count} agent(s)" → "…**beacon**(s)", `subtitle` "Automatically assign
   tags to agents…" → "…to **beacons**…" (ZH said 代理), and `empty_message`
   "…automatically tag agents on check-in." → "…**beacons** on check-in." with the ZH
   side retiring 签入 for **回连** — rule 5's check-in wording finally reaching the
   auto-tag copy.
3. **Marker discipline.** `No agents available` is deliberately *not* asserted absent
   from the bundle: `settings.modules.no_agents` still carries it and belongs to the
   settings domain. The check pins only what this wave owns — the macro/auto-tag
   strings, plus `Personalize for agent (optional)` — and every absence marker was
   verified zero in the source before the bundle assertion ran.
4. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests;
   `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and `internal/webdist/dist` re-synced (337 files); full e2e
   184 passed / 36 skipped with **no visual failure** — neither page is in
   `VISUAL_ROUTES` and `ai.pb_agent` renders inside a dialog, so no baseline is
   affected and no linux re-render is needed for this wave.

**Deployment probe.** The rebuilt binary (PID 28200 → **20300**, started 18:24:16,
launched with the `.NET Process` variant) answers `/health` with `ok`, and the served
chunks are byte-identical to `internal/webdist/dist`
(`assets/index-BA-tvbjw.js` = `9059414A…792F`, `assets/zh-8ORT-df3.js` =
`B300BA5C…DE8A`). The served bytes were then asserted marker by marker, 32 of 32 as
expected: the macro flow is live on the wire ("No beacons available", "Select the
beacons to run this macro on. Each beacon executes the steps independently and
sequentially.", "Macro dispatched to {count} beacons", "…replay it on any beacon with
one click.", "The beacon list could not be read - this is not a report that no
beacons exist."), the auto-tag page follows ("Applied auto-tags to {count}
beacon(s)", "…tag beacons on check-in.", "Automatically assign tags to beacons…",
with ZH 已为 {count} 个 Beacon 应用自动标签 / 在 Beacon 回连时自动打标签 / 自动为
Beacon 分配标签), and the shared playbook key reads "Personalize for beacon
(optional)" / 针对 Beacon 定制. The agent-side twins are gone in both languages
("Applied auto-tags to {count} agent(s)", "…tag agents on check-in", "assign tags to
agents", "replay it on any agent with one click", "Select the agents to run this
macro on", "Macro dispatched to {count} agents", "Personalize for agent (optional)"
and their ZH counterparts). "No agents available" is excluded from the absence
assertions by design — `settings.modules.no_agents` still carries it and belongs to
the settings domain — and every other absence marker was verified zero in the source
before the run, so the check cannot trip on another page's copy.

## U7 — Professional naming & page emoji (wave 3, body copy 7: the users domain, landed 2026-10-19)

The shortest wave so far, and the one that shows rule 6 doing the deciding rather
than the sweep: page-cluster scan over `pages/users/**` returns only `users.*` and
`common.*` (the phishing page borrows exactly one unflagged key, `label_username`),
and of the 14 flagged `users.*` values, **12 are kept**.

1. **The keeps (real login sessions).** `users.sessions` "Sessions" / 会话,
   `sessions_title` "Active Sessions" / 活动会话, `sessions_subtitle` "Sessions for
   {name}" / {name} 的会话, `sessions_empty` "No active sessions" / 暂无活动会话,
   `revoke_session` "Revoke Session" / 撤销会话, `confirm.kick` "Kick this user from
   all sessions?" / 踢出此用户的所有会话？, `confirm.revoke_all` "Revoke ALL active
   sessions for {name}?" / 撤销 {name} 的所有活动会话？, and the five toasts
   (`session_revoked`, `session_revoke_failed`, `sessions_revoked`,
   `sessions_revoke_failed`, `sessions_load_failed`). These name the operator's own
   console sessions — revocable, per-user, nothing to do with beacons — which is
   exactly the sense rule 6 protects. Rewriting any of them would have been a
   regression dressed up as consistency.
2. **The changes (beacon permission copy).** Only the role legend's two bullets talk
   about the target fleet: `users.admin_desc_4` "Lock/unlock agents" → **"Lock/unlock
   beacons"** / 锁定/解锁 Beacon, and `users.user_desc_4` "Lock agents for exclusive
   ops" → **"Lock beacons for exclusive ops"** / 锁定 Beacon 进行专属作业. Both render
   in `UsersPage.tsx`'s legend only.
3. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests;
   `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and `internal/webdist/dist` re-synced (337 files); full
   e2e 184 passed / 36 skipped, no visual route involved and no baseline touched.

**Deployment probe.** The rebuilt binary (PID 20300 → **24088**, started 19:38:08)
answers `/health` with `ok`, and the served chunks are byte-identical to
`internal/webdist/dist` (`assets/index-BZFcikvX.js` = `91CD1812…6B6C`,
`assets/zh-B1u61LTC.js` = `1DBECC24…B772`). The marker check asserts both halves of
this wave. The two rewrites are live ("Lock/unlock beacons", "Lock beacons for
exclusive ops" / 锁定/解锁 Beacon, 锁定 Beacon 进行专属作业) and the old bullets are
gone ("Lock/unlock agents", "Lock agents for exclusive ops"). And every one of the
twelve rule-6 keeps is still on the wire unchanged — "Active Sessions", "Sessions for
{name}", "No active sessions", "Revoke Session", "Kick this user from all sessions?",
"Revoke ALL active sessions for {name}?", "Session revoked", "All sessions revoked",
"Failed to load sessions", with the ZH twins (活动会话, 暂无活动会话, 撤销会话,
踢出此用户的所有会话？, 撤销 {name} 的所有活动会话？, 会话已撤销, 所有会话已撤销,
加载会话失败). 25 of 25 markers as expected — the keeps are asserted on purpose, so a
later sweep cannot quietly rewrite the login-session copy without failing this check.

## U7 — Professional naming & page emoji (wave 3, body copy 8: the circuit-breaker domain, landed 2026-10-19)

The smallest domain yet — and the one that closes the key wave 5 handed over.

1. **Page cluster.** `pages/circuit-breaker/**` renders `cb.*`, `circuit_breaker.*` and
   `common.*`. `circuit_breaker.*` has no flagged values at all — the parameter labels
   (`threshold_label`, `cooldown_label`, `half_open_label`, `force_state`, …) already
   read clean. `common.*` flags exactly one key, `common.session_expired` ("Session
   expired - please sign in again" / 会话已过期，请重新登录): this page does not render
   it (its consumer is `lib/errors.ts`'s global 401 mapping) and it names a real login
   session, so it is kept under rule 6.
2. **The rewrite.** `cb.honesty_desc` — the explainer wave 5 deferred by attribution —
   read "Failover quality depends on implant multi-C2 config and listener topology."
   The multi-C2 endpoint list is baked into the generated artifact at build time, so
   the owner is the payload: **"…depends on the payload's multi-C2 config and listener
   topology."** / **"…取决于载荷的多 C2 配置与监听器拓扑。"** (rule 5: generated
   artifact = payload / 载荷). The rest of the paragraph ("Tracks probe failures and
   state (closed/half-open/open)… Validate in lab before relying on it mid-engagement.")
   needed no change, and `cb.honesty_title` ("Listener health probe — not full multi-C2
   orchestration") was never flagged.
3. **Where "implant" stands now.** After this edit the noun survives only in domains
   that are still open: the AI copy (`ai.greeting_desc`, the quick-action trio), the
   dashboard's `quick_generate` ("Generate Implant"), and the lateral/pivoting strings
   (`lateral.online_implant`, `source_implant`, the two "Select …implant" labels, and
   `pivot_unsupported`).
4. **Verification, with one flake on record.** eslint and `tsc --noEmit` clean;
   172 files / 883 unit tests; `npm run check` 11/11 (parity 4727, bundle 3.99 MB /
   190 files, critical path 972 kB); `out/` rebuilt and `internal/webdist/dist`
   re-synced (337 files). The first full e2e run reported one failure — `ai light` on
   the mobile project, 9405 px (3 % of the image) — which passed in isolation and again
   on the full rerun (184 passed / 36 skipped). The diff showed the AI page's centered
   greeting block shifted about 20 px, i.e. a font-swap re-wrap, not this page's copy;
   under the evidence-scoped baseline protocol no snapshot file was adopted and the
   win32 set is untouched.

**Deployment probe.** The rebuilt binary (PID 24088 → **31872**, started 20:08:12)
answers `/health` with `ok`, and the served chunks are byte-identical to
`internal/webdist/dist` (`assets/index-CITkks3B.js` = `E5C96269…4070`,
`assets/zh-nDeZPJPW.js` = `03640AED…3FCF`). Ten of ten markers as expected: the new
copy is on the wire ("…depends on the payload's multi-C2 config and listener
topology." / "…取决于载荷的多 C2 配置与监听器拓扑。", with the paragraph's opening and
closing sentences intact), the old noun is gone ("implant multi-C2 config" / "implant
多 C2 配置"), and the kept login-session string is still served ("Session expired —
please sign in again" / "会话已过期，请重新登录"). That string appears twice in each
bundle because `common.session_expired` and `login.session_expired` share the value,
so the marker asserts presence rather than an exact count.

## U7 — Professional naming & page emoji (wave 3, body copy 9: the dashboard domain, landed 2026-10-19)

The first visual route since the listeners wave, and the first wave to *measure* its
baseline impact instead of inferring it.

1. **Page cluster.** `pages/dashboard/**` renders `dashboard.*`, `common.*` and
   `tasks.*`. The `tasks.*` scan returns five flagged values (`agent_filter`,
   `all_agents`, `col_agent`, `detail_agent`, `empty_message`), none of which the
   dashboard renders — it only uses the five status labels
   (`tasks.cancelled` / `completed` / `failed` / `pending` / `running`), all unflagged —
   so they are deferred by attribution to the tasks page (events/tasks domain).
   `common.*` has nothing flagged on this page.
2. **The rewrite.** `dashboard.quick_generate` in `OpsHome.tsx`'s Quick launch card:
   "Generate Implant" → **"Generate Payload"** / 木马 → **载荷**. The ZH value was the
   last 木马 in the console copy (grep-verified: one occurrence repo-wide, now zero),
   and the new pair matches `agents.generate_implant` exactly.
3. **The baseline, measured.** The dashboard is in `VISUAL_ROUTES`, so this wave could
   not simply assert "no baseline move". A temporary zero-tolerance probe (the same
   `prepare`/settle logic as `layout.spec.ts`, the four dashboard baselines copied into
   a throwaway snapshot directory, both deleted afterwards) compared the rebuilt page
   against the committed win32 files on mobile (390×844) and laptop (1280×800), light
   and dark: 78 px on mobile light and at most 536 px on laptop light — worst ≈0.05 %
   of the image against the 0.015 ratio contract. No snapshot file was adopted. Linux
   was not re-rendered: the change is one same-length label inside a fixed grid cell,
   and the listeners wave's linux data (worst 1.09 % for several long sentences across
   four pages) bounds it far below tolerance.
4. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests;
   `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and `internal/webdist/dist` re-synced (337 files); full e2e
   184 passed / 36 skipped.

**Deployment probe.** The rebuilt binary (PID 31872 → **36392**, started 20:27:51)
answers `/health` with `ok`, and the served chunks are byte-identical to
`internal/webdist/dist` (`assets/index-CJsXA1YU.js` = `9267C629…13F1`,
`assets/zh-BoChKmsw.js` = `C690FD01…D30A`). Five of five markers as expected:
"Generate Payload" is served (three times — the quick launch plus the beacons and
listener entry points), "Generate Implant" is now zero in the bundle, and on the ZH
side 生成载荷 is served (seven occurrences across the console) while 生成木马 and the
bare 木马 are zero — the trojan loan word is gone from the shipped copy entirely.

## U7 — Professional naming & page emoji (wave 3, body copy 10: the events & tasks domains, landed 2026-10-19)

Two page clusters, one of them the last data-dense surface whose "session" wording was
not a login session — plus the first *app* fix the visual gate forced.

1. **Page clusters & the session ruling.** `pages/events/**` renders only `events.*`;
   its two flagged values are the follow toggle's labels. The code passes
   `followSession ? dockAgentId : null` into the event filter — the "session" is the
   interact dock's beacon, not the operator's login session, so rule 6 does not apply:
   "This session" / "All sessions" → **"This Beacon" / "All Beacons"** (当前 / 全部
   Beacon), matching `workflows.scope_all`. `pages/tasks/**` renders `tasks.*`,
   `common.*` and `ai.*`; the five `tasks.*` values deferred by the dashboard wave are
   rewritten here ("Agent filter" → **"Beacon filter"**, "All Agents" → **"All
   Beacons"**, the column and detail field "Agent" → **"Beacon"**, "Select an agent and
   dispatch a task…" → "Select a **beacon**…").
2. **The NL-query attribution.** The tasks page's `NLQueryDialog` is the only consumer
   of `ai.nl_placeholder` and `ai.nl_hint` (grep-verified against `src/`), so the two
   flagged values move with this page while the rest of `ai.*` stays for the AI domain
   — 25 flagged values remain there. Rewrite: "…which beacons uploaded files…" /
   "…find beacons that ran mimikatz…", and the ZH examples (机器) move to Beacon too,
   since the entity being queried is the beacon.
3. **The flake, root-caused and fixed in the app.** The full suite failed twice in
   three waves on `ai light` / `ai dark` mobile (3–4 %, always green on rerun). The
   diff first looked like a font-swap re-wrap; a `document.fonts.ready` wait was tried,
   did not fix it, and was reverted. The images showed the whole centered block exactly
   32 px higher with the fourth quick action fully visible — the message log was
   scrolled. `AIPageContent`'s `activeSessionId` effect unconditionally called
   `scheduleScrollToBottom`, so when the session list resolved during capture an empty
   conversation sat 32 px down. The effect now skips empty conversations
   (`hasMessagesRef`), which is also the right user-facing behaviour — an empty chat has
   nothing to reveal. Two consecutive full runs (184 passed / 36 skipped) confirm it,
   and the committed baselines needed no change.
4. **Baseline measurement.** The events route is in `VISUAL_ROUTES`; the zero-tolerance
   probe on mobile/laptop light+dark found three captures pixel-identical and mobile
   light at 80 px (0.02 %), far inside the 1.5 % contract → no snapshot adopted, linux
   not re-rendered.
5. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests;
   `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and `internal/webdist/dist` re-synced (337 files); full e2e
   184 passed / 36 skipped twice.

**Deployment probe.** The rebuilt binary (PID 36392 → **6944**, started 21:03:41)
answers `/health` with `ok`, and the served chunks are byte-identical to
`internal/webdist/dist` (`assets/index-DrAwE8I1.js` = `2849E675…68B6`,
`assets/zh-D65CJkSs.js` = `C0ED90E3…8658`). Twenty-two of twenty-two markers as
expected: the events toggle serves "This Beacon" / "All Beacons" (当前 / 全部 Beacon)
with the old quoted value gone, and the tasks page serves "Beacon filter", the rewritten
empty state and the NL examples ("…which beacons uploaded files…", "…find beacons that
ran mimikatz…" / 哪些 Beacon 上传过文件, 跑过 mimikatz 的 Beacon) with the agent/machine
twins gone. Two absence markers are deliberately scoped: the bare 全部会话 is still
served by `agents.wechat_all_conversations` (WeChat history, its own domain), so only
the quoted events value is asserted absent, and "All Agents" is likewise left alone
(attack/loot still own it). The empty-conversation scroll fix ships in the same build
and is covered by the double full-suite run rather than by a string marker.

## U7 — Professional naming & page emoji (wave 3, body copy 11: the settings domain, landed 2026-10-19)

The largest configuration surface: 29 flagged values across the settings cards, with six
genuine login-session keepers.

1. **Beacons and payloads.** The About card's "Implants" labels `total_agents` /
   `online_agents`, so it becomes **"Beacons"** (Agent → Beacon in ZH). The agent
   section's label, its "Agent Behavior" title, the group label and the save toast all
   become **Beacon**; its subtitle turns "Default agent behavior for new implants" into
   "Default **beacon** behavior for new **payloads**" (running = beacon, artifact =
   payload). The database card's "Agents" row counts beacon rows → **"Beacons"**.
2. **The kill switch.** `settings.emergency.*` said implants and treated "beacon" as a
   time noun: "self-destructs ALL implants" → **beacons**, "on its next beacon" → "on its
   next **check-in**", "Implants behave normally" → "**Beacons** behave normally", and
   the ZH "已上报过" → "已回连过". The fleet is beacons and the event is a check-in
   (rule 5).
3. **Module deployment & routes.** The deploy card targets one beacon ("Select beacon",
   "Deploy to beacon", "writes the module to the **beacon** path on its next
   **check-in**"), the modules subtitle reads "for **beacon** tasks", the routes hint
   alerts on "**beacon** offline", and `settings.modules.no_agents` ("No agents
   available" → "No beacons available") closes the last clone of the wave-3 macros
   string.
4. **The access cards.** "Operator accounts, roles, and session labels" → "**beacon**
   labels", "filter the session table" → "filter the **beacon** table"; the ZH side still
   carried 会话集合 / 会话表 / 会话标签 and moves with them (the groups card already said
   beacons in EN).
5. **Rule-6 keeps (6).** `settings.access_users_desc` (operator accounts and their login
   sessions), `settings.confirm.jwt` (invalidates login sessions),
   `settings.security.subtitle`, `settings.server.session_timeout`,
   `settings.telemetry_empty_desc` (the current console session), and
   `common.session_expired` (kept in wave 8 too). `settings.agent.user_agent` is the
   literal HTTP header and keeps its name.
6. **Verification & baseline.** eslint and `tsc --noEmit` clean; 172 files / 883 unit
   tests; `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and dist re-synced (337 files); full e2e 184 passed / 36
   skipped once. The settings route is in `VISUAL_ROUTES`; the zero-tolerance probe on
   mobile/laptop light+dark found mobile dark pixel-identical and the others at 46, 377
   and 327 px (0.01 %) → inside the 1.5 % contract, no snapshot adopted, linux not
   re-rendered.

**Deployment probe.** The rebuilt binary (PID 6944 → **10072**, started 21:27:47)
answers `/health` with `ok`, and the served chunks are byte-identical to
`internal/webdist/dist` (`assets/index-WCV_vzoe.js` = `AA99F1B5…5E07`,
`assets/zh-CSR6yqJ9.js` = `60A31BB6…586D`). Thirty-seven of thirty-seven markers as
expected: the settings page serves the beacon-form agent section/title/toast ("Beacon
Behavior", "Beacon configuration saved", the payload-aware subtitle), the kill switch
("ARMED — every beacon will self-destruct on its next check-in." / 已武装 —— 所有 Beacon
将在下次回连时自毁。), the module deploy card, the routes alert, the access cards'
beacon labels/table (including the ZH 会话集合 → Beacon 集合), and the last "No agents
available" clone is gone (asserted quoted; the longer `bof.toast.no_agents` sentence
legitimately keeps the words). The rule-6 keepers (`User-Agent`, the JWT/login-session
strings, the session timeout) are unchanged in this build.

## U7 — Professional naming & page emoji (wave 3, body copy 12: the credentials, NTLM & cloud domain, landed 2026-10-19)

Three related pages in one wave: the credential tools are where the ZH 代理 (agent)
wording survived longest, while the cookie-proxy and relay-session strings are
deliberate keeps.

1. **NTLM.** The page selects a beacon (8 rewrites): "-- Select Agent --" → "-- Select
   Beacon --", "Target Agent" → "Target Beacon", the three toast validations ("Select a
   beacon first", "…and enter a relay target", "…and enter a target"), "load agents
   failed" → "load beacons failed", and both listener labels say "(**beacon** side)".
   The ZH side carried 代理/代理端 in all eight and now reads Beacon.
2. **Cloud.** Nine rewrites: the dispatch/empty/filter/target pickers say beacon, the
   experimental note dispatches "**beacon-side**" tasks, credentials are "harvested from
   **beacons**", the subtitle steals "from **beacon hosts**" (从 Beacon 主机; the last
   "agent machines" wording here), and the unreadable-rows warning no longer says the
   agent holds none but the **beacon**.
3. **The 14 keeps are evidence-backed.** The seven `cred.cookie_proxy_*` strings describe
   a real local 127.0.0.1 HTTP proxy that injects harvested cookies — ZH 代理 = proxy,
   the wave-4 ruling, and a detector false positive. The six `ntlm.*` session strings
   ("Session ID", "Relay Sessions", "No relay sessions", the active/inactive counts and
   description) are genuine NTLM relay sessions (rule 6), and `common.session_expired`
   stays the login session.
4. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests — one
   `src/lib/i18n/index.test.tsx` localStorage case failed once in the parallel full run,
   passed alone and on the full rerun (test isolation, not a copy dependency); `npm run
   check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path 972 kB); `out/`
   rebuilt and dist re-synced (337 files); full e2e 184 passed / 36 skipped. None of
   credentials/NTLM/cloud is in `VISUAL_ROUTES`, so no baseline was measured or moved.

**Deployment probe.** The rebuilt binary (PID 10072 → **30312**, started 21:46:40)
answers `/health` with `ok`, and the served chunks are byte-identical to
`internal/webdist/dist` (`assets/index-CnX0HdCW.js` = `FBB3AF18…B4D4`,
`assets/zh-BmbZGV0c.js` = `894540B5…8E3F`). Fifty of fifty markers as expected: the NTLM
page serves "-- Select Beacon --", "Target Beacon", "NTLM: load beacons failed" and both
"(beacon side)" listener labels; the cloud page serves the beacon picker, "Dispatches
beacon-side credential steal tasks", "Filter by Beacon", "harvested from beacons", "from
beacon hosts" and the beacon-holds-none warning, with the old agent strings gone in both
languages. Four absence markers are deliberately scoped instead of global: "-- Select
Agent --" and "Target Agent" still belong to other domains (loot/toolkit), and "Select an
agent first" / 请先选择代理 are shared with `toolkit.toast.select_agent_first`; only the
NTLM/cloud-specific absences are asserted. The cookie-proxy and relay-session keeps are
unchanged in this build.

## U7 — Professional naming & page emoji (wave 3, body copy 13: the pivoting, topology & proxy-chain domain, landed 2026-10-19)

Four page clusters in one wave, including the lateral page that carried the last
implant strings: 43 flagged, 31 rewritten (62 value edits), 12 kept.

1. **Pivoting (12).** The relay hints name the beacon at both ends ("Direct Beacon
   SOCKS", "…on the beacon host… the beacon IP", "Beacon connects outbound", "via the
   beacon", "Traffic routing via beacon", "through beacon infrastructure"), the pickers
   follow ("-- Select Beacon --", "Target Beacon"), and the TUN block turns every
   "implant" into a beacon ("Beacon CIDR", "Linux beacons open /dev/net/tun…", "adapter
   creation needs an elevated beacon", "macOS beacons return an honest error"). ZH had
   been split between 主机 and Agent for the same entity.
2. **Topology (6).** Legends, node hints and the merged-note speak beacons ("Online /
   Offline Beacon", "Discovered by beacon", "{n} discovered IPs matched existing
   beacons", "parent beacon ID", "Beacons will appear here when they connect").
3. **Proxy chains (6).** The subtitle reads "route beacon traffic through other beacons
   (Beacon → Beacon → C2)" — the ZH 代理 there meant agent, now Beacon — with "Set
   Parent Beacon", both pickers and the cleared toast. `set_proxy_parent` keeps its
   Proxy: the dialog picks a parent that acts as the proxy role.
4. **Lateral (7).** The last implants on this page become beacons: "Online Beacon",
   "Pivot Beacon", "Select online beacon…", "Source Beacon", "Select Beacon, target host
   and execution method", and "the beacon has no pivot channel, so movement… from the
   selected beacon".
5. **The 12 keeps.** Seven local-proxy strings (ZH 代理 = proxy, the wave-4/cookie-proxy
   ruling), the two real relay-session strings on the pivoting page ("No relay sessions
   configured", "Active Sessions"), `chain.set_proxy_parent` (proxy role),
   `topology.net_view` ("controlled hosts" is the correct host-level word; EN has no
   agent term) and `common.session_expired`.
6. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests;
   `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and dist re-synced (337 files); full e2e 184 passed / 36
   skipped. None of the four pages is in `VISUAL_ROUTES` (and no e2e spec references
   topology), so no baseline was measured or moved.

**Deployment probe.** The rebuilt binary (PID 30312 → **38952**, started 22:00:25)
answers `/health` with `ok`, and the served chunks are byte-identical to
`internal/webdist/dist` (`assets/index-DsthL0H-.js` = `9921497F…2423`,
`assets/zh-CRgEOprb.js` = `F7707502…77D5`). Seventy-one of seventy-one markers as
expected: pivoting serves "Direct Beacon SOCKS", the beacon-host hint, "Traffic routing
via beacon", "Beacon CIDR" and the Linux/Windows/macOS beacon TUN note; topology serves
"Online Beacon", "Discovered by beacon", "Beacons will appear here when they connect"
and "parent beacon ID"; the proxy-chain page serves the beacon subtitle, "Set Parent
Beacon" and the beacon pickers; and the lateral page serves "Pivot Beacon", "Select
online beacon…" and "Source Beacon", with the old agent/implant strings gone in both
languages. Three absence markers are deliberately scoped instead of global: 选择代理 and
目标 Agent still belong to scripting/toolkit/bloodhound/plugins/privesc, and 在线 Implant
still appears in `ai.quick_stale_query` (the AI domain, 25 flagged). The proxy-family and
relay-session keeps are unchanged in this build.

## U7 — Professional naming & page emoji (wave 3, body copy 14: the AI domain, landed 2026-10-19)

The last major domain, and the only one of the thirteen that is also a visual route
(`ai` is in `VISUAL_ROUTES`): 26 flagged (25 `ai.*` plus `common.session_expired`),
25 rewritten (40 value edits), 1 kept.

1. **Beacon vocabulary (8).** The assistant's permission copy ("Allow AI to execute
   commands on beacons", "…on your beacons") and its empty-state promise ("list live
   beacons") now match the objects it acts on; the quick actions follow ("List online
   beacons", "online/offline beacons", "high-integrity beacons", "Based on live
   beacons", "stale (missed check-ins) beacons"). Host-level vocabulary stays: "flag
   elevated hosts" and "do not invent hosts" are about machines, not running beacons.
   The ZH side, which had drifted between 主机 and Implant for the same objects, moves
   to Beacon only where the English names a beacon.
2. **Conversation vocabulary (17).** The AI chat object is a conversation — the page
   already said so in 12 ZH strings ("新建对话", "对话历史", "暂无保存的对话", "搜索对话")
   and in its English pin/archive/branch labels. The remaining "session" wording (the
   context-budget hint, the conversation-list toggle, the five create/delete/load/rename
   toasts) now says conversation in both languages, and ten ZH-only values that trailed
   (context title, attachment gate, policy failure, pin/archive/branch states, archive
   confirmation) move from 会话 to 对话. Key names keep their `*_session` spelling: the
   parity gate counts keys and the API object is still a session.
3. **The keep.** `common.session_expired` remains a real expired login session (rule 6).
4. **Visual route measured, not moved.** The AI page is a visual route, so the
   zero-tolerance probe was mandatory: all four captures (mobile/laptop × light/dark)
   passed against the committed win32 baselines. The reason is structural — `prepare()`
   mocks `/api/ai/profiles` as `[]`, so the captured state is the "not configured" card
   and never renders the greeting or the quick actions that changed. No baseline is
   adopted, and linux is not re-rendered.
5. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests;
   `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and dist re-synced (337 files); full e2e 184 passed / 36
   skipped.

**Deployment probe.** The rebuilt binary (PID 38952 → **38004**, started 22:21:47)
answers `/health` with `ok`, and the served chunks are byte-identical to
`internal/webdist/dist` (`assets/index-DlXzCzK0.js` = `8DEB9661…188B`,
`assets/zh-DC-Q7ZhV.js` = `A1E6B8DE…7F5E`). Seventy of seventy markers as expected:
English serves the beacon permission and quick actions ("Allow AI to execute commands on
beacons", "list live beacons", "List online beacons", "online/offline beacons",
"high-integrity beacons", "Based on live beacons", "beacons that look stale…"), keeps
"flag elevated hosts" and "do not invent hosts", and speaks conversation everywhere
("consider starting a new conversation", "Toggle conversations", the five
create/delete/load/rename toasts); Chinese serves the same set (允许 AI 在 Beacon 上执行命令,
列出在线 Beacon, 结合当前 Beacon, 建议开启新对话, 切换对话列表, 对话上下文, 置顶对话, 归档该对话)
with every old Agent/主机/Implant/会话 form absent. Key names are untouched (the bundles
still call the API's `*_session` objects); the one kept string is `common.session_expired`.

## U7 — Professional naming & page emoji (wave 3, body copy 15: the operation pages' pickers, landed 2026-10-19)

The long tail's first half — six page clusters whose flags are almost all `<select>`
pickers and table columns: 37 flagged, 32 rewritten (64 value edits), 5 kept.

1. **Scripting (8).** The editor's beacon picker and its four templates ("Select Beacon",
   "Target Beacon", "Beacon Enumeration", "Create tasks for all beacons", "Run network
   scan on all beacons", "Wait for beacons to check in" — the last one also settles the
   callback → check-in rule).
2. **BOF (4).** The library's target column, picker, available count and no-beacon toast
   all say beacon.
3. **Privilege Escalation (6).** The subtitle, the target picker and its validation, the
   new-task hint and the column.
4. **Toolkit (5).** "Beacon Info", "Stop beacon TUN", "Target Beacon:", the picker
   placeholder and the select-first toast.
5. **BloodHound (4) + scanner (5).** The collection dialog's beacon selector and its
   column; the scanner's honesty card ("runs from the beacon context", "Beacon-side recon
   scan") and its pickers.
6. **The 5 keeps.** The scripting `agents` global is an API identifier (the copy names the
   variable); "Enumerate Remote Desktop sessions" and "Logged-on sessions, idle, active
   window" are rule-6 real sessions; the BOF run confirmation's only hit is its `{agent}`
   placeholder (it renders the hostname, not the word); BloodHound's `Session` is the
   collection method sent to the backend.
7. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests;
   `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and dist re-synced (337 files); full e2e 184 passed / 36
   skipped. No page here is in `VISUAL_ROUTES`.

**Deployment probe.** The rebuilt binary (PID 38004 → **19988**, started 23:42:32)
answers `/health` with `ok`, and the served chunks are byte-identical to
`internal/webdist/dist` (`assets/index-C_MzXKIQ.js` = `9074E516…C8D1`,
`assets/zh-DEzxG3Y3.js` = `EDACBB54…1606`). Ninety-eight of ninety-eight markers as
expected: scripting serves "Select Beacon", "Beacon Enumeration", "Wait for beacons to
check in"; BOF "Select Beacon...", "Available Beacons"; the privilege-escalation page the
beacon subtitle, picker and validation; the toolkit "Beacon Info", "Stop beacon TUN",
"Select a beacon first"; BloodHound the beacon collection dialog; the scanner "runs from
the beacon context" and "Beacon-side recon scan" — with every old agent form gone in both
languages. The five keeps are intact ("Enumerate Remote Desktop sessions", "Logged-on
sessions, idle, active window", "Scopes the `agents` global…", the BOF `{agent}`
placeholder). Absence is scoped as before: "Target Agent" / 目标 Agent still live in
`plugins.*` (wave 16).

## U7 — Professional naming & page emoji (wave 3, body copy 16: the long tail's second half, landed 2026-10-19)

Eighteen namespaces in one sweep — the last flagged values that were neither a page's own
domain nor a documented keep: 42 flagged, 38 rewritten (75 value edits), 4 kept.

1. **Operations pages.** opsec (4: the beacon column, the subtitle and the two rekey tiles
   that named agents), campaign (5: breakdown title, columns, stat tiles and the seeded
   toast), the report page (5: the "Implant" section label → **Beacon**, the details
   heading, the column, the total tile and the count unit "agents" → "beacons"; ZH keeps
   台).
2. **Asset organisation.** groups, tags and the three asset-management cards — every
   "Agent" that organised, labelled or filtered beacons.
3. **Selection and chrome helpers.** attack/loot scope options ("All Beacons", "Filter by
   beacon"), the attack load warning, the plugin execution picker, the token table, the
   command palette action ("Kill **Beacon**"), the shortcuts description and the templates
   empty state.
4. **Rule-5 rewrites, not noun swaps.** The DNS listener copy ("Agents send A-type queries
   as beacons" → "**Beacons** send A-type queries as their **check-in**… each **beacon**
   UUID") and the domain-fronting note ("Agents should be generated" → "**Payloads** should
   be built" — the artifact is a payload).
5. **The 4 keeps.** opsec's crypto sessions (rekey status, active sessions, the
   `session rekeys` source that doubles as an API-response lookup key, and the unread
   banner), all rule 6.
6. **The sweep is complete.** The inventory now reports 73 flagged values, every one
   adjudicated as a keep: `User-Agent` headers (agents config, generate, profiles,
   settings, stager), real login/JWT sessions (users, session, settings, login, profiles),
   real protocol sessions (NTLM relay, opsec crypto, RDP/WTS, logon, recon, AMSI,
   BloodHound's Session collection method), the proxy families (cred cookie proxy,
   pivoting local proxy), the WeChat 会话 chat word, the scripting `agents` global, the BOF
   `{agent}` placeholder, `topology.net_view`'s controlled hosts, and
   `common.session_expired`.
7. **Verification.** eslint and `tsc --noEmit` clean; 172 files / 883 unit tests;
   `npm run check` 11/11 (parity 4727, bundle 3.99 MB / 190 files, critical path
   972 kB); `out/` rebuilt and dist re-synced (337 files); full e2e 184 passed / 36
   skipped.

**Deployment probe.** The rebuilt binary (PID 19988 → **18184**, started 23:51:09)
answers `/health` with `ok`, and the served chunks are byte-identical to
`internal/webdist/dist` (`assets/index-DmuSAS5L.js` = `DD81CC09…5B88`,
`assets/zh-DCqxLg01.js` = `1A568938…9D1E`). Eighty-one of eighty-one markers as expected:
opsec serves "Beacon ID" and "Rekeyed Beacons"; campaign "Beacon Breakdown" and
"({skipped} beacons skipped)"; the report serves "Beacon Details", "Beacon Count" and
`"report.units":"beacons"`; groups/tags/asset cards, attack/loot ("All Beacons", "Filter
by beacon"), the plugin picker, the token table, "Kill Beacon", the palette and shortcut
copy, the DNS check-in sentence and the payload-build line all match — with every old
agent/代理 form gone in both languages. No scoped absences were needed this time: all 45
absence candidates count zero in the source (the single FAIL during authoring was a typo'd
marker — a spaced `"units": "beacons"` — not a product defect).

## U7 residue - Plugins execute dialog fallbacks (landed 2026-10-20)

The live `plugins.execute_agent` / `plugins.execute_agent_ph` values were
already "Target Beacon" / "Select target beacon", but the JSX fallbacks still
read "Target Agent" / "Select an agent (optional)". Those literals are only
reachable when a translation is missing; both now match the vocabulary. No
visual or i18n-key change: 883 tests, e2e 184/36 and the bundle gates pass, and
the built Plugins chunk contains none of the banned literals.

## Batch map & reproduction

| Batch | Clusters |
| --- | --- |
| U1a status tokens & primitives (landed 2026-10-18) | C1, C2, C5, C6, C8, C9; component side of C3/C4/C7; `check:contrast` compositing switched to the browser's sRGB blend space |
| U1b page migration (landed 2026-10-18) | C3/C4/C7 page call sites: 300 × `text-X` → `text-X-ink`, 38 `-foreground`-on-tint chips → ink, light `primary/20` → `/15`; `check:contrast --strict` promoted into `npm run check` |
| U1c type system (landed 2026-10-18) | T1 size ramp (`--text-*` + 360-site migration + token gate Rules 5/6), T2 empty-state wrap, T8 no-op opacity sweep || U2 keyboard & names (landed 2026-10-18) | K1–K5: aria-labels on the video overlay and the phase chip, named breadcrumb home link, single skip link, labelled table region; `check:aria --strict` promoted into `npm run check` |
| U3 feedback loop (wave 1: settings tree, wave 2: agent-detail console, wave 3: users page, wave 4: persistence page, wave 5: phishing page, wave 6: infrastructure page, wave 7: collect/stealth sections, wave 8: agent sub-pages + the right-rail AI card — landed 2026-10-18) | F1/F2: `useMutation` call sites 6 → 85 across the eight waves; wave 1 retires the settings tree's raw-message error toasts (4 → 0) and makes previously-silent failures report; wave 2 migrates the three `/agents/:id` action hooks plus Diagnose/Apply-sleep, discards in-flight completions on an agent switch (notes editor, quick-shell history, sleep toast, Enter-key guard), and routes the console's 9 hand-rolled failure toasts through `useErrorToast`; wave 3 migrates the users page (dialog submits gain pending guards, `revokeLoading` retires into per-action pendings, the toggle/delete row spinners stay local by design) and retires its 10 raw error toasts; wave 4 migrates the persistence page (list/install/remove onto mutations, the poller's run-id guard replaces the shared cancel boolean, the route-id effect resets the page identity) and retires its 7 raw error toasts including the duplicate load toast; wave 5 migrates the phishing page and its Entra identity panel (ten mutations, the panel's shared boolean becomes the OR of four pendings so a first completion cannot re-enable the controls while a second request is in flight, the load failure becomes the inline `DataError` + retry, stop failures gain their own key) and retires the cluster's 10 raw error toasts; wave 6 migrates the infrastructure cluster (eight mutations; the SSH probe's dead `data.success` branch retires with its key, `SSH OK: undefined` becomes the reachability message, overlapping generations keep only the latest, a failed generation reports alongside its fallback template, the deploy banner is normalized, the duplicate load toasts retire) and retires its 9 raw error toasts; wave 7 migrates the collect/stealth cluster (the shared dispatcher behind nine sections plus Evasion/Inject/WebcamMic/Process; `errorText` becomes an i18n `errorKey` fallback so mapped statuses localize and 4xx backend messages surface, agent switches discard in-flight completions and clear agent-bound drafts cluster-wide, `check-i18n` learns `errorKey:` data) and retires its 7 raw error toasts; `useErrorToast` gains `fallbackParams`; the route-id reset rule lands in `FRONTEND_CONVENTIONS.md`; wave 8 (the closing wave) migrates the agent sub-pages and the right-rail AI card — the `/agents/:id` config/traffic/token pages, the screen-monitor and remote-desktop hooks, and `AISuggestCard` — onto mutations and the normalizer, retires the traffic page's duplicate load toast and the two dead `res.success` branches (with the `agents.traffic_adapt_error` key), adds the explicit whoami error flag and the `screen.capture_failed` key, and retires the tail's 24 hand-rolled failure toasts, so every cluster named for U3 has landed and the raw-toast tail named at wave 7 is retired; the agents list page's dock/menu helpers were never on the U3 map — wave 9 (follow-up, landed 2026-10-19) migrates them: `AgentDockCommands` (three mutations; the row `busy` identity stays local), `AgentDockFiles` (normalized load/pull/push; the per-row transfer fan-out stays hand-rolled so a sibling completion cannot be dropped), `AgentInteractDock` (approve/reject/cancel through the normalizer; the silent first-load failure gains an inline `ErrorState` + retry), `useAgentMenuAction` (screenshot/beacon-now) and `useAgentBulkOps` (nine catches into the action banner) — and single-sources the rule as `errorText`/`resolveErrorText` in `lib/errors.ts` (the infrastructure page's local copy retires): `useMutation` call sites 85 → 88, `useErrorToast` importing files 32 → 37, the cluster's raw `e.message ?? generic` sites 6 → 0 and its reason-swallowing failure copy 15 → 0; wave 10 (follow-up, landed 2026-10-19) takes the last `formatThrownError` action surfaces (`NtlmPage`'s three mutations behind one derived pending, `TemplatesPanel`'s save mutation plus the inline `ErrorState` + retry its silent load gained, `ToolkitPage`'s quick-action and template-run routing, and the pivoting cluster's normalizer routing with multi-row fan-out kept hand-rolled), retiring the helper from every action surface: `useMutation` call sites 88 → 92, `useErrorToast` importing files 37 → 42, `formatThrownError` action-surface call sites 11 → 1 (`ShellTerminal`'s deliberate 409 branch survives); wave 11 (follow-up, landed 2026-10-19) normalizes the `/ai` conversation workspace's 18 raw-message sinks across six files (`AIPageContent`'s six action surfaces, `AIAssistBar`, `AIContextPanel`, `PendingAIIntents`, `useAIConfig`, `useAISessions`; the config and session-list loads resolve once through `errorText` for both the inline banner and the toast): `useErrorToast` importing files 42 → 48, the cluster's raw `e.message` sinks 18 → 0, and the repo-wide `x.message`-ternary tail now stands at 76 (12 toast-level, 33 inline, 31 helper); wave 12 (follow-up, landed 2026-10-19) normalizes the tasks and macros workspaces' 13 failure sinks across five files (`TasksPageContent`'s load plus the `mutationError` helper behind its six task actions, `AIAnalysisPanel`, `NLQueryDialog`, `AIPlaybookDialog`, and `MacrosPageContent`'s three loads and four actions — delete and stop previously swallowed the reason whole — with the load surfaces resolved once through `errorText` for both the inline banner and the toast): `useErrorToast` importing files 48 → 52, the cluster's raw `e.message` sinks 13 → 0 (grep-verified), and the repo-wide `x.message`-ternary tail now stands at 65 (6 toast-level, 29 inline, 30 helper); wave 13 (follow-up, landed 2026-10-19) takes the agent console cluster — the deliberate 409-approval-warning trio (`AISuggestCard`, `useAgentQuickShell`, `AgentDetailPage`) unified through the new warning-toned `warnError` (same normalization and message rules as `toastError`; a message-less approval notice falls back to the surface's copy instead of toasting an empty string, and the quick-shell history row resolves the same reason through `errorText`), plus five inline raw-message reads retired through `errorText` (`HostInfoCard`'s sweep failure — the cluster's one displayed string, where a plain rejection object used to render the generic copy for every failure — and the display-inert `TimelineSection` timeline load, `TokenPage` token-list and process-list loads, and `useAgentData` lock table, whose plain-rejection form used to store `[object Object]`): `useErrorToast` importing files 52 → 53, the cluster's raw `e.message` sinks 9 → 0 in the touched files (grep-verified), and the repo-wide `x.message`-ternary tail now stands at 56 (3 toast-level, 23 inline, 30 helper); wave 14 (follow-up, landed 2026-10-19) takes the agent files workspace `useAgentFiles` — the hook behind both `/agents/:id/files` and the dock's file panel — where 15 `String(err)` sites and two `.message` reads retire (17 → 0 raw sinks): twelve un-prefixed toasts through `toastError`, the three prefixed batch fan-outs and both displayed states (the listing retry card and the OS blocker) through `errorText` with their toasts' fallback keys, plus the new `agents.files_drives_failed` key — `useErrorToast` importing files 53 → 54, repo-wide `String(err)` sites 30 → 15, and the `x.message`-ternary tail 56 → 53 (2 toast-level, 21 inline, 30 helper); wave 15 (follow-up, landed 2026-10-19) closes the settings tree's display states — the five single-surface unread warnings (API keys, ExtC2, notification routes, notifications, SIEM) and the three dual-surface loads (`BackupSection`, `ModulesSection`, `useSettingsData`) plus `ReloadStatusCard`'s probe move onto `errorText`/`toastError` (the touched files' raw reads 9 → 0; dual surfaces resolve once, so warning and toast cannot disagree), with `EmergencySection`/`useTOTP` verified as presence flags whose string is never rendered — `useErrorToast` importing files 54 → 55 and the repo-wide `x.message`-ternary tail 53 → 44 |
| U4 narrow screens (landed 2026-10-18) | R2 fixed (status label always rendered), T6 mobile palette entry, T7 touch targets, R3 extended to 10 routes |
| U5 motion (landed 2026-10-18) | M1: `router.navigate` defaults into `document.startViewTransition` (`?? true` beats a `Link`'s undefined key; explicit `false` kept), reduced-motion and API-absent skip; `src/router-transitions.test.ts` + the 4-viewport Playwright `route transitions` spec; `globals.css` untouched |
| U6 page-by-page polish | T3 focus mode, T4 stat tiles, T5 density selectors, T9 non-total list fallbacks (tokens, ntlm), T10 clipped fixed-width columns (toolkit, scripting), V1–V2 + per-page checklists |
| U7 naming & personality (wave 1, landed 2026-10-19) | "Beacon" vocabulary across nav/titles/tiles (45 nav + 42 titles + 5 tile/subtitle values per language, 92 keys); one emoji identity per page (nav = h1 = breadcrumb = document title, `aria-hidden` in chips); U6 residue: DNS tile tone, card-header 2-line clamp, privesc mojibake icons, geo 📍 fallback; contract in `docs/TERMINOLOGY.md` — L2 routes landed in wave 2, body copy swept page domain by page domain from wave 3 (Beacons done) |
| U7 naming & personality (wave 2, landed 2026-10-19) | L2 routing layer: six route renames plus the `/agents/:id` family → `/beacons/:id`, the 15-entry `LEGACY_REDIRECTS` table (id/sub-route/query/hash preserved), 175 files moved (page directories + route-entry components), 151 link literals across 48 files, visual baselines renamed not regenerated, 7 new e2e redirect cases; HTTP API, sqlite schema and OpenAPI untouched per the L3 boundary |
| U7 naming & personality (wave 3, landed 2026-10-19) | L1 body copy, first page domain (Beacons): inventory flags 100 values (97 `agents.*` + 3 `status_banner.*`), 87 rewritten per language (170 value edits, keys untouched per rule 3), 13 kept with reasons (UA header family, AMSI-session technique, recon/logon/RDP/console sessions, WeChat chat); new rules recorded in `docs/TERMINOLOGY.md` (running implant → beacon, generated artifact → payload, check-in event → "check-in" / 回连, `detail_session` → `Beacon`); two e2e locators follow their copy; win32 baselines refreshed only for the four `beacons-*` files, ten regenerated candidates rejected as unrelated noise (evidence: banner `null` under mocked stats, no rewritten key on those pages, byte-identical re-regeneration); the linux half landed from `visual-baselines.yml` run 39220123316 with the same scoping (4 of 28 rendered files adopted, 3 rejects all under 0.5 % of the canvas) |
| U7 naming & personality (wave 3, body copy 3, landed 2026-10-19) | L1 body copy, third page domain (automation cluster): inventory flags 27 across six namespaces (`auto.*` 6, `scheduler.*` 4, `oneshot.*` 4, `workflows.*` 5, `macros.*` 5, `autotag.*` 3), 19 rewritten per language (38 value edits), 0 kept, 8 deferred to their own pages (macros / auto-tag rules); event and rule-type labels become "Beacon offline/online" while the wire enum ids stay `agent_*`; `oneshot.*` closed here after wave 2's deferral; no locator or baseline depended on the strings |
| U7 naming & personality (wave 3, body copy 4 — residue pass, landed 2026-10-19) | L1 body copy, scope fix: the Beacons cluster was inventoried by key prefix in wave 1, leaving `shell.*` 11, `cred.*` 14, `screen.*` 2, `hostinfo.*` 1 unreviewed; 28 flagged, 21 rewritten (39 value edits), 7 kept (the cookie-proxy family: 代理 = proxy), 0 deferred; shell tabs are beacon-keyed so "Shell sessions" → "Beacon shells", "Switch session" → "Switch beacon"; method now starts from the pages, and the re-scan queued `listeners.*` (rendered by the payload generator) as its own wave |
| U7 naming & personality (wave 3, body copy 5, landed 2026-10-19) | L1 body copy, fourth page domain (listeners, run with the method fix): page-cluster scan over `pages/listeners/**` finds `listener.*` (singular) that prefix lists missed — 12 flagged, 12 rewritten (24 value edits), 0 kept, 2 deferred by attribution (`cb.honesty_desc` → circuit-breaker page, `dashboard.quick_generate` → dashboard); detail stats/headings/empty/action and list columns/subtitle/tip/hint swept; win32 visual tests pass with no snapshot update (diff under the 1.5 % tolerance), linux set re-rendered from this commit and compared |
| U7 naming & personality (wave 3, body copy 6, landed 2026-10-19) | L1 body copy, fifth page domain (macros + auto-tag rules, plus the shared `ai.pb_agent` the macros page renders): 9 flagged, 9 rewritten (17 value edits; `macros.empty_message` EN-only), 0 kept, 0 deferred; `settings.modules.no_agents` keeps "No agents available" by design (its own domain) so it is excluded from the bundle absence check; neither page is a visual route and the shared key sits in a dialog → no baseline move |
| U7 naming & personality (wave 3, body copy 7, landed 2026-10-19) | L1 body copy, sixth page domain (users): 14 flagged, **2 rewritten** (4 value edits — the role legend's beacon-permission bullets), **12 kept under rule 6** (every session string is a real login session), 0 deferred; no visual route, no baseline |
| U7 naming & personality (wave 3, body copy 8, landed 2026-10-19) | L1 body copy, seventh page domain (circuit breaker): 2 flagged, **1 rewritten** (2 value edits — `cb.honesty_desc`'s "implant multi-C2 config" → "the payload's multi-C2 config", closing wave 5's handover), **1 kept under rule 6** (`common.session_expired`, a real login session in the global 401 path), 0 deferred; `circuit_breaker.*` clean; non-visual route, no baseline (the one `ai light` mobile diff was a font-swap flake, passed on rerun, not adopted) |
| U7 naming & personality (wave 3, body copy 9, landed 2026-10-19) | L1 body copy, eighth page domain (dashboard): 1 rewritten (`dashboard.quick_generate` "Generate Implant" → "Generate Payload" / 生成木马 → 生成载荷 — the last 木马 in the copy), 0 kept, 5 `tasks.*` deferred by attribution to the tasks page; visual route **measured** with a zero-tolerance probe: 78–536 px (≤0.05 %) on mobile/laptop light+dark → no baseline adopted, linux not re-rendered |
| U7 naming & personality (wave 3, body copy 16, landed 2026-10-19) | L1 body copy, the tail's second half (18 namespaces: opsec, campaign, report, groups, asset_org, attack, loot, plugins, tags, tokens, command, dns, domain_fronting, notifications, palette, shortcuts, templates, traffic): 42 flagged → 38 rewritten (75 value edits — the last beacon nouns plus the DNS check-in and payload-build rule-5 rewrites), 4 keeps (opsec crypto sessions), 0 deferred; the inventory's remaining 73 flags are all documented keeps; no visual route |
| U7 naming & personality (wave 3, body copy 15, landed 2026-10-19) | L1 body copy, fourteenth page domain (long tail i: scripting + BOF + privesc + toolkit + BloodHound + scanner): 37 flagged → 32 rewritten (64 value edits — every beacon picker, column, template hint and honesty card on the six pages), 5 keeps (the `agents` scripting global, two real-session strings, the BOF `{agent}` placeholder, BloodHound's Session collection method), 0 deferred; no visual route |
| U7 naming & personality (wave 3, body copy 14, landed 2026-10-19) | L1 body copy, thirteenth and last major page domain (AI): 26 flagged → 25 rewritten (40 value edits — the assistant's beacon permission/greeting/quick actions, and the chat surface's session→conversation wording in EN and ZH), 1 keep (`common.session_expired`), 0 deferred; visual route measured at zero diff (the layout spec mocks no AI provider, so the captured card never renders the changed copy) — no baseline |
| U7 naming & personality (wave 3, body copy 13, landed 2026-10-19) | L1 body copy, twelfth page domain (pivoting + topology + proxy-chains + lateral): 43 flagged → 31 rewritten (62 value edits — the pivoting relay/TUN hints, topology legends and nodes, the chain subtitle/parent/pickers, and the lateral page's last implant strings), 12 keeps (7 local-proxy strings where 代理 = proxy, 2 real relay sessions, `chain.set_proxy_parent`, `topology.net_view`'s controlled hosts, `common.session_expired`), 0 deferred; no visual route, no baseline |
| U7 naming & personality (wave 3, body copy 12, landed 2026-10-19) | L1 body copy, eleventh page domain (credentials + NTLM + cloud): 31 flagged → 17 rewritten (34 value edits — the NTLM pickers/targets/toasts/listener sides, the cloud pickers, beacon-side tasks, "harvested from beacons", "from beacon hosts"), 14 evidence-backed keeps (7 cookie-proxy strings where 代理 = proxy, 6 real NTLM relay sessions, `common.session_expired`), 0 deferred; none of the three pages is a visual route, no baseline |
| U7 naming & personality (wave 3, body copy 11, landed 2026-10-19) | L1 body copy, tenth page domain (settings): 29 flagged → 23 rewritten (46 value edits — About "Implants"→Beacons, the agent section/title/toast/subtitle, the kill switch (implants→beacons, next beacon→next check-in), module deploy, the routes alert, the database card, the access cards' session-table wording, the last "No agents available"), 6 rule-6 keeps (login sessions + the literal User-Agent header), 0 deferred; settings visual route measured: mobile dark pixel-identical, others 46–377 px (0.01 %) → no baseline adopted |
| U7 naming & personality (wave 3, body copy 10, landed 2026-10-19) | L1 body copy, ninth page domain (events + tasks): 9 rewritten (18 value edits — the events follow toggle's "session" is the dock's beacon (`dockAgentId`), tasks filter/column/empty state, and the two `ai.nl_*` keys the tasks dialog owns), 0 kept, 0 deferred (25 `ai.*` remain); app fix: empty AI conversations no longer scroll to the bottom (the 3–4 % mobile visual flake, root-caused); events visual route measured: 3/4 captures pixel-identical, mobile light 80 px (0.02 %) → no baseline adopted |
| U7 naming & personality (wave 3, body copy 2, landed 2026-10-19) | L1 body copy, second page domain (payload pipeline): inventory flags 33 values (`generate.*` 14, `profiles.*` 10, `builds.*` 5, `oneshot.*` 4), 24 rewritten per language (46 value edits), 5 kept (proxy / UA header / login sessions), 4 deferred to the automation card; "Agent Binaries" → **Payload Formats** / 载荷格式 (smoke locator follows); format descriptors name the format (GUI implant → GUI executable) and subjectless clauses replace the banned noun; no visual route touched, so no baseline churn on either platform |

```powershell
cd frontend
npm run check                     # strict contrast + aria + token gates
node scripts/check-contrast.mjs   # report-only runs while iterating
node scripts/check-contrast.mjs --selftest
node scripts/check-aria.mjs --list
node scripts/migrate-fs-utilities.mjs --dry-run   # T1 regression sweep
npm run test:e2e                  # layout + smoke + visual baselines
```

Runtime probes (tab walk, screenshots, canvas contrast harness) were temporary
by design and are not committed; their method is reproducible from this file.

## U9 — Layout & UI program (2026-10)

U1–U8 closed colour, type, keyboard, motion, copy and the card header. What
the audit deliberately left out of scope — page re-layout and the spacing
contract — is what U9 opens, with the same evidence discipline: measure
first, change the primitive, keep the gates green.

### Method

A Playwright harness drives the **live** console (real login, real data) over
10 operator routes × 4 viewports (390 / 768 / 1024 / 1440) and records, per
route: the page container's width and row gap, the gap between the page's own
blocks, whether the filter row is `position: sticky` and where it lands after
an 800 px scroll, the page-level horizontal overflow, and the horizontal
overflow **inside** every table container.

### L1 · The shell clamped the content width twice

`AppLayout` centred its column at `--content-wide` (96rem) and
`PageContainer` applied `--content-standard` / `--content-wide` again on the
inside. Two nested max-widths, one value: harmless until a page needed a
narrower column, then the outer clamp fought the inner one.

**Fixed**: the shell owns the page padding only; the content width belongs to
`PageContainer` (`standard` / `wide` / `workspace`). The padding moved from
hard-coded Tailwind steps (`px-4 sm:px-6 lg:px-8` …) to three tokens
(`--page-pad-x` / `--page-pad-y` / `--page-gap`) that step at 640 px and
1024 px exactly like the steps they replaced — measured identical page widths
before and after (358 / 720 / 728 / 1144 px at the four viewports).

### L2 · Four vertical rhythms on one screen

Page blocks were spaced by `space-y-3` (71 call sites), `space-y-4` (68),
`space-y-5` (11) and `space-y-6` (6) — none of them breakpoint-aware, all of
them fighting the container's own `gap-4 sm:gap-5`.

**Fixed**: the container gap and the page's own group roots read
`--page-gap` (16 px < 640, 20 px above). Measured after the change: every
route reports the same row gap (16 px at 390, 20 px at 768/1024/1440) and the
dashboard's inner blocks report 16 / 20 px. Forms, dialogs and panel bodies
keep their `space-y-4` — a form's rhythm is its own business.

**Trap, worth writing down**: Tailwind v4 does **not** compile `space-y` with
a custom property. Neither `space-y-(--page-gap)` nor
`space-y-[var(--page-gap)]` produces a rule (verified against the built
CSS: the `.space-y-\(--page-gap\)` selector is absent), and the page silently
loses every block gap. Page-level groups use `gap-(--page-gap)` on a
`flex flex-col` container.

### L3 · A sticky filter row that could never stick

`position: sticky` is bounded by the top edge of its containing block. When
the filter row was the **first** child of the page body, that containing
block's top edge *was* the filter row's top edge — the travel budget was
zero and the row could never pin, however long the page scrolled. The
Dashboard's view switcher lived in the shell's `toolbar` slot instead, which
had no sticky behaviour at all: it scrolled away with the panels.

Measured after the fix (800 px scroll, 390 px viewport): the Dashboard row
pins at 60 px (56 px top bar + the band's 4 px padding), the Audit and
Plugins rows pin at 56 px, and no route reports a row that scrolls away
while its page still has content below.

**Fixed**: one implementation — the `PageContainer` `toolbar` slot, sticky by
default (`stickyToolbar`, opt-out for the panels embedded in the Events tab
rail, whose own rail is already sticky). The dead `stickyHeader` and `aside`
props are gone (0 call sites).

### L4 · A nested `<main>` and a 100vh overflow

`SystemStatePage` (the 403 surface) rendered a `<main>` inside the shell's
`<main>` — invalid HTML, and `min-h-screen` in a column that already
reserves the top bar's height overflowed the viewport by exactly one bar.

**Fixed**: a `<div>` with `min-h-full`.

### L5 · Cards had no interior padding contract

`Card` derives its padding from `--card-spacing` (16 px, 20 px from 640 px),
but 62 call sites overrode it with a hard-coded step — `p-3`, `p-4`, `p-5`,
`p-6`, `p-3.5`, `p-3 sm:p-4` — so a card's interior was 12, 16, 20 or 24 px
depending on which page it sat on. `SectionCard` made it worse: it renders
its body unpadded, so every consumer padded by hand.

**Fixed**: 62 paddings now read `p-(--card-spacing)` (the flush cards —
`p-0` — keep their edge-to-edge table). The dashboard's five hand-copied
list rows became one `ListRow` primitive (leading slot, two-line text, right
slot) on the shared hover surface.

### L6 · The operator's actions were scrolled out of sight

A table that is wider than its container scrolls sideways, and the last
column — the one with the buttons — went with it. The Listeners table
clipped "Copy / Edit / Stop / Delete" by **144 px on a 1440 px screen**;
below 1024 px the whole action column was gone.

**Fixed**: the action column pins. A new `sticky-end` slot keeps the cell
glued to the right edge and paints the row's own surface (stripe, hover) so
the columns sliding underneath never show through it. Secondary columns now
step out with the viewport instead of only below 640 px (Listeners: type /
tags / agents below 1024 px, address below 768 px, notes below 1536 px;
Beacons: a 850 px floor became 640 px until 1280 px; Circuit Breaker, Campaign
and OPSEC each drop one column on phones).

**Contract**: a table that scrolls horizontally must pin a column. The
layout audit enforces it — it now reports an overflowing table as a
violation unless one of its cells is sticky, and the run is green
(45 routes × 4 viewports, 0 violations).

### Backlog (U9.4 → U9.7)

| Batch | Defect | Evidence |
| --- | --- | --- |
| U9.4 | Dialogs: 72 `DialogContent` instances across nine widths (`default` 16, `md` 18, `lg` 14, `2xl` 8, `3xl` 4, `sm` 4, `xl` 3, `4xl` 2, `5xl` 2) plus one raw `w-[95vw]`; the footer's negative margins are hard-wired to the popup's `p-5` | measured in the harness above |
| U9.5 | Detail pages (beacon: 15 sections, listener, campaign, report, circuit breaker) and the four workspace surfaces (shell / files / screen / remote desktop) each grow their own chrome | page-by-page review |
| U9.6 | Density mode changes the type ramp and the table cell padding only — not the card padding, the page gap or the control heights | `[data-density="compact"]` in `globals.css` |
| U9.7 | No layout gate: the defects above are all reachable by a single class name | — |
