# Capacity and hot-path performance

How many implants one teamserver core can carry, how that was measured, and how
to re-measure it on another machine. Everything here is read-only or opt-in:
the profiling endpoint is off by default, and none of it changes what the
server serves.

This document exists because "it feels fast" is not a capacity plan. The
numbers below are from one machine, so treat them as ratios and as a
regression baseline, not as a promise about yours.

## 1. The beacon check-in is the hot path

Every implant walks the same path every few seconds to every few minutes,
depending on its sleep interval: encrypted frame in, per-agent key derivation,
agent upsert, pending-task claim, encrypted frame out. Operator traffic is
bursty; beacon traffic is the steady-state load that sizes the box.

Three benchmarks in `internal/server/beacon_hotpath_bench_test.go` cover its
shapes, all through the real handler and router over an in-memory SQLite
database:

| Benchmark | Shape |
|---|---|
| `BenchmarkBeaconCheckin` | Idle check-in: no tasks waiting, nothing to report. The steady state of a quiet fleet. |
| `BenchmarkBeaconTaskDelivery` | Check-in that claims one pending task (queue selection, claim, task in the response). |
| `BenchmarkBeaconCheckinWithResult` | Check-in carrying one shell result (status transition, output storage, the audit/event rows it drives). |

Each iteration includes the request and response crypto (AEAD with the
per-agent session key), because dropping it would hide the cost that dominates
a small payload.

## 2. Measured numbers

Reference machine: Intel Core Ultra 5 225H, Windows 11, Go 1.26.0, 2026-10.
Command:

```bash
go test ./internal/server/ -run '^$' -bench 'BenchmarkBeacon' -benchtime=2s -count=2
```

Two runs per benchmark (run-to-run spread was about ±7%):

| Benchmark | ns/op | check-ins/s per core | allocs/op | B/op |
|---|---|---|---|---|
| `BenchmarkBeaconCheckin` | 535k – 551k | ≈ 1 840 | 1 257 | 158 KB |
| `BenchmarkBeaconTaskDelivery` | 716k – 735k | ≈ 1 380 | 1 500 | 206 KB |
| `BenchmarkBeaconCheckinWithResult` | 939k – 1 048k | ≈ 1 010 | 2 104 | 257 KB |

## 3. Deriving fleet capacity

A fleet of *N* implants on a *T*-second base interval produces `N/T` check-ins
per second. On the reference machine that costs, per core:

- idle check-ins: `N/T ÷ 1840` cores
- results on every check-in: `N/T ÷ 1010` cores

Worked examples on one core at a 30 s interval:

| Fleet | Idle | One result per check-in |
|---|---|---|
| 1 000 implants | 1.8% | 3.3% |
| 10 000 implants | 18% | 33% |
| 55 000 implants | ~100% | — |
| 30 000 implants | — | ~100% |

So a single modern core saturates somewhere around 55 000 idle implants on a
30 s interval, or 30 000 if every check-in carries a shell result — before
counting the database's own fsync cost, operator/UI polling, task fan-out and
plugin hooks, all of which share that core. Budget for headroom: the disk is
the usual bottleneck long before the CPU is.

Scaling out is by sharding fleets across teamservers (listeners are already
per-listener addressable), not by adding cores to one process: the SQLite
write path is deliberately single-writer, and `database.pool` stays 1 for it.

## 4. Profiling a running server (opt-in, loopback only)

Set a loopback address and restart:

```yaml
server:
  pprof_addr: "127.0.0.1:6060"   # empty (default) keeps profiling off
```

Then, from that host:

```bash
curl -s -o cpu.out  'http://127.0.0.1:6060/debug/pprof/profile?seconds=30'
curl -s -o heap.out 'http://127.0.0.1:6060/debug/pprof/heap'
go tool pprof -top -cum cpu.out
go tool pprof -top -cum heap.out
```

Properties, all enforced by tests in `internal/server/server_pprof_test.go`:

- **Off by default; loopback only.** A non-loopback `pprof_addr` is refused at
  config load (`config.Validate`) and again when the listener is bound, and the
  server refuses to start rather than serving profiles on a routable address.
  The endpoints have no authentication, so this is a fail-closed rule, not a
  warning.
