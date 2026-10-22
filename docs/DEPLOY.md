# ForgeC2 Deploy (simplest path)

Prebuilt `forgec2-server` binaries for Linux and Windows are published on every
`v*` tag via `.github/workflows/release.yml` (frontend already embedded —
single file, no Node.js needed at runtime).

## Option 0 — from a source checkout: one command

The deploy script takes the working tree to a running, verified server:

```
preflight -> build (frontend bundle + Go binary) -> stop the old instance
-> start the new one -> health/ready/login smoke -> rollback
```

```bash
bash scripts/deploy.sh                 # Linux / macOS
powershell -File scripts/deploy.ps1    # Windows
```

What it adds over "build and restart by hand":

- **Preflight** — the toolchain and the config must exist; the probe port is
  read from `server.port`; a wildcard bind without `server.allowed_origins`
  is called out (remote browsers would be refused as cross-origin).
- **One build path** — `scripts/build-embedded.ps1` owns the
  frontend → `internal/webdist/dist` sync (staged copy, LF-normalized, so the
  committed bundle stays byte-identical on every platform). The deploy script
  never copies the bundle itself.
- **A clean handover** — the old instance is stopped and the port is waited
  out, so the new one never fights for the bind.
- **Verification** — `/health`, `/ready` and `/login` are probed; a failed
  `/health` is not silently ignored.
- **Rollback** — the previous binary is kept as `forgec2-server.prev` (or
  `forgec2-server.exe.prev`). A new instance that does not come up healthy
  is rolled back to it and started again: a bad build never leaves the box
  down.

Useful flags: `--skip-checks` (skip the frontend gates), `--skip-frontend`
(backend-only change), `--no-start` (build only), `--no-rollback`,
`--port`, `--timeout`.

## Option A — Linux, Docker (recommended, SQLite, zero-config)

```bash
cp config.example.yaml config.yaml
docker compose up -d --build
curl -k https://127.0.0.1:8000/health   # {"status":"ok",...}
```

Secrets (JWT, beacon key, TLS cert) auto-generate on first run. Missing storage
keys (`crypto.loot_key` / `extc2_key` / `backup_key` / `totp_key` / `csrf_key`)
are generated at startup and written back to `config.yaml`, so the example
config is a working first start; set them explicitly (or via `FORGEC2_*` env)
when the config file is mounted read-only. Data persists in the
`forgec2_data` volume (`FORGEC2_DATA_DIR=/data`, which also anchors logs and
backups). No `.env` needed unless you want overrides (see `.env.example`).
Postgres is opt-in:
`docker compose --profile postgres up -d --build` (set `DB_PASSWORD`,
`FORGEC2_DB_DRIVER=postgres`, `FORGEC2_DB_DSN`).

> **Postgres backup caveat:** the built-in database backup/restore/VACUUM
> endpoints are SQLite-only (they use `VACUUM INTO` plus file replacement).
> On a Postgres deployment they return `501` with this explanation instead of
> pretending to work — schedule `pg_dump`/`pgBackRest` yourself.

### Backups

Encrypted snapshots (`<data_dir>/backups/*.fbk`, AES-256-GCM) run on a schedule:

```yaml
backup:
  retain: 14          # snapshots kept (0 = default 7)
  schedule: "daily"   # hourly | daily | weekly | monthly
```

Env equivalents: `FORGEC2_BACKUP_RETAIN`, `FORGEC2_BACKUP_SCHEDULE`.

Freshness is observable, so a stalled job is not discovered at restore time:

- `GET /api/v1/health` → `backup: { retain, age_seconds, last_success, last_failure, last_error }`
- Prometheus: `forgec2_backup_age_seconds`, `forgec2_backup_success_total`, `forgec2_backup_failures_total`

Alert on `forgec2_backup_age_seconds` exceeding ~2× your schedule interval.

### Production overlay

