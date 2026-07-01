---
name: add-api-endpoint
description: Add REST API endpoints to ForgeC2 (route, handler, auth, OpenAPI sync). Use when adding API routes or updating api/openapi.yaml.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Steps

### 1. Register route

**File**: `internal/server/server.go` — inside `setupAuthGroup()`:

```go
auth.GET("/api/your-resource", s.handleYourAPI)
auth.POST("/api/your-resource", s.handleYourAPICreate)
```

Use `middleware.AuthRequired` + role checks inside handler.

### 2. Handler

**File**: `internal/server/handlers_*.go`

```go
func (s *Server) handleYourAPI(c *gin.Context) {
    role, _ := c.Get("user_role")
    if role == "viewer" { c.JSON(403, gin.H{"error": "forbidden"}); return }
    // ...
    c.JSON(200, gin.H{"success": true, "data": result})
}
```

### 3. OpenAPI

**File**: `api/openapi.yaml` — add path, tags, request/response schemas.

Served at `/api/docs/openapi.yaml`.

### 4. Audit (mutations)

```go
s.logAction(c, "your_action", "resource", id, detail)
```

## Auth

- Session cookie: `forgec2_session` from `POST /login`
- Roles: `admin`, `operator`, `viewer`, `guest`
- Rate limit: `middleware/rate_limit.go` (WS and beacon exempt)

## Verify

```bash
curl -c cookies.txt -X POST http://localhost:8080/login -d "username=admin&password=admin"
curl -b cookies.txt http://localhost:8080/api/your-resource
go test ./internal/server/...
```