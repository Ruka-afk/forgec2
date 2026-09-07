# ForgeC2 Deploy (simplest path)

Prebuilt `forgec2-server` binaries for Linux and Windows are published on every
`v*` tag via `.github/workflows/release.yml` (frontend already embedded —
single file, no Node.js needed at runtime).

## Option A — Linux, Docker (recommended, SQLite, zero-config)

```bash
cp config.example.yaml config.yaml
docker compose up -d --build
curl -k https://127.0.0.1:8000/health   # {"status":"ok",...}
```

Secrets (JWT, beacon key, TLS cert) auto-generate on first run. Data persists
in the `forgec2_data` volume. No `.env` needed unless you want overrides
(see `.env.example`). Postgres is opt-in:
`docker compose --profile postgres up -d --build` (set `DB_PASSWORD`,
`FORGEC2_DB_DRIVER=postgres`, `FORGEC2_DB_DSN`).

## Option B — Linux, bare binary

```bash
# from the GitHub Release page for your version:
curl -LO https://github.com/Ruka-afk/forgec2/releases/download/vX.Y.Z/forgec2-server-linux-amd64
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
