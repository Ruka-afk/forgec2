# Agent modules

The Go agent's optional command surface is split into **modules** that the
payload generator can compile in or out at build time. Excluding a module
removes its commands from the build entirely — the operator cannot queue them
by accident, and an agent that still receives one answers with a named,
honest error instead of `unknown task type`. Everything on this page is
enforced by code and tests, not by convention:

| where | what |
| --- | --- |
| [`internal/payload/agent_modules.go`](../internal/payload/agent_modules.go) | the catalogue (17 modules + the always-on core), and the selection normalizer |
| [`internal/payload/agent/task_registry_<module>.go`](../internal/payload/agent) | one registry file per module — the commands it owns |
| [`internal/payload/agent/task_modules.go`](../internal/payload/agent/task_modules.go) | the runtime bookkeeping (`agentModulesSummary`, `excludedModuleFor`) |
| [`internal/payload/agent/modules_default.go`](../internal/payload/agent/modules_default.go) | hand-built agents: nothing excluded |
| [`internal/payload/generator_modules.go`](../internal/payload/generator_modules.go) | `resolveAgentPlan` — what a build drops and injects |
| [`internal/payload/agent_modules_test.go`](../internal/payload/agent_modules_test.go) | catalogue integrity plus the compile gate over every module-off combination |

## What a module is

A module entry carries:

- **tasks** — the command types it owns (shown in `help`, counted in the docs).
- **registry files** — always exactly one `task_registry_<module>.go` that
  assigns those handlers in `init()`. The generator drops the whole file, so
  every entry point disappears and the linker can dead-code the handlers that
  live in shared files.
- **exclusive files** — further sources the module fully owns. They are
  dropped only when nothing outside the module (and no always-compiled file)
  references their symbols; the compile gate below is what decides that.

`core` is not an entry: it is the always-compiled remainder (shell, file ops,
config, timing, transports, session control — `task_registry.go` plus
`task_registry_v2.go`). Selecting `core` means "no optional module".

## The catalogue

| module | commands | what it covers |
| --- | --- | --- |
| `transports_advanced` | (transport) | gRPC, QUIC and WSS beacon transports — see below |
| `screen` | 9 | screenshots, streamed screen, on-screen triggers, remote input, window control |
| `collection` | 11 | clipboard, keylogger, webcam/mic capture, browser/WeChat history |
| `creds` | 11 | LSASS/katz-style dumps, DPAPI, LSA bypass, cert store, password spray, VPN/WiFi secrets |
| `ad` | 32 | ADCS ESC1-8, LDAP recon, DCSync, Kerberos tickets and delegation abuse, SharpHound |
| `lateral` | 20 | WMI/WinRM/PsExec/DCOM/SCF execution, SSH/SCP, SMB scan, coercion, NTLM relay |
| `injection` | 17 | process injection, reflective DLL/PE loading, BOF, .NET assembly execution, migration |
| `evasion` | 29 | AMSI/ETW/HWBP bypasses, unhooking, sleep masks, ghost protocol, wipes, EDR pack |
| `tokens` | 7 | token listing/stealing/creating, impersonation, named-pipe impersonation |
| `privesc` | 9 | privesc checks, UAC bypasses, potato-style SYSTEM escalation |
| `socks_relay` | 7 | SOCKS5 pivoting, port forwarding, tunnel routes |
| `persistence` | 3 | add/list/remove persistence (registry, scheduled task, startup) |
| `tun` | 2 | start/stop the `fc2tun0` tunnel device |
| `cloud` | 3 | cloud token theft (AWS/Azure/GCP), Entra PRT extraction, SCCM recon |
| `container` | 4 | container detection and escape helpers (Docker/K8s) |
| `usb` | 2 | USB device enumeration and payload dropping (autorun) |
| `p2p` | 1 | gossip-based peer discovery |

`transports_advanced` is the special one: it owns no commands. Excluding it
is the legacy **slim / light build** — the build strips the `utls` dependency,
splices transport stubs in, and trims `go.mod`; `slim` is now exactly the
"`transports_advanced` off" preset, and both flags agree (pinned by an
equivalence test).

## Selecting modules

