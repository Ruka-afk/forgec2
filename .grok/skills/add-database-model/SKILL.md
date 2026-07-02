---
name: add-database-model
description: Add GORM models and migrations to ForgeC2 SQLite database. Use for new DB tables, schema changes, or /add-database-model.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add a new persisted entity or column to the ForgeC2 database.

## Key files

| File | Role |
|------|------|
| `internal/db/models.go` | GORM struct definitions |
| `internal/db/db.go` | `AutoMigrate` list |
| `config.yaml` | `database.path` (default `data/db/forgec2.db`) |

## Add model checklist

1. Define struct with GORM tags:

```go
type YourModel struct {
    ID        string    `gorm:"primaryKey"`
    CreatedAt time.Time
    // fields...
}
```

2. Add to `AutoMigrate()` in `db.go`:

```go
db.AutoMigrate(&db.YourModel{}, /* existing... */)
```

3. Create handler queries with `s.db.Create` / `Find` / `Where`.
4. For breaking changes: prefer additive columns; document manual migration if needed.

## Conventions

- Primary keys: UUID string or `uint` for users
- Soft delete: use `gorm.DeletedAt` if required
- JSON columns: `datatypes.JSON` or `string` with marshal in handler
- Indexes: `gorm:"index"` on foreign keys and search fields

## Verify

```bash
go test ./internal/db/...
go build ./cmd/server/
```

- Fresh start: tables created automatically
- Existing DB: new columns appear after restart (AutoMigrate)
- CRUD from UI/API works without SQL errors