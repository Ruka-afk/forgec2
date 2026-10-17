# Frontend UI/UX audit (2026-10)

Evidence base for the UI/UX program agreed in 2026-10. Scope: the operator
console in `frontend/` (~54 routes, ~200 page components) audited as **polish
within the existing design language** — consistency, accessibility, feedback,
motion and narrow-screen quality. Out of scope by decision (do not re-open):
information-architecture changes, page re-layouts, a new component system.

Defect clusters below carry an owner batch (U1–U6) and the evidence that
reproduces them. `npm run check:contrast` / `npm run check:aria` are the two
new report-mode scanners (prototypes from this batch; strict flags already
exist, promotion into `npm run check` happens in U1/U2 once clean).

## Method

- **Static scan** — `frontend/scripts/check-contrast.mjs`: parses the token
  blocks in `globals.css`, builds every pair the UI renders (solid fills,
  8–20 % tinted surfaces over background/card, text-on-surface, charts, rings)
  and computes WCAG ratios; `--selftest` pins the color math (white/black = 21,
  `#777` cases, alpha compositing). 288 pairs across light+dark checked;
  **114 text pairs below 4.5** (≈10 root causes with many usage variants),
  2 non-text below 3.0, 6 informational (input/border boundaries).
- **Browser cross-check** (temporary harness, not committed): the exact
  Tailwind class combinations were rendered in Chromium against the built CSS
  and measured from canvas pixels — the browser's own oklch/color-mix
  compositing — to confirm the static math. Where the two disagree the browser
  is authoritative: the static script composites alpha tints in linear space
  and reads ~0.3–1.3 high on 8–20 % tints.
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

## D5 — Motion (owner: U5)

- **M1 · No route transitions** — `startViewTransition` has 0 call sites while
  `globals.css` already carries `::view-transition` rules *and* their
  reduced-motion override (lines ~480). Progressive-enhancement opportunity:
  add it where route chunks now swap instantly.
- Skeleton usage (80) and `animate-*` classes are consistent; the global
  reduced-motion kill switch covers third-party animation.

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

## Batch map & reproduction

| Batch | Clusters |
| --- | --- |
| U1 contrast & dark theme | C1–C10; promote `check:contrast --strict` into `npm run check` |
| U2 keyboard & names | K1–K5; promote `check:aria --strict` |
| U3 feedback loop | F1–F2 |
| U4 narrow screens | R2–R3 |
| U5 motion | M1 |
| U6 page-by-page polish | V1–V2 + per-page checklists |

```powershell
cd frontend
npm run check:contrast            # report; --strict exits 1 on text failures
npm run check:contrast -- --selftest
npm run check:aria                # report; --strict exits 1 on missing names
npm run check:aria -- --list
```

Runtime probes (tab walk, screenshots, canvas contrast harness) were temporary
by design and are not committed; their method is reproducible from this file.
