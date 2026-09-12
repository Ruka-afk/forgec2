# C Implant (prototype)

Wire-compatible C implementation of the ForgeC2 implant for Windows,
built with mingw-w64. No Go toolchain, no runtime, ~64KB release binary.

## Status: working prototype, E2E verified

- v3 registration + ECDH session + AES-GCM encrypted beacons against the
  live server (`/api/v1/beacon`), seq persistence + handshake recovery,
  requeue on failure, resync fast-forward
- Tasks: shell, hostinfo/recon, file ops (upload/download/mkdir/rename/
  delete/chmod), download_url, traffic jitter camouflage, per-disguise
  LNK icons; unsupported types fail closed with `unsupported in C implant`
- Crypto: X25519 via embedded constant-time ladder (`curve25519.c`),
  verified at startup against Go-stdlib vectors (`x25519_selftest`,
  fail closed); AES-GCM/HMAC/RNG via CNG (works back to Windows 7)

## Build

Requires mingw-w64 (`x86_64-w64-mingw32-gcc`):

```bat
build.bat <C2_HOST> <C2_PORT> <SECRET_ID> <SECRET_B64>
```

Mint `SECRET_ID`/`SECRET_B64` via Generate → one-liner (each secret binds
to exactly one agent id; identity persists in `%TEMP%\fc2c.dat`).
Add `-DE2E_DEBUG` for the `C:\Windows\Temp\cbeacon-dbg.log` frame trace
(build-e2e.bat pattern). Never commit real secrets (`.exe` is gitignored).

## Size comparison (windows/amd64, -O2)

| Implant | Size | Ratio |
|---|---|---|
| Go agent (full) | ~23.3 MB | 1× |
| Go agent (slim/http-only) | ~17.0 MB | 0.7× |
| Go agent (slim + Win7/go1.20) | ~14.9 MB | 0.6× |
| **C implant (release)** | **~64 KB** | **1/360×** |

## Gaps vs the Go agent (by design, for now)

- Transports: HTTP only (no TCP/DNS/ICMP/WSS/gRPC/SSH/P2P/SMB)
- No BOF/CLR/execute-assembly, no sleep masks, no PPID spoof
- No screenshots/keylogging/persistence/sleep jitter beyond beacon interval
- No malleable request transforms (plain JSON inner bodies)
- No auto self-update; redeploy manually

## Files

- `beacon.c` — main loop, frames, task handlers, persistence
- `crypto_cng.{c,h}` — CNG AES-GCM/HMAC/RNG + X25519 glue
- `curve25519.{c,h}` + `x25519_vectors.h` — ladder + self-test vectors
  (generated from Go stdlib `crypto/ecdh`; regenerate mechanically,
  never hand-edit)
- `selftest_main.c` — standalone `x25519_selftest` runner