**API** — every Go-agent build form (`/generate/exe`, `/generate/dll`,
`/generate/linux`, `/generate/macos`) accepts a `modules` field: a
comma-separated include list. Empty or absent means **every module**. Unknown
names answer `400` before any build work starts. `core` is accepted ("no
optional module").

**Generator (Go)** — `payload.ImplantConfig.Modules` takes the same list;
`resolveAgentPlan` normalizes it (order, duplicates, `core`, slim interplay)
and everything downstream — source extraction, go.mod trimming, the injected
`modules_excluded.go` — derives from that single plan.

**UI** — the Generate page's exe/dll/linux/macos panels carry an *Agent
modules* section fed by `GET /api/generate/modules` (the catalogue, so the UI
cannot drift from the generator). All boxes ticked means the empty "all"
sentinel; the advanced-transport box shows as locked while *Light build* is
on, because the server forces it out.

## What exclusion actually saves

Measured locally (windows/amd64, the repo's pinned Go toolchain, UPX and
obfuscation off, one module off per row):

| build | size | vs. full |
| --- | --- | --- |
| all modules | 31.9 MB | — |
| `core` only | 19.8 MB | −12.1 MB |
| no `transports_advanced` (= slim) | 22.0 MB | −9.9 MB |
| no `collection` | 31.5 MB | −414 KB |
| no `lateral` | 31.6 MB | −319 KB |
| no `screen` | 31.7 MB | −255 KB |
| no `cloud` | 31.7 MB | −235 KB |
| no `injection` | 31.7 MB | −223 KB |
| no `ad` | 31.8 MB | −138 KB |
| no `evasion` | 31.8 MB | −125 KB |
| no `creds` | 31.8 MB | −90 KB |
| no `socks_relay` | 31.8 MB | −82 KB |
| no `privesc` | 31.9 MB | −44 KB |
| no `persistence` | 31.9 MB | −37 KB |
| no `tokens` | 31.9 MB | −19 KB |
| no `usb` | 31.9 MB | −17 KB |
| no `tun` / `container` / `p2p` | 31.9 MB | ≤ 3 KB |

The honest reading of this table: the transport module and the core set carry
the weight that a build can actually shed; the small modules mostly lose
their **command surface** (registry + dedicated files where compile-safe),
not megabytes, because shared files — `agent_windows.go`'s injection-method
dispatch, `task_credentials.go`, `agent_beacon.go`'s p2p hooks — still
reference their implementation symbols, so the linker keeps what is reachable.
That direction (splitting the mixed files so more of each module's code is
exclusive) is deliberate future work; the compile gate is what tells the truth
about which files can be dropped today.

## What the operator/agent sees

- An excluded command still queued (old task, scripted operator, wrong
  profile) fails with:
  `task type "X" is not compiled into this build: agent module "Y" was excluded at generation time`.
- `help` counts the exclusions in its header (`N commands available (M not
  compiled into this build; agent modules: core, …)`) and marks each excluded
  command with `[not in this build: module "Y"]`.
- `hostinfo` reports the compiled set in its `modules` field (`all modules
  (hand-built agent)`, `core only`, or `core, screen, …`).

## Verifying a change

Unit level:

```sh
go test ./internal/payload/ -run 'TestAgentModule' -count=1
```

Compile gate — a real `go build` for every module-off combination against a
staged copy of the agent tree (windows/amd64 by default):

```sh
FORGEC2_AGENT_MODULE_BUILD_TEST=1 go test ./internal/payload/ -run TestAgentModuleExclusionBuilds -count=1 -v
```

Knobs: `FORGEC2_AGENT_MODULE_BUILD_TARGETS` (comma list,
e.g. `linux/amd64,darwin/arm64`) and `FORGEC2_AGENT_MODULE_BUILD_CASES`
(comma list of case names). The gate is opt-in because it invokes the Go
toolchain; CI runs a reduced matrix, and the full matrix is the local check
before changing `ExclusiveFiles`.

End to end — the standard agent E2E takes a module list and, when a module is
excluded, also asserts the honest error and the help markers:

```sh
powershell -NoProfile -File scripts\e2e-agent-local.ps1 -Modules core
```

## Adding or changing a module

1. Add (or move) the command handlers' registry file:
   `internal/payload/agent/task_registry_<module>.go` with `init()`
   assignments only.
2. Add the catalogue entry in `agent_modules.go` (name, title, description,
   tasks, registry file). `TestAgentModuleCatalogMatchesRegistry` fails if a
   task is not actually registered by the named file, or vice versa.
3. Optional: list exclusive files — only those whose symbols nothing outside
   the module references. When in doubt, list nothing and let the linker
   dead-code: the gate below turns a wrong claim into a red build instead of a
   broken payload.
4. Run the Go unit tests, then the compile gate (all cases, all platforms you
   ship). Extend `docs/AGENT_MODULES.md` (catalogue row) and CHANGELOG.

Excluded-module behaviour must stay honest: never "fix" an excluded command
by adding a fallback stub that pretends the module is there — the error text
is the contract.
