# Frontend conventions (operator console)

Short, enforceable rules for `frontend/`. Patterns that have a shared helper
are not open for re-invention; each section names the tool or test that keeps
the rule honest.

## Async mutations: `useMutation` + `useErrorToast`

Use `src/lib/hooks/useMutation.ts` for every operator action that awaits the
API and drives a pending state:

```tsx
const { toastError } = useErrorToast();
const { mutate: save, isPending: saving } = useMutation({
  fn: () => api.postJson(paths.x.save, form),
  onSuccess: () => toast.success(t("x.saved")),
  onError: (e) => toastError(e, "x.save_failed"),
});
```

Why not hand-rolled `useState(saving)` + `try/catch`:

- `useMutation` guards stale completions (`generationRef`) and unmounts
  (`mountedRef`), so a slow save that resolves after a newer one cannot flip
  `isPending` back or fire callbacks out of order — the hand-rolled version
  gets this wrong by default.
- `useErrorToast` normalizes the thrown value (`src/lib/errors.ts`), maps
  backend error codes to i18n keys and falls back to the caller's key. Never
  `toast.error((e as Error).message)`: raw backend text leaks untranslated
  into the UI.
- Keep *validation* toasts direct (`toast.error(t("x.required"))`): that is not
  an API failure and must not go through error normalization.

Reference migration: `src/pages/settings/components/ExtC2Section.tsx`.

## Locale-aware formatting: `@/lib/intl`

Every date, time and grouped number rendered for the operator goes through
`src/lib/intl.ts` (`formatDateTime`, `formatDateOnly`, `formatTimeOnly`,
`formatNumber`). It formats with the **UI language** (`getLocale()` from
`src/lib/i18n`), never the browser default: an OS in `en-US` must not leak
`AM/PM` into a zh console, and vice versa.

- No bare `toLocaleString` / `toLocaleTimeString` / `toLocaleDateString` and no
  `Intl.*Format` without an explicit locale tag.
- `lib/utils.ts` (`formatTime`, `nowTime`) delegates to the helpers; keep new
  call sites on those, not on raw `Date` methods.
- Invalid input degrades to `-`; never render `Invalid Date`.

Pinned by `src/lib/intl.test.ts` (locale mapping, option pass-through).

## Store selectors: return stored references

`useAppStore((s) => s.x)` compares with `Object.is`. A selector that returns a
stored reference (even an object or array) needs **no** `useShallow` — the
store replaces the value only when it actually changes:

```tsx
// fine: `stats` gets a new reference only on fetch
const stats = useAppStore((s) => s.stats);
// needs useShallow: new array on every store update
const ids = useAppStore(useShallow((s) => s.agents.map((a) => a.id)));
```

For derived containers, prefer `useMemo` over a custom equality wrapper.

## Accessibility invariants

- Every form control has an accessible name: an associated label
  (`<Label htmlFor="x">` + `<Input id="x">`), a wrapping `<label>`, or an
  `aria-label` on the control. Caption-only `<Label>` elements are the legacy
  pattern and still outnumber associated ones; name the control you touch —
  do not mass-rewrite the repo. Placeholders are not labels.
- **Spot-checked 2026-10 (batch T)**: the six "label + control already has an
  id" candidates resolved to four controls already named by `aria-label`, one
  disabled input and one Switch that got real `htmlFor`/`id` pairs, and one
  trigger that got an `aria-label`. Keep this check in the loop when a form is
  touched.
- `TableHead` renders `scope="col"` by default; pass `scope="row"` explicitly
  for row headers.

## Dead code hygiene

- `npm run audit:unused-exports` should report **0 files**. When a helper stops
  being imported, delete it or drop its `export` — do not keep an exported
  symbol "for the API". A symbol that is only used inside its own file is not
  an export.
- Delete both `en` and `zh` entries together when a key loses its last caller
  (`scripts/prune-i18n.mjs` lists candidates). `npm run check:i18n` only fails
  on *used-but-missing* keys, so dead keys rot silently.
- One route = one lazy chunk (`src/lib/route-imports.ts` + `router.tsx`); never
  import a page statically from another page.
