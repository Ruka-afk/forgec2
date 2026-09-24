---
name: add-i18n
description: Add internationalization keys to ForgeC2 following the locales.go / i18n en+zh pattern
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: i18n
---

## When to use

Replace hardcoded UI strings or add translations for a new page/feature.

## Supported languages

Two, on both the server and the frontend.

| Code | Server (`internal/server/locales.go`) | Frontend (`frontend/src/lib/i18n/`) |
|------|--------------------------------------|--------------------------------------|
| `en` | `enTranslations` | `en.ts` |
| `zh` | `zhTranslations` | `zh.ts` |

`SupportedLanguages` and `translations` wire these together on the server;
`check:i18n` fails if a `t()` key is missing from either frontend block.

## Add a new key

Add the same key to **both** maps:

```go
// enTranslations
"myfeature.title": "My Feature",

// zhTranslations
"myfeature.title": "我的功能",
```

## Usage patterns

### Frontend (`frontend/src/lib/i18n/index.tsx`, keys in `en.ts` + `zh.ts`)

```tsx
const { t } = useI18n();
t("myfeature.title")
```

### Go handlers

Keys are injected via `LocaleJSON` in `addUserToData`.

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