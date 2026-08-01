## Objective
Comprehensively fix all security, error handling, concurrency, and code quality issues identified in the ForgeC2 codebase review.

## Important Details
- Codebase at `C:\Users\18354\Downloads\C2\forgec2`
- All changes must pass `go build ./...` and `go test ./internal/...`
- Follow existing code patterns in the repo (respondError, gin handlers, etc.)
- Fixes applied incrementally by severity: Critical → High → Medium → Low
- ~60 issues identified across 6 phases; ~18 fixed so far

## Work State
### Completed
- **CRITICAL**: REST API endpoints missing authorization — added `requireOperator(c)` to `apiListAgents`, `apiGetAgent`, `apiListTasks`, `apiGetTask`, `apiListCredentials`, `apiListListeners`, `apiDashboardStats` in `api.go`
- **CRITICAL**: `apiListAuditLogs` — added `requireAdmin(c)` check in `api.go`
- **HIGH**: `handleChangePassword` — added `s.revokeAllUserSessions(user.ID)` after password change in `handlers_settings.go`
- **HIGH**: `pwdChangeTimes` map memory leak — added periodic cleanup (entries older than 10min) in the existing dedup cache goroutine in `server.go`
- **HIGH**: `RateLimiter.visitors` unbounded growth — added `maxVisitors` field with default 100000 and single-entry eviction when at capacity in `middleware/middleware.go`
- **HIGH**: Audit batch insert atomicity — wrapped `CreateInBatches` in `Transaction` block in `audit.go`; added `gorm` import
- **HIGH**: GeoIP per-beacon unnecessary SELECT — removed `First(&agent)` read, replaced with single conditional `WHERE id = ? AND (country != ? OR city != ?)` UPDATE in `handlers_beacon.go`
- **HIGH**: Task result per-row full Save — changed `s.db.Save(task)` to `s.db.Model(task).Updates(...)` with specific columns in `handlers_beacon.go`
- **HIGH**: Screen control task Save-before-Delete waste — removed the intermediate `Save` in `handlers_beacon.go`
- **MEDIUM**: `handleListListeners` missing error response — added `respondError` on DB failure in `handlers_listeners.go`
- **MEDIUM**: `handleListenerDetail` nil slice — initialized `agents` as `make([]db.Implant, 0)` in `handlers_listeners.go`
- **MEDIUM**: Internal error details leaked in backup restore — replaced `fmt.Sprintf(...)` with generic message in two places in `handlers_backup.go`
- **MEDIUM**: Listener port/host validation — added port range (1–65535) and `isValidHost` check in `handleCreateListener` in `handlers_listeners.go`; added `isValidHost` helper in `helpers.go`
- **MEDIUM**: Listener edit port/host validation — added port range (1–65535) and host validation in `handleUpdateListener` in `handlers_listeners.go`
- **MEDIUM**: Purge days upper bound — added max cap of 365 days for task purge, 730 days for audit log purge in `handlers_settings.go`
- **MEDIUM**: Command length limit — added `MaxCommandLength` (10k chars) to constants, validated in `createTask` (central) and `handleSendCommand` (early exit) in `server.go`/`handlers_commands.go`
- **MEDIUM**: Agent notes/tags length limit — added `MaxNotesLength` (5k chars) validation for both JSON and form inputs in `handlers_agents.go`
- **MEDIUM**: `StopAllListeners` SOCKS race — added `socksMu` mutex, nil-check under lock in `subsystems.go`; added nil guard in `cleanupStaleSocks` goroutine in `handlers_socks_relay.go`
- **PREVIOUS-FIXES** (carried forward): WebSocket session revocation check, backup code TOCTOU atomic update, handleSetUserPassword session revocation, user toggle status message, first-login password complexity, login lockout recordLoginFailure return value

### Active
- Remaining MEDIUM items (M5: shellcode command validation, M6: other input limits)
- LOW items L1–L10 (minor code quality, style, unused checks)
- Build and test validation — **PASSES** (`go build ./...` + `go test ./internal/...` all clean)

### Blocked
- (none)

## Next Move
1. Fix remaining MEDIUM items (shellcode handler command validation, any other input limits)
2. Fix LOW items (L1–L10: minor code quality issues)
3. Run `go build ./... && go test ./internal/...` after each batch

## Relevant Files
- `api.go`: REST API endpoint authorization + audit log admin check
- `handlers_settings.go`: password change session revocation, purge days upper bounds
- `server.go`: pwdChangeTimes periodic cleanup in dedup goroutine, central command length validation in createTask
- `middleware/middleware.go`: RateLimiter maxVisitors eviction
- `audit.go`: audit batch insert wrapped in transaction (added `gorm` import)
- `handlers_beacon.go`: GeoIP conditional UPDATE, task result column-specific Updates, removed Save-before-Delete
- `handlers_listeners.go`: error propagation, port/host validation (create + update)
- `handlers_backup.go`: internal error sanitization for restore verification
- `helpers.go`: added `isValidHost` helper
- `handlers_agents.go`: notes/tags max length validation
- `handlers_commands.go`: early-exit command length check
- `handlers_socks_relay.go`: nil guard in cleanupStaleSocks goroutine
- `subsystems.go`: socksMu for safe StopAllListeners
- `constants.go`: added MaxCommandLength, MaxNotesLength
