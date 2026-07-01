---
name: add-i18n
description: Add internationalization keys to ForgeC2 following locales.go en/zh/ja/ko/ar pattern
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: i18n
---

## When to use

Replace hardcoded UI strings or add translations for a new page/feature.

## Supported languages

Defined in `internal/server/locales.go`:

| Code | Map variable |
|------|--------------|
| `en` | `enTranslations` |
| `zh` | `zhTranslations` |
| `ja` | `jaTranslations` |
| `ko` | `koTranslations` |
| `ar` | `arTranslations` |

`SupportedLanguages` and `translations` map wire these together. `ar` is RTL.

## Add a new key

Add the same key to **all five** `*Translations` maps:

```go
// enTranslations
"myfeature.title": "My Feature",

// zhTranslations
"myfeature.title": "我的功能",

// jaTranslations
"myfeature.title": "マイ機能",

// koTranslations
"myfeature.title": "내 기능",

// arTranslations
"myfeature.title": "ميزتي",
```

## Usage patterns

### Go templates (`internal/server/templates/*.html`)

```html
{{tr .Lang "myfeature.title"}}
{{printf (tr .Lang "myfeature.count") .Count}}
```

### JavaScript (`internal/server/templates/static/js/*.js`)

```js
__t("myfeature.title")
__tf("js.task_completed", taskId)
```

Keys are injected via `LocaleJSON` in `renderPage`.

### Go handlers

```go
c.JSON(200, gin.H{"message": T(c, "common.success")})
```

## Naming conventions

- `nav.*` — sidebar navigation
- `common.*` — shared labels (save, cancel, …)
- `js.*` — client-side toasts and dynamic text
- `<page>.*` — page-specific keys (`tasks.title`, `agents.filter`, …)

## Verify

```bash
go run ./cmd/i18n-tool check --lang zh
go run ./cmd/i18n-tool check --lang ja
go build ./internal/server/...
```

- Switch language in UI settings; all five locales render without showing raw keys
- RTL layout correct for `ar`