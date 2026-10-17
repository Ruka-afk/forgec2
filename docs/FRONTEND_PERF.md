# Embedded frontend delivery (serving contract)

How `forgec2-server` serves the embedded SPA (`internal/webdist/dist`, embedded
with `//go:embed all:dist`) over HTTP: cache policy, conditional requests,
on-the-fly compression and what the shell loads before first paint. This is the
reference for the numbers quoted in `CHANGELOG.md` and for judging future
serving- and loading-layer changes.

## Where it lives

| File | Role |
| --- | --- |
| `internal/server/server_http.go` | `SetStaticFS` middleware: auth redirect for `/`+`/dashboard`, malleable-profile safety net, trailing-slash normalization, cache policy, 304 handling, compression decision |
| `internal/server/static_assets.go` | boot-time metadata (content-hash ETag, size, compressibility), ETag/`Accept-Encoding` parsing, pooled gzip writer |
| `internal/server/server_http_static_test.go` | cache/304/compression-skip matrix over a `fstest.MapFS` |
| `frontend/scripts/check-bundle.mjs` | raw bundle budget (raw bytes only; compression does not shrink what it measures) and the critical-path referee |
| `frontend/vite.config.ts` | `manualChunks`, incl. the measured base-ui core/forms split (`BASEUI_PAGE_ONLY_KEYS`) |
| `frontend/src/lib/route-prefetch.ts` + `route-imports.ts` | hover/focus + idle route prefetch; the import map is shared with `router.tsx` so both request the same chunks |

The middleware runs before route handlers and aborts the chain for static
files; beacon routes, the malleable-profile safety net, `/api`, `/ws`,
`/extc2`, `/admin/`, `/debug/`, `/rd/`, `/payloads/`, `/stage/` and
`/screenshots/` never enter it.

## Cache contract

