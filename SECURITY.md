# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 2.4.x   | Yes       |
| 2.3.x   | Yes       |
| < 2.3   | No        |

## Reporting a Vulnerability

If you discover a security vulnerability in ForgeC2, please report it responsibly.

**Do not** open public GitHub issues for security vulnerabilities.

Instead, email security details to: **security@forgec2.dev** (or the maintainer's private contact).

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response timeline

- **Acknowledgment**: within 48 hours
- **Initial assessment**: within 1 week
- **Fix or mitigation**: depends on severity, typically 2-4 weeks

## Security Features

ForgeC2 includes the following built-in security measures:

### Authentication & Authorization

- JWT + bcrypt password hashing
- HttpOnly secure session cookies with `SameSite=Lax`
- CSRF double-submit cookie protection (`forgec2_csrf` + `X-CSRF-Token` header)
- TOTP two-factor authentication with backup codes
- Per-route RBAC permission system (agents, listeners, settings, plugins, etc.)
- IP-based login lockout with progressive delay

### Transport Security

- TLS enforcement option (`require_tls_for_auth`)
- Configurable CORS origin allowlist (`allowed_origins`)
- Cookie domain restriction (`cookie_domain`)
- CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy headers

### Data Protection

- AES-GCM encrypted automatic database backups
- ECDH session key exchange for agent communications
- Loot encryption at rest
- Path traversal prevention on all file operations
- Request body size limits (2MB max JSON)

### Operational Security

- OPSEC guard pre-flight rule engine
- Circuit breaker for listener health monitoring
- Audit logging for all sensitive operations
- Auto-generated random admin password and JWT secret on first run
- Login lockout with cleanup goroutine

## Hardening Checklist

- Use a reverse proxy (Nginx/Caddy) to terminate TLS in production
- Set `allowed_origins` to restrict WebSocket/CORS access
- Enable TOTP 2FA for all users
- Rotate JWT secret via `/api/settings/jwt/regenerate`
- Review audit logs regularly (`AuditLog` table)
- Use `VACUUM` and DB backups via Settings UI
- Set `require_tls_for_auth: true` in production

## Known Limitations

- Single-user admin panel (no multi-tenant isolation)
- SQLite database (not suitable for high-concurrency production without careful tuning)
- No built-in WAF or DDoS protection (use external reverse proxy)
