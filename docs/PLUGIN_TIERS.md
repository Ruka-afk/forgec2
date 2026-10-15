# ForgeC2 plugin tiers

Plugins are executable code. ForgeC2 therefore treats them in two tiers:

| Tier | `interpreter:` | Isolation | Use for |
|------|----------------|-----------|---------|
| Native (trusted) | `python`, `python3`, `powershell`, `pwsh`, `bash`, `sh`, `go` | child process: minimal env, sanitized absolute-only `PATH`, per-run process group / Windows Job Object, 300s server timeout ceiling, CPU-time budget (default 120s, Linux/Windows), capped stdout/stderr | bundled plugins and code you authored |
| WASM (untrusted) | `wasm` | wazero runtime: **no WASI**, no filesystem/network/env, 16 MiB linear-memory cap, wall-clock timeout, capped logs and result | third-party or downloaded plugins |

A native plugin runs with the team server account's reach — env scrubbing and
process isolation are hygiene, not a sandbox. If you do not trust the code,
ship it as a WASM module.

## ABI v1 (WASM tier)

A module must export:

```wat
(memory (export "memory") ...)
(func (export "run") (param i32 i32) (result i32 i32))
```

`run(ptr, len)` receives the input JSON through its arguments: the host writes
the input past the module's initial linear memory (growing memory as needed),
so the module's own data — globals, heap, stack, wherever its toolchain placed
them — can never be overwritten by the input. Always read it through `ptr`
rather than assuming an address.

`run` returns **one `i64`** locating the JSON result anywhere in the module's
own memory: low 32 bits are the pointer, high 32 bits the length, and `0` means
"no result". The host clamps the pair to the module's memory bounds and to
2 MiB.

The pair is packed rather than returned as two values on purpose: a
`(result i32 i32)` export cannot be produced by ordinary C/Rust toolchains —
clang lowers a two-field struct return to an indirect (sret) parameter — so the
multi-value form would have excluded exactly the languages this tier targets.

Optional import (module `forgec2`):

```wat
(func $log (import "forgec2" "log") (param i32 i32))
```

Diagnostics are capped at 256 KiB per run and surface as plugin stderr.

Nothing else is importable: no WASI means no files, no sockets, no
environment, no clock beyond the host timeout.

A complete buildable example lives in
[`examples/wasm/hello-report`](../examples/wasm/hello-report) — freestanding C
(no libc), manifest, build script, and a committed `plugin.wasm` that
`internal/plugin/wasm_example_test.go` runs through the real executor on every
test run. The parts that matter:

```c
/* The reply lives in the module's data segment — which is why the host writes
   the input past the module's initial memory instead of at a fixed offset. */
static unsigned char reply[4096];

typedef unsigned long long u64;

__attribute__((export_name("run")))
u64 run(const unsigned char *in, unsigned len) {
    /* parse in[0..len) ... */
    unsigned n = 0;
    /* ... build the reply JSON into reply[0..n) ... */
    return ((u64)n << 32) | (u64)(unsigned long)reply;
}
```

Build it with any wasm32-capable clang (no sysroot needed) and export the
memory, because the ABI requires the module to export it:

```sh
clang --target=wasm32-unknown-unknown -Os -nostdlib \
      -Wl,--no-entry -Wl,--export=run -Wl,--export-memory \
      -o plugin.wasm plugin.c
```

## Package trust

`manifest.yaml` may carry:

```yaml
digest: "sha256:<hex>"     # canonical hash over every package file
signature: "<hex>"         # Ed25519 signature over that digest string
publisher: "forgec2"
includes: ["lib"]          # shared paths (relative to the scanned tree) that
                           # also belong to this package
```

The digest covers the entry point and every helper file, plus anything named in
`includes` (the bundled plugins all import `lib/`, which can read and write the
SQLite database — leaving it out would let a tampered helper satisfy every
signature). Runtime noise (`__pycache__`, `*.pyc`, `*.sig`, VCS/editor files)
is excluded, and the plugin environment sets `PYTHONDONTWRITEBYTECODE=1` so a
first run cannot invalidate its own signature.

The server verifies it on load and on install:

```bash
go run ./cmd/sign-plugin -gen                        # once, offline
PLUGIN_SIGNING_KEY=<seed> go run ./cmd/sign-plugin -root plugins -include lib plugins/recon/ad-recon
go run ./cmd/sign-plugin -verify -key <pubkey> -root plugins plugins/recon/ad-recon
```

The 52 bundled plugins ship signed by a project key whose private half was
discarded at release, so those signatures are permanent and unforgeable. The
matching public key is in `config.example.yaml`:

```yaml
plugins:
  trusted_keys: ["<bundled-or-your-pubkey-hex>"]  # or FORGEC2_PLUGIN_TRUSTED_KEYS
  require_signed: true                            # refuse unsigned packages
  max_concurrent: 4                               # simultaneous plugin processes
  max_cpu_seconds: 120                            # CPU-time budget per run (Linux/Windows)
```

`max_cpu_seconds` bounds the CPU time one native run may burn across its whole
process tree (Windows: Job Object accounting; Linux: process-group sampling of
`/proc`). The watchdog samples once a second, so a spinning plugin is killed
within about a second of exhausting the budget and the run fails with an
explicit CPU-budget error instead of a wall-clock timeout. Platforms without
CPU accounting (macOS) log once and stay bounded by the 300s timeout; the WASM
tier remains wall-clock bounded only.

Changing any bundled plugin file (or `lib/`) without re-signing makes the server
refuse to load it — that is the point, and `go test ./internal/plugin/` fails
too (`TestBundledPluginsVerifyUnderStrictPolicy`).
