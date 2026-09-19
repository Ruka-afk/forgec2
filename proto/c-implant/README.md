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
  startup folder), traffic jitter camouflage, per-disguise LNK icons;
  unsupported types fail closed with `unsupported in C implant`
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

This script automates the full CSRF flow: login → GET to rotate CSRF cookie → POST task → poll for result.

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

- Transports: HTTP only (no TCP/DNS/ICMP/WSS/gRPC/SSH/P2P/SMB)
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

## Files

- `beacon.c` — main loop, frames, task handlers, persistence
- `evade.{c,h}` — sleep mask, Halo's-Gate NtDelayExecution, PPID-spoofed
  exec, startup sandbox gate (see header for exact capability)
- `crypto_cng.{c,h}` — CNG AES-GCM/HMAC/RNG + X25519 glue
- `curve25519.{c,h}` + `x25519_vectors.h` — ladder + self-test vectors
  (generated from Go stdlib `crypto/ecdh`; regenerate mechanically,
  never hand-edit)
- `selftest_main.c` — standalone `x25519_selftest` runner