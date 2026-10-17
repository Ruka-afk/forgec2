# Embedded frontend delivery (serving contract)

How `forgec2-server` serves the embedded SPA (`internal/webdist/dist`, embedded
with `//go:embed all:dist`) over HTTP: cache policy, conditional requests and
on-the-fly compression. This is the reference for the numbers quoted in
`CHANGELOG.md` and for judging future serving-layer changes.

## Where it lives

| File | Role |
| --- | --- |
| `internal/server/server_http.go` | `SetStaticFS` middleware: auth redirect for `/`+`/dashboard`, malleable-profile safety net, trailing-slash normalization, cache policy, 304 handling, compression decision |
| `internal/server/static_assets.go` | boot-time metadata (content-hash ETag, size, compressibility), ETag/`Accept-Encoding` parsing, pooled gzip writer |
| `internal/server/server_http_static_test.go` | cache/304/compression-skip matrix over a `fstest.MapFS` |
| `frontend/scripts/check-bundle.mjs` | raw bundle budget (raw bytes only; compression does not shrink what it measures) |

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

## Measurements (2026-10, local host, `curl -w '%{size_download}'`)

| File | Raw | Gzip |
| --- | --- | --- |
| `index-*.js` (entry) | 394,657 | 110,807 |
| `vendor-baseui-*.js` | 206,278 | 67,573 |
| `vendor-react-*.js` | 198,366 | 62,747 |
| `index-*.css` | 212,764 | 30,433 |
| `vendor-router-*.js` | 90,691 | 30,818 |
| `vendor-zod-*.js` | 66,393 | 17,827 |
| `vendor-icons-*.js` | 58,538 | 18,383 |
| `vendor-misc-*.js` | 56,300 | 19,881 |
| `vendor-sonner-*.js` | 33,419 | 9,565 |
| `/login` shell (HTML) | 2,257 | 880 |
| `/js/vis-network.min.js` (vendored, unhashed) | 476,620 | 128,752 |

Sum of the 8 JS chunks + 1 CSS the `/login` shell references: **1,317,406 raw →
368,034 gzip** (72% saved), plus the 880-byte shell. A revisit of the shell
revalidates to `304` with a 0-byte body — the payload moves only on version
change.

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
```

## Recorded limits / not done

- No brotli or zstd: stdlib-only policy; gzip already covers the payloads.
- No API JSON compression: operator payloads are small; the beacon hot path
  must stay allocation-free. Re-evaluate only with a measured win.
- No precompressed cache at boot: per-request gzip on the operator plane is
  negligible, and the dist is re-embedded on every release anyway.
- No `Last-Modified`: `embed.FS` has zero modtimes; the ETag carries identity.
- The entry chunk still carries the full i18n dictionaries (~33% of the entry
  chunk is the `en` dictionary); splitting them is tracked separately.
