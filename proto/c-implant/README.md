# C Implant (prototype)

Wire-compatible C implementation of the ForgeC2 implant for Windows,
built with mingw-w64. No Go toolchain, no runtime, ~660KB release binary
(SQLite amalgamation vendored for `wechat_history`).

## Status: working prototype, E2E verified

- v3 registration + ECDH session + AES-GCM encrypted beacons against the
  live server (`/api/v1/beacon`), seq persistence + handshake recovery,
  requeue on failure, resync fast-forward
- Tasks: shell, hostinfo/recon, file ops (upload/download/mkdir/rename/
  delete/chmod), download_url, **set_sleep**, **wechat_history**,
  system management (services, reg_get/reg_set/reg_delete, killproc,
  suspend/resume, reboot/shutdown, window_list/window_close),
  persistence (persistence_add/list/remove: registry Run, scheduled task,
  startup folder), recon extensions (**drives**, **find**, **portscan**,
  **clipboard_get/clipboard_set**, **get_sleep_mode**), traffic jitter
  camouflage, per-disguise LNK icons; unsupported types fail closed with
  `unsupported in C implant`
- Crypto: X25519 via embedded constant-time ladder (`curve25519.c`),
  verified at startup against Go-stdlib vectors (`x25519_selftest`,
  fail closed); AES-GCM/HMAC/RNG via CNG (works back to Windows 7)

## Build

Requires mingw-w64 (`x86_64-w64-mingw32-gcc`):

```bat
REM Development build (with debug, -O2)
build.bat <C2_HOST> <C2_PORT> <SECRET_ID> <SECRET_B64>

REM Release build (size-optimized, -Os + strip + gc-sections, optional UPX)
build-release.bat <C2_HOST> <C2_PORT> <SECRET_ID> <SECRET_B64>

REM Local E2E build with true secrets (gitignored, not for commit).
REM Secrets come from env vars or args, never from the file itself:
REM   set SECRET_ID=...
REM   set SECRET_B64=...
REM   build-e2e-local.bat
REM or:  build-e2e-local.bat SECRET_ID SECRET_B64
build-e2e-local.bat
```

Mint `SECRET_ID`/`SECRET_B64` via Generate → one-liner (each secret binds
to exactly one agent id; identity persists in `%TEMP%\fc2c.dat`).
Add `-DE2E_DEBUG` for the `C:\Windows\Temp\cbeacon-dbg.log` frame trace
(build-e2e.bat pattern). Never commit real secrets (`.exe` is gitignored).

## One-click E2E verification

```bat
scripts\e2e-c-implant.bat <HOST> <PORT> <USER> <PASS> <AGENT_ID> [TASK_TYPE] [COMMAND]
```

Example (shell):
```bat
scripts\e2e-c-implant.bat 127.0.0.1 8000 labtest labtest b5eb40d2-aa40-402d-b019-272452c504b3 shell "echo CBEACON-E2E-VERIFIED"
```

Example (wechat_history):
```bat
scripts\e2e-c-implant.bat 127.0.0.1 8000 labtest labtest b5eb40d2-aa40-402d-b019-272452c504b3 wechat_history all
```

This script automates the full CSRF flow: login → GET to rotate CSRF cookie → POST task → poll for result. It exits 0 only when the task reaches
`completed`; a failed task, a missing CSRF cookie, a non-2xx login or the 60s
poll timeout print the server's own error text and exit 1.

An eighth argument, or the `E2E_EXTRA_JSON` environment variable, appends raw
JSON fields to the task body for tasks that need more than `command`:

```bat
set E2E_EXTRA_JSON="path":"C:\\Windows\\Temp"
scripts\e2e-c-implant.bat 127.0.0.1 8000 labtest labtest <AGENT_ID> find *.exe
```

Those values are JSON, so every backslash must be doubled
(`C:\\Windows\\Temp`): a single `\U` is not a valid JSON escape and the
server answers `invalid request`. Prefer the environment variable over the
argument, because cmd strips quotes from command-line arguments.

### Recon task matrix

| Task | Result shape (as stored by the server) | Notes |
|---|---|---|
| `drives` | `Drive`/`Type`/`Free`/`Total` tab-separated table, sizes in GB | same table as the Go agent |
| `find` | `<path>\t<size>\t<YYYY-MM-DD HH:MM>` lines | needs `command` (the pattern) and usually `path`; the implant sends base64 and the server decodes it before storing |
| `portscan` | one `<host>:<port> open\|closed` line per port | TCP only, 2s connect timeout; `udp` fails closed, `host:port-range`/comma lists parse like the Go agent |
| `clipboard_get` | the clipboard text (UTF-8) | needs an interactive desktop; without one: `cannot open the clipboard` |
| `clipboard_set` | `clipboard set` | writes `CF_UNICODETEXT`; a locked clipboard: `clipboard set failed` |
| `get_sleep_mode` | `sleep interval N s, jitter M%` | the C implant has no adaptive sleep modes, so it reports the active interval/jitter |

