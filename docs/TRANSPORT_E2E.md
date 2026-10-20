# Advanced Transport E2E Checklist

Authorized lab validation only. Goal: prove **Generate → Listener → Check-in → Task** for each advanced channel.

## Common prerequisites

1. Server running (`forgec2-server.exe -config config.yaml`), `/health` → `ok`.
2. Operator logged into UI with generate + listeners permissions.
3. Implant host can reach the listener (firewall, NAT, TLS certs as needed).
4. After implant code/ldflags change: **regenerate** payload (do not rely on `set_sleep` alone).

## HTTP(S) baseline (Core)

| Step | Action | Pass? |
|------|--------|-------|
| 1 | Create HTTP listener (e.g. `:8443`) | |
| 2 | Generate page: select listener, transport HTTP(S) | |
| 3 | Build Windows EXE / Linux ELF | |
| 4 | Run implant; agent appears online | |
| 5 | Shell `whoami` / `id` returns output | |

## Multi-C2 failover (HTTP(S) baseline extension)

| Step | Action | Pass? |
|------|--------|-------|
| 1 | Create two HTTP(S) listeners, e.g. primary `:8443` and backup `:9443` | |
| 2 | Generate page: select the primary listener; set **Failover C2** to `https://backup-host:9443` | |
| 3 | Confirm the preview/sent `c2_url` is `https://primary-host:8443,https://backup-host:9443` | |
| 4 | Build a Go artifact (EXE/ELF/DLL) or token stager; the full comma-separated list is embedded | |
| 5 | Run the artifact with both listeners up; the beacon registers over the primary | |
| 6 | Stop the primary listener (keep the backup up); the next beacon fails over and returns online | |
| 7 | Restart the primary; the agent may continue on the backup or move back, and tasks still round-trip | |
| 8 | Re-register/restart the artifact; the server-delivered primary replaces the first entry but the backup list survives | |

Pass criteria:

- The generated config contains the whole list, not just the listener origin.
- A server-delivered network config updates the primary without deleting the operator's backups.
- The stager's stage-1 fetch goes to the primary origin; stage-2 keeps the full list.
- Unsupported failover schemes, credentials or query strings are rejected at generate time (400), not baked into a dead payload.
- The PowerShell one-liner is intentionally single-endpoint (its template dials one URL); use a Go artifact for failover.

## Backpressure, 5xx and invalid 200s

| Step | Action | Pass? |
|------|--------|-------|
| 1 | Temporarily lower `rate_limit.beacon` in the lab config or drive more than the default 100 requests/60 s from one IP | |
| 2 | Observe the 429 response: `Retry-After` header and JSON `retry_after` are both present | |
| 3 | Keep the agent running; it stays registered and wakes after the advertised delay instead of re-handshaking in a tight loop | |
| 4 | Return 503 (or 500) from the beacon route for a few requests; the agent backs off and recovers without dropping its ECDH session | |
| 5 | Put a proxy/captive portal in front that answers `200 text/html`; the agent treats it as a failure, keeps pending results and retries | |
| 6 | Return a malformed 200 once, then restore the real beacon; queued results arrive after recovery and are not lost | |
| 7 | Force a 400 on the register frame (e.g. clear the local state marker against an already-registered UUID); the next beacon uses the authenticated handshake path | |
| 8 | Trigger a server-side re-registration (`reregister`); the local `registered` marker is removed, so a restart re-enrolls instead of assuming a stale registration | |

Pass criteria:

- 429/5xx do not invalidate the session or force immediate fast-mode retries; the backoff floor is at least the server's `Retry-After`.
- Invalid 200 bodies do not clear `beaconConsecutiveFailures` and do not silently drop task results.
- Encrypted-frame results/acks are re-queued on transport/parse failure; auth frames requeue before transmission because they carry no task body.
- Result replay is safe: the server's ResultID idempotency cache absorbs a duplicate when the request was processed but the reply was lost.

## Online config refresh, network changes and secret TTL

| Step | Action | Pass? |
|------|--------|-------|
| 1 | Keep a Go beacon online with a long sleep (e.g. 300 s) and note its listener endpoint / sleep values | |
| 2 | Change the bound listener's host or port, or change the agent's sleep via the UI / `set_sleep`, without rebuilding | |
| 3 | On the next beacon the encrypted response carries `network_config`; the agent applies the new endpoint/sleep and persists it | |
| 4 | Leave the config unchanged for several beacons: no `network_config` is attached; after 15 minutes it is resent as a self-heal | |
| 5 | Restart the agent: it starts from the persisted applied config, not the original build-time values | |
| 6 | Toggle the network (Wi-Fi/VPN unplug/replug); the agent wakes within ~20 s instead of sleeping out the interval and reconnects | |
| 7 | Lower `rate_limit.beacon`, exceed it, and scrape `/metrics`: `forgec2_beacon_rate_limited_total` increases while the 429 keeps `Retry-After` | |
| 8 | Mint a secret and leave it unused past `server.reg_secret_orphan_ttl_hours` (age the row to avoid waiting): registration is refused before the hourly sweep runs | |
| 9 | Use a secret that is already bound to a deployed agent after any TTL: it still authenticates | |

Pass criteria:

- Config delivery is change-triggered and periodic (15 min); an unchanged config is not retransmitted on every beacon.
- Datagram transports (UDP/ICMP/DNS) skip a blob that cannot fit one response frame; the next fitting beacon delivers it.
- Re-applying the identical config is a no-op — no persisted-blob rewrite and no network reparse.
- Both the regular sleep and the JIT-scheduler sleep are interrupted by a network change; empty/transient interface enumeration never wakes the agent.
- Expired unbound secrets are refused at authentication, not only deleted later; bound secrets never expire; the orphan sweep runs at startup and hourly.

