# File Intelligence

File Intelligence is the read-only discovery layer behind the Beacon **Files →
Media** tab. It answers "what files are on this target?" without turning the
console into a content indexer.

M1a shipped metadata search; M1b adds bounded thumbnails for image/video hits
and HTTP Range playback of staged audio/video; M1c adds an explicit,
text-only sensitive-data scan. Search never reads file contents. Thumbnails and
scans only run when the operator asks for them, and raw scan matches remain on
the encrypted operator surface.

## M1a: metadata search

```
Browser                         Server                         Go agent
  POST /agents/:id/files/search
  path, pattern, categories,
  size/date/depth filters
        └──────────────────────►
                                file_search task
                                (JSON options, ≤5,000 hits)
                                        └─────────────────────►
                                                              filepath.WalkDir
                                                              metadata only
                                        ◄─────────────────────
                                encrypted task result
  pollTask ────────────────────►
  render Media table
```

- The browser never talks to the target directly. The task is queued through
  the normal agent task channel and the result is polled like every other
  Beacon command.
- Only paths, names, extensions, categories, sizes and modification times are
  returned. **The M1a search does not read file contents**; previews are added
  only by the explicit M1b thumbnail task, and no OCR or content classification
  runs in the search path.
- The server does not walk the filesystem and does not cache results.

### API

`POST /agents/:id/files/search`

Requires `agents.write` and an operator session. The body is form-encoded
(JSON is also accepted for typed clients):

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `path` | string | — | Required search root. Absolute paths are recommended. |
| `pattern` / `command` | string | `*` | Comma-separated filename globs; a value without `*`/`?` is a case-insensitive substring match. |
| `categories` | string | all | Comma-separated: `image`, `video`, `audio`, `document`, `archive`, `key`, `config`, `other`. `all` or empty means no category filter. |
| `min_size` / `max_size` | bytes | 0 | Non-negative; 0 means no bound. |
| `modified_after` / `modified_before` | string | — | RFC3339 or `YYYY-MM-DD`. Dates without a time are interpreted in the target's local timezone by the agent. |
| `include_hidden` | `1`/`true`/`0` | false | On Windows this also honours the hidden file attribute; on Unix it skips dot-prefixed entries. |
| `max_depth` | int | 8 | 1–16; the root is depth 0. |
| `max_results` | int | 2,000 | Hard maximum 5,000. |
| `timeout_seconds` | int | 20 | Hard maximum 60. |

The handler responds with the normal task acknowledgement:
`{"success":true,"task_id":N}`. Poll `GET /agents/:id/tasks/:taskId` until the
task is `completed` or `failed`.

### Result shape

The agent returns one JSON object (never a TSV or a file path list):

```json
{
  "root": "C:\\Users\\alice",
  "patterns": ["*.jpg", "*.png"],
  "categories": ["image"],
  "hits": [
    {
      "path": "C:\\Users\\alice\\Pictures\\photo.jpg",
      "name": "photo.jpg",
      "ext": "jpg",
      "category": "image",
      "size": 184320,
      "modified": "2026-10-20T10:00:00+08:00"
    }
  ],
  "scanned": 4213,
  "errors": 2,
  "truncated": false,
  "elapsed_ms": 812
}
```

`truncated` is true with `truncated_reason` set to `max_results`, `max_bytes`,
`timeout` or `cancelled` when the agent stopped early. `errors` counts entries
whose metadata could not be read (for example a permission failure); it does
not fail the whole task.

Hits are sorted newest-first (then by path). The complete JSON document is kept
under 900 KiB so the server's 1 MiB per-result cap never turns it into invalid
JSON.

### Categories

| Category | Extensions (examples) |
|----------|-----------------------|
| `image` | jpg, jpeg, png, gif, bmp, webp, tiff, ico, svg, heic, avif, raw |
| `video` | mp4, mkv, avi, mov, wmv, flv, webm, m4v, mpg, mpeg, ts, 3gp |
| `audio` | mp3, wav, flac, aac, ogg, m4a, wma, opus, aiff, midi |
| `document` | pdf, doc(x), xls(x), ppt(x), txt, md, rtf, csv, odt/ods/odp |
| `archive` | zip, rar, 7z, tar, gz, tgz, bz2, xz, zst, iso, cab, jar, apk |
| `key` | pem, key, p12, pfx, ppk, kdbx, ovpn, asc, gpg, jks, der, crt |
| `config` | ini, conf, config, cfg, yaml/yml, json, xml, toml, env, reg, plist |
| `other` | everything else, including extensionless files |

The category is inferred from the filename only. It is a navigation aid, not a
content claim.

### Bounds and skip rules

