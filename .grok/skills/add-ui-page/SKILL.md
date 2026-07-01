---
name: add-ui-page
description: Add a new ForgeC2 web page (route, handler, template, page_assets bundle, nav, i18n). Use when adding pages, navigation items, or /add-ui-page.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Steps

### 1. Route

**File**: `internal/server/server.go`

```go
auth.GET("/your-page", s.handleYourPage)
```

### 2. Handler

```go
func (s *Server) handleYourPage(c *gin.Context) {
    data := gin.H{
        "Title": "ForgeC2 - Your Page",
        "ActiveNav": "yourpage",
        "IsFullPage": false, // true for AI-style full pages
    }
    s.renderPage(c, "your_page_content", data)
}
```

### 3. Template

**File**: `internal/server/templates/your_page.html`

```html
{{define "your_page_content"}}
<div id="your-page" class="max-w-7xl mx-auto">
  <h1>{{T .CurrentLang "yourpage.title"}}</h1>
</div>
{{end}}
```

### 4. JS bundle

**File**: `internal/server/page_assets.go` — add to `pageScriptMap`:

```go
"your_page_content": {Bundle: "ops.bundle.js", Scripts: []string{"your_page.js"}},
```

Add source to `build_js.ps1` bundle group or new bundle. Run `make bundle`.

### 5. Navigation

**File**: `templates/layout.html` — link with `ActiveNav` match.

### 6. i18n

**File**: `locales.go` — en, zh, ja, ko, ar keys. See `add-i18n` skill.

### 7. Rebuild

```bash
make build-all
```

## Verify

- Page loads, nav highlights
- JS handlers work (use `type="button"` + direct `addEventListener` for critical actions)
- Language switch works