## WSS (Hardened / Experimental)

| Step | Action | Pass? |
|------|--------|-------|
| 1 | Listener that serves WebSocket beacon path (same host as C2 HTTP or dedicated) | |
| 2 | Generate: **Beacon Transport = wss**, C2 URL `wss://host:port/...` or scheme from listener | |
| 3 | Ensure TLS cert trusted or `skip_tls` for lab only | |
| 4 | Implant check-in over WSS | |
| 5 | One interactive shell command | |

Notes: implant uses `SkipTLSVerify` when skip_tls is set. Production must use real certs.

## gRPC (Experimental)

| Step | Action | Pass? |
|------|--------|-------|
| 1 | Enable gRPC listener (`grpc_enabled` + `grpc_addr`) | |
| 2 | Generate: **Beacon Transport = grpc** | |
| 3a | Lab: C2 URL `grpc://host:port` (plaintext / insecure credentials) | |
| 3b | Prefer TLS: enable `server.tls_enabled` + cert/key; C2 URL `grpcs://host:port` | |
| 4 | Implant check-in | |
| 5 | Task round-trip | |

Notes:
- Implant uses **TLS when URL is `grpcs://`**, otherwise insecure `grpc://` (lab only).
- Server enables gRPC TLS when `tls_enabled` and cert/key load successfully.
- `SkipTLSVerify` applies to grpcs self-signed certs (same as HTTPS C2).
- Do not expose insecure gRPC on real networks.

## SSH transport (Experimental)

| Step | Action | Pass? |
|------|--------|-------|
| 1 | Server SSH transport listener (`ssh_enabled`, host key path, user) | |
| 2 | Generate: **Beacon Transport = ssh**, fill SSH user/password or base64 client key | |
| 3 | Optional: pin host key (`ssh_host_key` form field or auto from `server.ssh_host_key` private key file) | |
| 4 | Implant connects via SSH tunnel to C2 | |
| 5 | Task round-trip | |

Notes:
- When **SSHHostKeyStr** is set at generate time (auto from server host key file if present), implant **pins** the server host key.
- When pin is empty, implant falls back to `InsecureIgnoreHostKey` — **lab only**.

## DNS (Hardened)

| Step | Action | Pass? |
|------|--------|-------|
| 1 | DNS page: set domain/addr, Start | |
| 2 | Generate: protocol/transport DNS, optional DoH/DoT fields | |
| 3 | Implant check-in via DNS (or DoH fallback) | |
| 4 | Task round-trip | |

## Sign-off

| Transport | Date | Operator | Result | Notes |
|-----------|------|----------|--------|-------|
| HTTP(S) | | | | |
| WSS | | | | |
| gRPC | | | | |
| SSH | | | | |
| DNS | | | | |

Related: `docs/CAPABILITY_MATRIX.md`, Generate Shared Settings, Listeners page.

## API smoke (no implant required)

```powershell
powershell -File scripts/api-smoke.ps1 -TryDefaultAdmin
```

Expect health/ready/login SPA + unauthenticated 401 on `/api/*`, then (if login works) agents/modules/listeners/attack coverage + CSRF gate.

## Automated Go-agent E2E (no UI required)

The primary implant has a runtime gate that walks the whole operator path against
a throwaway server - no UI, no manual steps, nothing written outside `-WorkDir`:

```powershell
# Windows: real EXE
powershell -NoProfile -File scripts\e2e-agent-local.ps1
# Linux (also the CI variant): real ELF
pwsh -NoProfile -File scripts/e2e-agent-local.ps1 -WorkDir "$env:RUNNER_TEMP/e2e-agent" -KeepWork
```

What it proves, in order: throwaway server boot + `/health`; login with CSRF
cookie; `POST /api/listeners` (HTTP) reaches `running`; `POST /generate/{linux,exe}`
queues a real build through the job queue; the poll returns `completed` and the
download **is an executable** (magic checked: `MZ` / ELF); the agent registers
over v3, comes online, and its hostname/os/arch/pid/listener are backfilled on
the beacon after the registration frame; then a four-task matrix round-trips -
`shell` (random echo marker), `read` (file content stored as plain text after the
server decodes the wire base64), `find` (`name<TAB>size<TAB>YYYY-MM-DD HH:MM`
for a script-written fixture) and `get_sleep_mode` (`default`). The script also
locks an API contract: `POST /api/v1/tasks` for `read` without `command` answers
`400 task type read requires 'command' parameter` (not the opaque 500 this gate
originally hit).

Two operator-path traps this harness had to encode; they apply to any scripted
integration, not just this one:

1. **The server process must run with its CWD inside the checkout.** The payload
   generator injects its module `replace` directive by walking up from the
   current working directory to find `go.mod`; started elsewhere, every build
   fails and the API only reports a sanitized "Build failed" (the real reason is
   in the server log).
2. **Artifact downloads must ask for JSON.** The SPA middleware
   (`internal/server/server_http.go`) answers any non-API GET that does not carry
   `Accept: application/json`, `X-Requested-With: XMLHttpRequest` or a CSRF
   header with `index.html` - a download request without the header silently
   saves the SPA shell (2 KB "implant") instead of the binary.

`shell` is asserted ops-profile-aware: on a host the agent classifies as a
sandbox (VM heuristics in `env_windows.go`), the ops profile refuses shell by
design, so the gate asserts that refusal - including that no command output
leaks - instead of the marker. Linux probes keep the permissive profile and
assert the marker.

On failure the script keeps the DB, config, binaries and logs and tails the
server log; a fully green run deletes them, because they carry the
server-minted registration secret.