- Default maximum depth 8, hard maximum 16.
- Default 2,000 hits, hard maximum 5,000.
- Default 20 s wall clock, hard maximum 60 s.
- The agent skips `.git`, `.svn`, `.hg`, `node_modules`, Windows system trees
  (`Windows`, `Program Files`, `ProgramData`, recovery/MSOCache), Unix
  pseudo-filesystems (`proc`, `sys`, `dev`, `run`) and hidden entries unless
  `include_hidden` is set. A skipped directory can still be selected as the
  explicit root.
- Symlinks are not followed. Only regular files are returned.
- A depth limit skips deeper directories but does not mark the result as
  truncated.

### UI (M1a)

`FilesPage` has three views:

- **Browse** - the existing directory table, previews, upload/pull and Find/Hunt.
- **Media** - category chips, glob, size/date/depth filters, hidden toggle, and
  the result view. **Show in folder** switches back to Browse at the hit's
  parent directory and selects the file.
- **Sensitive** - the M1c detector form and raw match table described below.

The Media view defaults to `image`, `video` and `audio`; clicking **All** clears
the category filter. It shows the scanned count, unreadable-path count and a
truncation banner, so an empty view is distinguishable from a search that hit a
limit.

## M1b: thumbnails and playback

M1b keeps the metadata boundary from M1a and adds two explicit content paths:
a bounded thumbnail cache and staged playback. Neither path sends a whole file
through a task result.

### Thumbnail API

`POST /agents/:id/files/thumb`

Requires `agents.write` and an operator session. Accepts a form-encoded `paths`
value (a JSON array or newline-separated list) or a JSON body
`{"paths":["..."]}`. Up to **12 unique paths** per request; each path is limited
to 4,096 characters and must not contain NUL. The handler returns the normal
task acknowledgement and queues one `file_thumb` task.

The agent returns one JSON document with a per-path result:

```json
{
  "items": [
    {
      "path": "C:\\Users\\alice\\Pictures\\photo.jpg",
      "ok": true,
      "mime": "image/jpeg",
      "width": 320,
      "height": 213,
      "source_size": 184320,
      "modified": "2026-10-20T10:00:00Z",
      "data": "<base64 JPEG>"
    }
  ]
}
```

`GET /agents/:id/files/thumb/:thumb` serves the cached JPEG. The id is the
SHA-256 of the encoded image; the server accepts only 64 lowercase hex
characters. The route is operator- and tenant-scoped, and responses carry
`Cache-Control: private, max-age=86400` plus `X-Content-Type-Options: nosniff`.

### Agent thumbnail limits

- Image formats: JPEG, PNG and GIF through the Go standard library. A single
  frame of an animated GIF is used. Images above 64 MiB or 40 megapixels are
  rejected before decoding.
- The long edge defaults to 320 px and is clamped to 64-512 px. JPEG quality is
  stepped down until the encoded image fits the byte budget.
- Default encoded-image budget is 256 KiB per item (hard 512 KiB). The complete
  task result is kept under 900 KiB, so a noisy batch cannot approach the
  server's 1 MiB result cap.
- Video thumbnails require `ffmpeg` on the target. The agent seeks to 1 s, then
  retries at 0 s, extracts exactly one JPEG frame, and applies a 12 s timeout
  and the same long-edge/size limits. A missing `ffmpeg`, an unsupported codec
  or a corrupt file produces a per-item error; other paths in the batch still
  complete.
- Files with unsupported extensions or non-regular paths produce per-item
  errors. The task itself fails only for invalid options, no usable paths or
  too many paths.

### Server cache

- Encoded JPEGs are written to
  `data/thumbs/<agent-id>/<sha256-of-jpeg>.jpg` with `0600` files under a `0700`
  directory. The agent id is resolved through `safeJoin`, so a crafted id cannot
  escape the cache root.
- The server checks the JPEG magic bytes, the 512 KiB maximum and base64
  decoding before writing. Invalid items keep their per-path error and are not
  cached.
- The task result is rewritten to metadata plus `thumb_id`. Base64 image data
  is removed before the encrypted task result is stored, so task rows do not
  become a second unbounded image store.
- Cached thumbnails expire after **7 days** through the periodic cleanup loop.
  Identical content is content-addressed and reused.

### Media grid

The Media result defaults to a **grid** and offers a **list** toggle. The grid
lazily requests thumbnails for the first **24** image/video hits in batches of
12. Only those first-page hits are previewed; documents, archives, keys and
configs keep their category icon. Failed thumbnails remain listed with the
per-item error collapsed into a non-blocking banner.

### Playback