- **Only the profiler is reachable.** The listener serves a dedicated mux with
  `/debug/pprof/*` and nothing else: the operator API, the SPA and the beacon
  routes are not exposed on that socket (`/api/v1/...`, `/health`, `/` answer 404).
- **Static setting.** `server.pprof_addr` is classified as a static reload
  token in the config status matrix, so a live edit reports "restart required"
  instead of silently doing nothing.
- **Bound at startup, closed at shutdown.** A port conflict fails the boot
  synchronously, and `Shutdown` closes the listener.

The `-cpuprofile` flag of the benchmark above is the other half of the loop:
profile the isolated path, then confirm on the running server.

## 5. SQL plan regressions

`internal/db/hot_path_index_test.go` measures `EXPLAIN QUERY PLAN` for every
query shape an index migration claims to fix: the pre-index plan must contain a
known-bad fragment (a full scan or a temp b-tree sort), and after the migration
the plan must use the index and stop sorting. An index nobody's query uses
fails the build.

The harness now supports one migration per probe (`hotPathProbe.migration`):
adding an index to an already-applied migration would leave every existing
database without it, because the runner records migrations as applied.

Current probes and where the queries come from:

- `idx_tasks_status_updated`, `idx_tasks_created_by_created`, `idx_alerts_tenant_created`,
  `idx_notifications_severity_created`, `idx_token_entries_agent_created`,
  `idx_build_logs_platform_created`, `idx_build_logs_status_created`,
  `idx_chat_messages_channel_created`, `idx_user_sessions_user_created`,
  `idx_api_keys_user_created` (batch of 2026-10-15)
- `idx_implants_listener_seen` (2026-10-16): the listener detail page lists an
  agent page as `WHERE listener_id = ? ORDER BY last_seen DESC LIMIT 5000`, and
  the active-agent counters reuse the predicate with a last_seen window. The
  single-column `idx_implants_listener_id` found the rows but the sort went to
  a temp b-tree (the LIMIT could not stop early), and the counters read
  `last_seen` off every row. The composite makes the page an ordered index walk
  and the counter an index-only range.

Tables are declared per probe (`creates`) with a baseline of the indexes a
production database already has (`pre`), because `AutoMigrate` alone only
creates the `gorm:"index"` tag ones. A probe whose "before" fragment no longer
appears fails the test: the query shape changed, and the index must be
re-examined rather than kept on faith.

## 6. Measured and rejected

Recorded so a future batch does not re-try it from the profile alone.

GORM's `PrepareStmt` (one prepared statement per distinct SQL text) looked like
the obvious answer to the CPU profile, which puts SQLite's parser
(`_yy_reduce`) at roughly a quarter of the check-in path. Measured A/B on the
three benchmarks above (2 runs each, same machine):

| Benchmark | without cache | with `PrepareStmt` |
|---|---|---|
| `BenchmarkBeaconCheckin` | 543k ns/op | 594k ns/op |
| `BenchmarkBeaconTaskDelivery` | 725k ns/op | 790k ns/op |
| `BenchmarkBeaconCheckinWithResult` | 993k ns/op | 1 005k ns/op |

It is 1–9% *slower* and allocates more (GORM's cache bookkeeping costs more
than parsing these short statements). Not shipped. The remaining parse cost is
structural — roughly half of `processBeacon` is `processAgentRegistration`,
which upserts the agent inside a transaction on every check-in — so any real
win there is a behaviour change to the registration path, not a config flag.

## 7. What these numbers do not include

- **Disk.** The benchmarks use in-memory SQLite; a file-backed database adds
  WAL fsync per write transaction. Expect materially lower throughput for
  result-heavy fleets on spinning or network storage.
- **Concurrency.** The benchmarks are single-goroutine: no lock contention, no
  GOMAXPROCS effects, no SQLite busy-retry pacing.
- **Network.** `httptest` records the response in memory; kernel time, TLS
  handshakes and per-connection overhead are outside the measurement.
- **The rest of the server.** Operator UI polling, plugin hooks, event fan-out,
  AI context builders and cleanup loops all share the same core.