`docker-compose.prod.yml` hardens the base stack for real deployments:
`read_only` rootfs (+ tmpfs for `/tmp` and home), `mem_limit` / `pids_limit`,
`FORGEC2_JWT_SECRET` required, storage keys forwarded from `.env`, bind
`0.0.0.0` by default.

```bash
cp config.example.yaml config.yaml
cp .env.example .env            # then set FORGEC2_JWT_SECRET=$(openssl rand -hex 32)
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
curl -fsS https://127.0.0.1:8000/health
```

## Option B — Linux, bare binary

```bash
# from the GitHub Release page for your version:
curl -LO https://github.com/r0si/forgec2/releases/download/vX.Y.Z/forgec2-server-linux-amd64
chmod +x forgec2-server-linux-amd64
cp config.example.yaml config.yaml
./forgec2-server-linux-amd64 -config config.yaml
```

## Option C — Windows, bare binary

```powershell
# download forgec2-server-windows-amd64.exe from the GitHub Release page,
# rename it, then from that folder:
Copy-Item config.example.yaml config.yaml
.\forgec2-server.exe -config config.yaml
```

Open `https://<host>:8000` (self-signed cert on first run; TLS on by default).
Data (SQLite + uploads) lives under `data/` (or the `forgec2_data` volume).

The console binds **every interface** by default (`server.host: 0.0.0.0`), so
it can be operated from another host or container network. Two consequences:

- A browser on another host is refused as cross-origin unless its hostname is
  listed under `server.allowed_origins` in `config.yaml`. The server logs a
  warning at startup when the bind is a wildcard and the list is empty.
- Set `server.host: 127.0.0.1` to keep the console local. The SOCKS relay
  stays local either way (`server.socks_listen_host`).

TLS posture is configurable per termination point: extra `https`/`tls`
listeners can require verified client certificates, verify them against their
own CA, or raise their floor to TLS 1.3 — see
[`docs/LISTENER_TLS.md`](LISTENER_TLS.md) for the merge rules and the
client-certificate path for implants.

## Cut a release (maintainers)

```bash
git tag vX.Y.Z
git push origin vX.Y.Z   # release.yml builds linux/windows × amd64/arm64
```

The tag is stamped into the binary (`-version` reports it; `/health`
reports it too). The `linux/amd64` build asserts this in CI and fails the
release if the tag didn't make it in. Custom `FORGEC2_PORT` is honored by
both the server and the container healthcheck.

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `port 0.0.0.0:8000 is already in use` | Another instance is running: `taskkill /f /im forgec2-server.exe` (Windows) or `docker compose down` / `pkill forgec2-server` (Linux). Or change `server.port` in `config.yaml`. |
| `unable to open database file` (Docker) | `/data` not writable — fixed in-image (nonroot-owned seed); `docker compose down -v` on a stale root-owned volume, then `up` again. Bare binary: run from a writable directory. |
| `unhealthy` container right after `up` | Normal for ~30s (Go build + first-run migrations). `docker compose ps`; `start_period` covers it. If still unhealthy: `docker compose logs forgec2`. |
| Browser cert warning | Expected: self-signed cert generated at `data/server.crt`. Accept once, or put `cert_file`/`key_file` with a real cert in `config.yaml`. |
| Windows SmartScreen / Defender flag | Unsigned exe — click “More info → Run anyway”; consider adding a Defender exclusion for the folder on lab machines. |
| Windows Firewall prompt on first start | Allow private networks so agents can reach port 8000. |
| `AI not configured` | Set `ai.api_key` in `config.yaml` or `FORGEC2_AI_API_KEY` env. |
| Agents can't check in (Docker) | `FORGEC2_HOST` defaults to `127.0.0.1` — set it to the server's LAN IP (or `0.0.0.0` listen + agent URL pointing at you) and open the port. |
| `DB_PASSWORD ... required` | Only when using `--profile postgres`. Plain `docker compose up` (SQLite) doesn't need `.env` at all. |