Audio and video hits expose a **Play** button. Playback uses the existing
chunked pull path: the server asks the Beacon for 4 MiB ranges, stages the
complete file under the normal per-agent upload cache, and the browser then
plays it through `GET /agents/:id/files/exfil/:filename`. That endpoint uses
the same range-capable file server as other downloads, so seeking and browser
buffering work without a custom media server.

- Files must have a known size between 1 byte and **50 MiB**. Unknown/zero
  sizes and larger files are refused before any task is queued, rather than
  producing a corrupt partial media file.
- Staging shows bounded progress and can be cancelled. Closing the playback
  dialog does not delete the staged file; it is removed by the existing upload
  cache cleanup.
- Browser codec support still decides whether a particular container/codec can
  play. A format the browser cannot decode fails in the player; it is not
  transcoded server-side.
- Thumbnails are not frames of live capture and playback is not a webcam or
  microphone stream.

## M1c: sensitive-data scan

M1c adds `file_scan`, a bounded read-only text scan. It is deliberately not an
indexer: there is no OCR, no document parsing, no archive extraction and no
background crawl. The operator selects a root, a filename glob and an explicit
detector set.

### API

`POST /agents/:id/files/scan`

Requires `agents.write` and an operator session. The body is form-encoded or
JSON:

| Field | Type | Default | Hard maximum | Notes |
|-------|------|---------|--------------|-------|
| `path` | string | — | 4,096 chars | Required scan root; NUL is rejected. |
| `pattern` / `command` | string | `*` | — | Comma-separated filename globs, same matching rules as metadata search. |
| `rules` | string | `email,phone,bank_card` | — | Comma-separated allowlist. `all` selects every detector; unknown names are rejected. |
| `include_hidden` | `1`/`true`/`0` | false | — | Hidden files and directories are skipped unless set. |
| `max_depth` | int | 8 | 16 | Root is depth 0. |
| `max_files` | int | 200 | 1,000 | Matching regular files read. |
| `max_file_bytes` | bytes | 1 MiB | 8 MiB | Larger files are skipped, not partially scanned. |
| `max_total_bytes` | bytes | 32 MiB | 64 MiB | Stops the scan before the file that would exceed the budget. |
| `max_findings` | int | 500 | 2,000 | Unique path/rule/value findings. |
| `timeout_seconds` | int | 30 | 60 | Wall-clock cap checked during the walk and between files. |

The handler clamps limits, validates the detector list and queues one
`file_scan` task. It responds with the normal
`{"success":true,"task_id":N}` acknowledgement. Poll
`GET /agents/:id/tasks/:taskId` until the task is `completed` or `failed`.

### Detectors

| Rule | Default | Match |
|------|---------|-------|
| `email` | yes | ASCII email address with local/domain length and dot checks. |
| `phone` | yes | 7–15 digits with common `+`, spaces, dots, parentheses and hyphens. Dates, IPv4 addresses and SSN-shaped strings are rejected. |
| `bank_card` | yes | 13–19 digits (separators allowed) that pass the Luhn checksum. |
| `ssn` | opt-in | US SSN shape with area/group/serial validity checks. |
| `aws_key` | opt-in | `AKIA`/`ASIA` access-key IDs. |
| `private_key` | opt-in | `-----BEGIN … PRIVATE KEY-----` markers. The key body is not parsed. |
| `jwt` | opt-in | Three base64url JWT segments beginning with `eyJ`. |
| `password` | opt-in | Conservative `password/passwd/pwd/pass/secret/api_key/access_token/client_secret` assignment; quoted or a single token. Placeholders and variable references are rejected. |

Detectors run in a fixed specificity order so a broad phone match cannot
consume a card, SSN, key or assignment first. Findings are unique per file and
rule; repeated occurrences increment `occurrences` instead of duplicating rows.

### Agent scanning behavior

- Files are matched by the same walk, hidden/system-directory and symlink rules
  as `file_search`. Only regular files are read. Known binary/container
  extensions (images, media, archives, Office/PDF, executables, databases and
  keystores) are skipped before reading.
- Unknown files with NUL bytes or a high control-byte ratio are reported as
  skipped binary. UTF-8 and UTF-16LE/BE with a BOM are decoded; malformed UTF-8
  is scanned with replacement characters so ASCII indicators are not lost.
- The scan reads each accepted file once. It never uploads the file, stores a
  copy, extracts archives or invokes `ffmpeg`/OCR.
- The raw JSON result is kept under **900 KiB**. If findings exceed the byte
  budget, the lowest-sorted rows are dropped and `truncated_reason` becomes
  `max_bytes`; counts describe only rows still present.
- `truncated` is also set for `max_files`, `max_total_bytes`, `max_findings`,
  `timeout` and `cancelled`.

### Result shape

