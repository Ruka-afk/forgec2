# Contributing to ForgeC2

Thank you for your interest in contributing to ForgeC2. This document outlines the guidelines for contributing to this project.

## Prerequisites

- **Go 1.22+**
- **Node.js 20+**
- **PowerShell** (Windows) or **Bash** (Linux/macOS)
- **Docker** (optional, for containerized builds)
- **Git**

## Getting Started

1. Fork and clone the repository:

```bash
git clone https://github.com/your-username/forgec2.git
cd forgec2
```

2. Install Go dependencies:

```bash
go mod download
```

3. Install frontend dependencies:

```bash
cd frontend && npm install
```

4. Build and run:

```powershell
# Windows
powershell -File scripts/dev-backend.ps1

# Linux/macOS
go build -o forgec2-server ./cmd/server
./forgec2-server -config config.yaml
```

## Code Style

### Go

- Follow [Effective Go](https://go.dev/doc/effective-go) conventions
- Use `gofmt` / `goimports` for formatting
- Run `go vet ./...` before committing
- No comments unless requested
- Table-driven tests with subtests (`t.Run`)

### TypeScript / React

- Functional components with hooks
- shadcn/ui components for all UI elements
- `cn()` utility for class merging (no template literals)
- `t("key")` for all user-visible text (i18n)
- TypeScript strict mode, no `as any`

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new task type for credential dumping
fix: resolve WebSocket broadcast race condition
docs: update API documentation for /agents endpoint
refactor: split handlers_auth.go into smaller files
```

## Testing

### Running Tests

```bash
# Go tests
go test ./...

# Frontend type check
cd frontend && npx tsc --noEmit
```

### Writing Tests

- Use table-driven tests with `t.Run` subtests
- Place test files alongside the code they test (`foo_test.go` next to `foo.go`)
- Use `internal/testutil.SetupTestDB(t)` for database tests
- Use `internal/testutil.NewGinTestServer(t, db)` for HTTP handler tests
- Do not add third-party assertion libraries (testify, etc.)

### Test Pattern

```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    int
        wantErr bool
    }{
        {"valid", "abc", 3, false},
        {"empty", "", 0, false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := len(tt.input)
            if got != tt.want {
                t.Errorf("got %d, want %d", got, tt.want)
            }
        })
    }
}
```

## Project Structure

```
cmd/server/              — Go entrypoint
internal/
  server/                — HTTP handlers, WebSocket, AI, routing (80+ files)
  server/middleware/      — Auth, CSRF, rate limiting, security headers
  db/                    — GORM models, migrations, TTL cache
  config/                — Configuration loader and validation
  crypto/                — Encryption, signing, ECDH sessions
  payload/               — Agent payload generation (DO NOT refactor)
  plugin/                — Plugin runtime and marketplace
  scripting/             — JavaScript scripting engine
  malleable/             — C2 profile compiler and presets
  infrastructure/        — Redirector config generation
  testutil/              — Test helpers (SetupTestDB, NewGinTestServer)
frontend/
  src/app/               — Next.js App Router pages
  src/components/        — React components
  src/components/ui/     — shadcn/ui primitives
  src/lib/               — Utilities, API client, i18n, hooks
```

## Pull Request Process

1. Create a feature branch from `main`
2. Make your changes following the style guidelines above
3. Run tests: `go test ./...` and `cd frontend && npx tsc --noEmit`
4. Run `go vet ./...` and fix any warnings
5. Submit a pull request with a clear description of the changes
6. Wait for CI checks to pass and for a review

## Security

**Do not** open public issues for security vulnerabilities. See [SECURITY.md](SECURITY.md) for responsible disclosure instructions.
