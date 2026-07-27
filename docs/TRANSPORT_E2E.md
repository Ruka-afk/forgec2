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