```json
{
  "root": "C:\\Users\\alice",
  "patterns": ["*.txt", "*.log"],
  "rules": ["email", "phone", "bank_card"],
  "files_scanned": 42,
  "files_skipped": 7,
  "skipped_binary": 6,
  "skipped_large": 1,
  "bytes_scanned": 208411,
  "findings": [
    {
      "path": "C:\\Users\\alice\\notes.txt",
      "name": "notes.txt",
      "rule": "email",
      "value": "alice@example.com",
      "line": 4,
      "occurrences": 2
    }
  ],
  "counts": {"email": 1},
  "errors": 0,
  "truncated": false,
  "elapsed_ms": 81
}
```

`value` is the raw match. `line` is 1-based. `counts` is a per-rule summary of
the returned findings. Per-path read failures increment `errors` without
failing the whole task.

### Redaction boundary

The raw values are only for the authenticated operator view. The teamserver
stores the result with the normal encrypted task-result hook and replaces every
`value` with `[redacted]` before the result reaches:

- AI tools, AI task context, timeline/query context and result analysis;
- task-completion callbacks;
- handover export and task CSV export;
- IOC extraction (the `file_scan` task type is skipped entirely, so raw matches
  are not copied into IOC/STIX data).

The redacted copy carries `auxiliary_redacted: true` plus a note. A malformed or
unparseable `file_scan` document is withheld rather than passed through, so a
truncated or hand-crafted result cannot bypass redaction. Webhook/automation
events never carried task result bodies; they continue to receive only task
metadata. Logs suppress `file_scan` error text, and audit details contain only
the root, glob, detector names and limits.

### UI (M1c)

The **Sensitive** tab exposes:

- eight detector checkboxes, with email, phone and bank card selected and marked
  **default**; the scan button is disabled when no detector is selected;
- glob, depth, file/byte/finding limits, timeout and hidden-file controls;
- a prominent warning that the table shows raw values;
- a summary line (files scanned, bytes read, skipped binary/oversized, errors),
  truncation banner, per-rule rows, line number and occurrence count;
- **Copy value** and **Show in folder**. Revealing a finding switches to Browse
  at its parent directory and selects the file.

Nothing is written to `localStorage`, logs or the clipboard automatically.

## Security boundary

- Search results contain metadata only. `file_thumb` opens target bytes and
  returns only downscaled JPEGs, never the original file. `file_scan` is the
  first feature that puts raw matched values in a task result; those values
  remain in the encrypted operator result and are redacted from AI, callbacks,
  IOC extraction and exports. Logs and audit details never contain them.
- Thumbnail task results contain base64 only on the wire; the server replaces
  it with an opaque content id before storing the encrypted result. Cached
  images live under `data/thumbs` and expire after 7 days.
- Audit details contain the root, patterns, categories, detector names, limits
  or the first thumbnail path - never file contents, image bytes or scan
  matches.
- The agent has no network listener for this feature. It only responds to the
  normal Beacon task channel.
- Server-side validation rejects negative limits, unknown detectors, NUL bytes,
  oversized roots, more than 12 thumbnail paths and paths over the length
  limit. The agent re-applies its own hard caps so a hand-issued task cannot
  bypass them.

## Compatibility

- The Go agent (`internal/payload/agent`) implements `file_search`,
  `file_thumb` and `file_scan` as core commands. They are not optional modules.
- `file_thumb` relies on target-side `ffmpeg` for video frames; image
  thumbnails and all `file_scan` detection use only the Go standard library. A
  target without `ffmpeg` still returns image thumbnails and an explicit
  per-item video error.
- The C implant does not implement these tasks yet; a C Beacon returns the
  normal unsupported-task error. The UI is usable against any Beacon, but
  results only arrive from agents that implement the command.
- `file_search`, `file_thumb` and `file_scan` are declared in `pkg/protocol` and
  mirrored to the Win7 shim, so older build toolchains see the same task type
  metadata.

## Roadmap

**M1b - thumbnails and playback is complete** with the bounded cache and staged
playback described above. Container/codec coverage follows the browser and the
target's `ffmpeg`; there is no server-side transcoding.

**M1c - sensitive data scan is complete.** `file_scan` supports the default
email/phone/bank-card detectors and five opt-in detectors, keeps raw values on
the encrypted operator surface, and redacts every auxiliary consumer described
above.

**M2 - real-time screen.** The per-frame HTTP/base64 path cannot reach 30 fps.
M2 replaces it with a WebRTC (Pion) sender using an embedded TURN relay;
signaling and input stay on authenticated WSS. See
[`REMOTE_DESKTOP.md`](REMOTE_DESKTOP.md) for the current screen contract and the
planned M2 boundary.
