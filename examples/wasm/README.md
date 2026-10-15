# WASM plugin examples

WASM plugins run in the sandboxed tier: no WASI, so a module cannot reach
files, sockets, environment variables or clocks; linear memory is capped at
16 MiB; runs are bounded by the manifest timeout; logs and results are capped.
The contract is ABI v1 — see [`docs/PLUGIN_TIERS.md`](../../docs/PLUGIN_TIERS.md).

## hello-report

A `report` plugin written in **freestanding C** — no libc, no WASI, one
optional host import — that reads `params.label` from the input JSON and
answers with a markdown report echoing it.

| File | What it is |
| --- | --- |
| `plugin.c` | the whole ABI in ~150 lines: `run(ptr,len) -> i64` (low 32 bits pointer, high 32 bits length), reading the input through the pointer the host passes, writing the reply into module-private data, the optional `forgec2.log` import |
| `build.sh` | how to build it (`clang`, wasi-sdk or `zig cc`) |
| `plugin.wasm` | the committed build (`SHA256SUMS`, built with zig 0.17.0 / LLVM 21) |
| `manifest.yaml` | `interpreter: wasm` requires an entry ending in `.wasm` |

The C source keeps state in `static` arrays on purpose: that data lands in the
module's data segment — address 1024 for wasm-ld's default layout — which is
exactly the region the host must never use for the input. The host writes the
input *past* the module's initial linear memory instead, and the ABI hands the
module the pointer to read it from.

### Try it

```sh
# 1. rebuild (optional; a prebuilt plugin.wasm is committed)
./build.sh
# with a toolchain that is not plain clang:
CC="zig cc" TARGET=wasm32-freestanding sh build.sh

# 2. install: copy the directory into the plugin directory the server scans
#    (config `plugins.dir`, default: ./plugins), or drop it in and reload.

# 3. run it: "Generate report" in the UI for `wasm-hello` with a `label`
#    parameter, or POST /api/v1/plugins/wasm-hello/report
```

Packages that are not signed are refused when `plugins.require_signed` is on
(the shipped `config.example.yaml` enables it). Either sign this example with
your own key:

```sh
PLUGIN_SIGNING_KEY=<64-hex-seed> go run ./cmd/sign-plugin examples/wasm/hello-report
```

or set `plugins.require_signed: false` for a lab.

### Why this example exists

`internal/plugin/wasm_example_test.go` loads this directory's **real manifest**,
runs the committed `plugin.wasm` through the real executor and parses the
report it produced, checking that the label travelled from `params.label` into
the title, the (base64) body and the `forgec2.log` diagnostics. The example is
therefore covered by `go test ./internal/plugin/...` and cannot rot silently
when the host or the ABI changes.
