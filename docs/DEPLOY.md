# ForgeC2 Deploy (simplest path)

Prebuilt `forgec2-server` binaries for Linux and Windows are published on every
`v*` tag via `.github/workflows/release.yml` (frontend already embedded —
single file, no Node.js needed at runtime).

## Option A — Linux, Docker (recommended)

```bash
cp .env.example .env        # set DB_PASSWORD inside
cp config.example.yaml config.yaml
docker compose up -d --build
curl -k https://127.0.0.1:8000/health   # {"status":"ok",...}
```

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
