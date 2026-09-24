# ForgeC2 plugin tiers

Plugins are executable code. ForgeC2 therefore treats them in two tiers:

| Tier | `interpreter:` | Isolation | Use for |
|------|----------------|-----------|---------|
| Native (trusted) | `python`, `python3`, `powershell`, `pwsh`, `bash`, `sh`, `go` | child process: minimal env, sanitized absolute-only `PATH`, per-run process group / Windows Job Object, 300s server timeout ceiling, capped stdout/stderr | bundled plugins and code you authored |
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

`run(ptr, len)` receives the input JSON, which the host wrote at address
`1024` (linear memory is grown as needed). It returns `(ptr, len)` locating
the JSON result anywhere in the module's own memory. The host clamps both to
the module's memory bounds and to 2 MiB; `(0, 0)` means "no result".

Optional import (module `forgec2`):

```wat
(func $log (import "forgec2" "log") (param i32 i32))
```

Diagnostics are capped at 256 KiB per run and surface as plugin stderr.

Nothing else is importable: no WASI means no files, no sockets, no
environment, no clock beyond the host timeout. Implementation skeleton (C):

```c
__attribute__((export_name("memory"))) static unsigned char mem[576*1024];

__attribute__((export_name("run")))
int run(const unsigned char *in, int len) {
    /* parse in[0..len), build a JSON reply in mem[524288..] */
    const char *reply = "{\"success\":true,\"output\":\"hello\"}";
    int n = 0; while (reply[n]) n++;
    for (int i = 0; i < n; i++) mem[524288 + i] = (unsigned char)reply[i];
    return (524288, n); /* multi-value return: see wasm-ld specifics */
}
```

## Package trust

`manifest.yaml` may carry:

```yaml
digest: "sha256:<hex>"     # canonical hash over every package file
signature: "<hex>"         # Ed25519 signature over that digest string
publisher: "acme-labs"
```

The digest covers the entry point and every helper file (the manifest itself
and `*.sig` sidecars are excluded), so editing a library invalidates the
package. The server verifies it on load and on install:

```bash
go run ./cmd/sign-plugin -gen                        # once, offline
PLUGIN_SIGNING_KEY=<seed> go run ./cmd/sign-plugin plugins/recon/hostinfo
go run ./cmd/sign-plugin -verify -key <pubkey> plugins/recon/hostinfo
```

```yaml
plugins:
  trusted_keys: ["<64-hex-pubkey>"]   # or FORGEC2_PLUGIN_TRUSTED_KEYS
  require_signed: true                # refuse unsigned packages
  max_concurrent: 4                   # simultaneous plugin processes
```

With `trusted_keys` set, a signed package is verified against them; a
digest-only package is accepted with a warning (pin it out of band). Set
`require_signed: true` to refuse unsigned packages entirely. The bundled
plugins ship unsigned, so enabling strict mode means signing them first.
