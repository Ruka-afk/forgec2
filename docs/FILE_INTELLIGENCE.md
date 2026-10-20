# File Intelligence

File Intelligence is the read-only discovery layer behind the Beacon **Files →
Media** tab. It answers "what files are on this target?" without turning the
console into a content indexer.

M1a (this batch) ships **metadata search**. Thumbnails and sensitive-data scans
are separate follow-on batches so the wire format, caps and security boundary
are settled before any file bytes are opened.

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
  returned. **File contents are not read**, previews are not generated, and no
  OCR or content classification runs in this batch.
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

### UI

`FilesPage` has two views:

- **Browse** — the existing directory table, previews, upload/pull and Find/Hunt.
- **Media** — category chips, glob, size/date/depth filters, hidden toggle, and
  a result table. **Show in folder** switches back to Browse at the hit's parent
  directory and selects the file.

The Media view defaults to `image`, `video` and `audio`; clicking **All** clears
the category filter. It shows the scanned count, unreadable-path count and a
truncation banner, so an empty table is distinguishable from a search that hit
a limit.

## Security boundary

- Search results contain metadata only. Sensitive-data scans in M1c will be the
  first feature that puts raw values in a result; those values will remain in
  the encrypted task result and must not enter logs, audit details, AI tools,
  callbacks or webhooks.
- Audit details contain the root, patterns, categories and limits — never file
  contents.
- The agent has no network listener for this feature. It only responds to the
  normal Beacon task channel.
- Server-side validation rejects negative sizes, inverted size/date ranges,
  unknown categories, NUL bytes and oversized roots. The agent re-applies its
  own hard caps so a hand-issued task cannot bypass them.

## Compatibility

- The Go agent (`internal/payload/agent`) implements `file_search` as a core
  command. It is not an optional module.
- The C implant does not implement this task yet; a C Beacon returns the normal
  unsupported-task error. The UI is usable against any Beacon, but results only
  arrive from agents that implement the command.
- `file_search` is declared in `pkg/protocol` and mirrored to the Win7 shim, so
  older build toolchains see the same task type metadata.

## Roadmap

**M1b — thumbnails and playback.** Add a `file_thumb` task and a bounded
`data/thumbs` cache, a media grid, and HTTP Range playback for video/audio.
Thumbnail extraction will be size/duration bounded and disabled by policy where
required.

**M1c — sensitive data scan.** Add `file_scan` for explicit opt-in rules
(email, phone and bank card enabled by default; other detectors opt-in). Raw
findings stay unmasked only in the encrypted task result; the server stores
them exactly as it stores other encrypted results, and every auxiliary surface
(logs, audit values, AI tools, callbacks, webhooks) receives redacted summaries.

**M2 — real-time screen.** The per-frame HTTP/base64 path cannot reach 30 fps.
M2 replaces it with a WebRTC (Pion) sender using an embedded TURN relay;
signaling and input stay on authenticated WSS. See
[`REMOTE_DESKTOP.md`](REMOTE_DESKTOP.md) for the current screen contract and the
planned M2 boundary.