| Request | Body served | `Cache-Control` | Validators |
| --- | --- | --- | --- |
| `/assets/<name>-<hash>.<ext>`, exact file | that file | `public, max-age=31536000, immutable` | ETag (honored too) |
| HTML shells: `/`, `/login`, `<route>.html`, `<route>/` | `<route>.html` | `no-cache` | ETag, content hash |
| `/js/*`, `/css/*`, `/favicon.ico`, other name-stable files | that file | `no-cache` | ETag |
| unknown client route (`/agents/42`, …) | `index.html` (SPA fallback) | `no-cache` | index.html ETag |
| malformed path (`..`, `\`) | 400 from `http.FileServer` | `no-cache` | none stamped |
| `/index.html` literally | 301 → `./` (FileServer canonical rule) | policy applied | ETag set before the 301 |
| API / WS / beacon / malleable GETs | route handler | API policy | untouched |

Notes:

- `embed.FS` carries no mtimes (`http.FileServer` sees the zero time), so a
  content-hash ETag is the only validator available. It is computed **once at
  boot** for every embedded file (`buildStaticAssetMeta`) — a few milliseconds
  over a ~4 MB dist — never per request.
- `no-cache` means *revalidate*, not *do not store*: a matching
  `If-None-Match` (exact, weak `W/`, or `*`) is answered `304` with no body,
  and the shell stays bfcache-eligible. The previous blanket `no-store` on
  every static response forbade both.
- The blanket API `NoCache` middleware runs earlier and stamps
  `Pragma: no-cache` / `Expires: 0`; static responses delete both so nothing
  contradicts the per-file policy.
- Immutable is granted only when the body is literally the addressed
  `/assets/` file. A stale/unknown hash falls back to the SPA shell under a
  JavaScript MIME and must not be cached under that URL.

## Compression contract

- Gzip only, stdlib `compress/gzip`, pooled writers — no new dependency.
- Applied when **all** hold: `GET`; no `Range` header; resolved body extension
  in `{js, css, html, svg, json, txt, xml, map}`; size ≥ 1 KiB; and
  `Accept-Encoding` selects gzip (`gzip` or `*`, with an explicit `gzip;q=0`
  refusal honored).
- `Vary: Accept-Encoding` is set on every negotiable response, identity
  variants included, so shared caches never hand a gzipped body to a client
  that refused it.
- A compressed response drops the identity `Content-Length` (net/http then
  uses chunked encoding, or re-computes the compressed length when the whole
  body fits its buffer). HEAD keeps identity headers, Range keeps identity
  bytes for `Content-Range`, binary formats and sub-1 KiB files are left alone.
- The path lives strictly inside the SPA static middleware: `/api`, `/ws`,
  `/extc2`, beacon routes and malleable-profile URIs are not touched, and the
  beacon hot path keeps its allocation profile.

## Loading contract (2026-10 batch S)

The shell HTML preloads only the entry chunk and its static vendor dependencies
(`vendor-*` modulepreload links); every route chunk is fetched on demand.
`frontend/scripts/check-bundle.mjs` is the referee for raw bytes: a 5.50 MB
total budget and a 1,200 KiB critical-path budget (entry + preloaded vendor
chunks + that route's own chunks).

- The login page is a lazy route (`src/router.tsx`), which keeps zod — pulled in
  through the form schema — out of the critical path: `vendor-zod` now loads
  only on pages that actually parse forms. The one extra round trip an
  unauthenticated visitor pays is compensated by `src/lib/login-preload.ts`:
  `handleUnauthorized` starts the login chunk as soon as a 401 is seen, so the
  redirect lands on a page that is already fetching.
- The base-ui split is driven by **measured reachability**, not module size: a
  probe build isolated every `@base-ui/*` subpath in its own chunk, then the
  entry's static import closure over the emitted files named the subpaths no
  entry-reachable module imports. Those keys (`BASEUI_PAGE_ONLY_KEYS` in
  `vite.config.ts`) form `vendor-baseui-forms`; everything else stays in the
  preloaded `vendor-baseui`. Re-run the probe before editing the key list.
- Prefetch is interaction-driven: sidebar and command-palette hover or focus
  call `prefetchRoute`, which shares its import map with the router
  (`src/lib/route-imports.ts`), and once the browser goes idle three likely-next
  routes (`/dashboard`, `/agents`, `/listeners`) warm — all of it skipped on
  save-data connections. Only top-level routes prefetch: `/agents/42/shell`
  would warm the wrong chunk, so it is ignored.
- The vis-network bundle (`/js/vis-network.min.js`, served by this server, not a
  CDN) is treated as a fetch that can fail: `TopologyGraph` has `onerror` plus a
  15 s watchdog, renders a fallback with a working retry, and only then gives
  up. A blocked or stalled fetch used to leave a blank canvas with no
  explanation.

## Loading measurements (2026-10 batch S, local host; raw from the bundle, gzip via zlib
level 6, the same level Go's `compress/gzip` defaults to; `curl` numbers for
the served bytes are below)

| File | Raw | Gzip | When |
| --- | --- | --- | --- |
| `index-*.js` (entry) | 380,907 | 107,705 | critical path |
| `vendor-react-*.js` | 198,366 | 62,619 | critical path |
| `vendor-baseui-*.js` (core) | 174,191 | 57,925 | critical path |
| `vendor-router-*.js` | 90,691 | 30,717 | critical path |
| `vendor-icons-*.js` | 58,532 | 18,428 | critical path |
| `vendor-misc-*.js` | 56,300 | 19,888 | critical path |
| `vendor-sonner-*.js` | 33,419 | 9,547 | critical path |
| `index-*.css` | 212,764 | 30,437 | critical path |
| `vendor-zod-*.js` | 66,393 | 17,846 | lazy (form pages) |
| `vendor-baseui-forms-*.js` | 34,102 | 11,783 | lazy (checkbox/popover/tabs/…) |
| `LoginPage-*.js` | 10,597 | 3,357 | lazy + prewarmed on 401 |
| `/login` shell (HTML) | 2,176 | 865 | first request |
| `/js/vis-network.min.js` (vendored, unhashed) | 476,620 | 128,752 | topology page |

The 7 JS chunks + 1 CSS the shell preloads: **1,205,170 raw → 337,266 gzip**
(72% saved), plus the 865-byte shell. Batch S took the 7-chunk critical path
from 1,104,896 to 992,406 raw and keeps the lazy zod / base-ui-forms / login
chunks out of `index.html`. A revisit of the shell revalidates to `304` with a
0-byte body — the payload moves only on version change.

## Server measurements (2026-10 batch R, local host, `curl -w '%{size_download}'`)

The bytes on the wire are the same files the loading table lists above; what
makes them cheap is the serving contract in this document:

- A hashed asset arrives gzip-encoded when `Accept-Encoding: gzip` is offered:
  measured `index-*.js` 380,907 raw → 107,992 gzip on the wire (Go's gzip,
  ~0.3% larger than the zlib-6 figures in the loading table),
  `/js/vis-network.min.js` 476,620 → 128,752, the `/login` shell → 865.
- A revisit of the shell revalidates to `304` in 0 bytes; immutable assets are
  never revalidated at all.
- The pre-batch-S numbers (entry 394,657, `vendor-baseui` 206,278, zod on the
  critical path at 1,317,406 raw for 8 JS + CSS) are the historical baseline
  quoted in the batch R `CHANGELOG.md` entry.

## Verification

```bash
# immutable + negotiated gzip on a hashed asset
curl -sS -D - -o NUL -H 'Accept-Encoding: gzip, deflate, br' \
  http://localhost:8000/assets/index-*.js
#   -> Cache-Control: public, max-age=31536000, immutable
#   -> Content-Encoding: gzip, Vary: Accept-Encoding, no Content-Length

# revalidation (send the quoted ETag verbatim; PowerShell 5.1 strips quotes in
# native-command argument passing, so use a client that does not mangle headers)
curl -sS -D - -o NUL -H 'If-None-Match: "<etag>"' http://localhost:8000/login
#   -> HTTP/1.1 304, no body

# unit matrix (cache policy, 304 semantics, compression skips)
go test ./internal/server/ -run TestStatic -count=1

# loading contract: budgets + which chunks the shell actually preloads
cd frontend && npm run build && npm run check:bundle
node -e "const h=require('node:fs').readFileSync('out/index.html','utf8');\
console.log(/vendor-baseui-forms/.test(h) ? 'FAIL: forms preloaded' : 'OK: forms lazy')"
```

## Recorded limits / not done

- No brotli or zstd: stdlib-only policy; gzip already covers the payloads.
- No API JSON compression: operator payloads are small; the beacon hot path
  must stay allocation-free. Re-evaluate only with a measured win.
- No precompressed cache at boot: per-request gzip on the operator plane is
  negligible, and the dist is re-embedded on every release anyway.
- No `Last-Modified`: `embed.FS` has zero modtimes; the ETag carries identity.
- Page chunks are deliberately not preloaded: the shell carries no page code
  and a click costs one chunk fetch, which hover/focus and idle prefetch
  usually hide. Do not "fix" this by preloading routes back into the shell.
- A cold `/login` visit pays one extra round trip for the lazy login chunk; the
  401 prewarm covers the common expired-session path, not a first-ever visit.
- The entry chunk still carries the full i18n dictionaries (~33% of the entry
  chunk is the `en` dictionary); splitting them is tracked separately.
- `frontend/e2e/layout.spec.ts` win32 visual baselines were last refreshed
  2026-08-29 and are stale locally; CI enforces the Linux set, which is
  current. Re-baseline win32 separately before trusting local pixel diffs.
