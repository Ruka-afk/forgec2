---
name: report-section
description: Add sections to ForgeC2 operation reports (HTML + Markdown in generator.go). Use for report chapters or /report-section.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Files

- `internal/report/generator.go` — `ReportData` struct, HTML/Markdown templates
- `internal/server/handlers_report.go` — data collection

## Steps

### 1. Data struct

```go
type YourSectionItem struct {
    Field1 string
    Field2 int
}

type ReportData struct {
    // ...
    YourSection []YourSectionItem
}
```

### 2. Collect data

In report handler, query DB and populate `data.YourSection`.

### 3. HTML template (embedded string in generator.go)

```html
{{if .YourSection}}
<div class="section">
  <h2>Your Section</h2>
  {{range .YourSection}}...{{end}}
</div>
{{end}}
```

### 4. Markdown output

```go
if len(data.YourSection) > 0 {
    md += "\n## Your Section\n\n"
    for _, item := range data.YourSection { ... }
}
```

## Existing sections

Overview, Agents, Tasks, Credentials, Screenshots, Vulnerabilities, IOC.

## Verify

- Empty section hidden (`{{if .YourSection}}`)
- HTML and Markdown both render
- `go build ./internal/report/...`