Every other task type the server can dispatch keeps failing closed with
`unsupported in C implant`, so a type that lands in the C build by accident
can never silently no-op.

## Local E2E without the UI

`scripts/e2e-c-implant-local.ps1` runs the whole loop against a throwaway
server and touches no operator state:

```powershell
powershell -NoProfile -File scripts\e2e-c-implant-local.ps1
```

1. builds the server and starts it on `127.0.0.1:8100` (fresh data dir, plain
   HTTP, generated beacon key and admin password, all under one work dir)
2. mints a v3 registration secret straight into that server's `reg_secrets`
   table with `go run ./cmd/mint-reg-secret` - no operator session, no UI
3. cross-builds the implant against the secret and runs it with `TEMP`/`TMP`
   redirected, so a developer's `%TEMP%\fc2c.dat` identity is never reused
4. drives `scripts/e2e-c-implant.bat` once per task (shell, drives, find,
   clipboard_set, clipboard_get, portscan, get_sleep_mode) and asserts the
   result text

It exits 0 only when every assertion holds and keeps the work dir of a failed
run; clipboard tasks count as passed when they either complete or fail closed
with the documented clipboard error, because a service session has no
interactive desktop. Requires `go`, `x86_64-w64-mingw32-gcc` and `curl.exe`.

## CI gate

The `c-implant` job in `.github/workflows/ci.yml` cross-builds the implant with
mingw-w64 under `-Wall -Wextra -Werror` (the vendored `sqlite3.c` is compiled
separately, without `-Werror`, so its warnings cannot mask ours), asserts every
recon task string is present in the binary, `-fsyntax-only` builds the
`C2_TRANSPORT_DNS` variant, runs `x25519_selftest` as a host binary, and builds
and runs the shipped WASM example through `cmd/plugin-check`.

## Size comparison (windows/amd64)

| Implant | Size | Ratio |
|---|---|---|
| Go agent (full) | ~23.3 MB | 1× |
| Go agent (slim/http-only) | ~17.0 MB | 0.7× |
| Go agent (slim + Win7/go1.20) | ~14.9 MB | 0.6× |
| **C implant (release, -Os+strip+gc-sections)** | **~660 KB** | **1/36×** |
| C implant (release + UPX --best --lzma) | **~660 KB** | **1/36×** |

Note: The -Os+strip+gc-sections build yields ~660KB (vs ~197KB pre-SQLite).
The bulk is the vendored SQLite amalgamation (`sqlite3.c`, required for
`wechat_history`); UPX yields minimal further reduction on this binary due
to already-stripped sections and high entropy.

## Gaps vs the Go agent (by design, for now)

- Transports: HTTP only for the beacon loop (no TCP/DNS/ICMP/WSS/gRPC/SSH/
  P2P/SMB). `dns_transport.{c,h}` already carries the DNS TXT transport with
  Go-agent wire parity, but it is not wired into the loop yet and stays out
  of default builds (gated on `C2_TRANSPORT_DNS`; CI syntax-checks it)
- No BOF/CLR/execute-assembly
- Evasion (evade.c, honest scope): XOR+NOACCESS sleep mask over a 4KiB
  config shadow (C2 host/path, secret id, UA, UUID) + Halo's-Gate
  NtDelayExecution (direct syscall, Sleep fallback); PPID-spoofed shell
  spawn (explorer parent, _popen fallback); startup sandbox gate
  (tick-acceleration + 1c/<2GiB hw gate, -DSANDBOX_CHECKS=0 to strip).
  NOT full-image AES, NOT AMSI/ETW patching, NOT direct syscalls elsewhere.
- No keylogging/webcam/mic; no SOCKS/port-forwarding yet
- Streams are beacon-paced (1 frame per beacon), not realtime
- No malleable request transforms (plain JSON inner bodies)
- No auto self-update; redeploy manually
- Clipboard tasks need an interactive desktop (a service session fails closed
  with `cannot open the clipboard` / `clipboard set failed`)

## Files

- `beacon.c` — main loop, frames, task handlers, persistence
- `evade.{c,h}` — sleep mask, Halo's-Gate NtDelayExecution, PPID-spoofed
  exec, startup sandbox gate (see header for exact capability)
- `crypto_cng.{c,h}` — CNG AES-GCM/HMAC/RNG + X25519 glue
- `curve25519.{c,h}` + `x25519_vectors.h` — ladder + self-test vectors
  (generated from Go stdlib `crypto/ecdh`; regenerate mechanically,
  never hand-edit)
- `selftest_main.c` — standalone `x25519_selftest` runner
- `dns_transport.{c,h}` — DNS TXT beacon transport (Go-agent wire parity,
  gated on `C2_TRANSPORT_DNS`, not wired into the main loop yet)
- `sqlite3.c` / `sqlite3.h` — vendored SQLite amalgamation
  (`wechat_history`), built separately from the implant sources

Harnesses: `scripts\e2e-c-implant.bat` (one task against a live server),
`scripts\e2e-c-implant-local.ps1` (the whole loop, no UI needed) and
`go run ./cmd/mint-reg-secret` (a registration secret without the Generate
UI).