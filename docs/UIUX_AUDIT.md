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

## Batch map & reproduction

| Batch | Clusters |
| --- | --- |
| U1a status tokens & primitives (landed 2026-10-18) | C1, C2, C5, C6, C8, C9; component side of C3/C4/C7; `check:contrast` compositing switched to the browser's sRGB blend space |
| U1b page migration (landed 2026-10-18) | C3/C4/C7 page call sites: 300 × `text-X` → `text-X-ink`, 38 `-foreground`-on-tint chips → ink, light `primary/20` → `/15`; `check:contrast --strict` promoted into `npm run check` |
| U1c type system (landed 2026-10-18) | T1 size ramp (`--text-*` + 360-site migration + token gate Rules 5/6), T2 empty-state wrap, T8 no-op opacity sweep || U2 keyboard & names (landed 2026-10-18) | K1–K5: aria-labels on the video overlay and the phase chip, named breadcrumb home link, single skip link, labelled table region; `check:aria --strict` promoted into `npm run check` |
| U3 feedback loop (wave 1: settings tree, wave 2: agent-detail console, wave 3: users page, wave 4: persistence page, wave 5: phishing page, wave 6: infrastructure page — landed 2026-10-18) | F1/F2: `useMutation` call sites 6 → 67 across the six waves; wave 1 retires the settings tree's raw-message error toasts (4 → 0) and makes previously-silent failures report; wave 2 migrates the three `/agents/:id` action hooks plus Diagnose/Apply-sleep, discards in-flight completions on an agent switch (notes editor, quick-shell history, sleep toast, Enter-key guard), and routes the console's 9 hand-rolled failure toasts through `useErrorToast`; wave 3 migrates the users page (dialog submits gain pending guards, `revokeLoading` retires into per-action pendings, the toggle/delete row spinners stay local by design) and retires its 10 raw error toasts; wave 4 migrates the persistence page (list/install/remove onto mutations, the poller's run-id guard replaces the shared cancel boolean, the route-id effect resets the page identity) and retires its 7 raw error toasts including the duplicate load toast; wave 5 migrates the phishing page and its Entra identity panel (ten mutations, the panel's shared boolean becomes the OR of four pendings so a first completion cannot re-enable the controls while a second request is in flight, the load failure becomes the inline `DataError` + retry, stop failures gain their own key) and retires the cluster's 10 raw error toasts; wave 6 migrates the infrastructure cluster (eight mutations; the SSH probe's dead `data.success` branch retires with its key, `SSH OK: undefined` becomes the reachability message, overlapping generations keep only the latest, a failed generation reports alongside its fallback template, the deploy banner is normalized, the duplicate load toasts retire) and retires its 9 raw error toasts; `useErrorToast` gains `fallbackParams`; the route-id reset rule lands in `FRONTEND_CONVENTIONS.md`; remaining clusters (the collect/stealth sections) migrate in later waves |
| U4 narrow screens (landed 2026-10-18) | R2 fixed (status label always rendered), T6 mobile palette entry, T7 touch targets, R3 extended to 10 routes |
| U5 motion (landed 2026-10-18) | M1: `router.navigate` defaults into `document.startViewTransition` (`?? true` beats a `Link`'s undefined key; explicit `false` kept), reduced-motion and API-absent skip; `src/router-transitions.test.ts` + the 4-viewport Playwright `route transitions` spec; `globals.css` untouched |
| U6 page-by-page polish | T3 focus mode, T4 stat tiles, T5 density selectors, T9 non-total list fallbacks (tokens, ntlm), T10 clipped fixed-width columns (toolkit, scripting), V1–V2 + per-page checklists |

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
