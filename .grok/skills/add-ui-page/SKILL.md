---
name: add-ui-page
description: Add a new ForgeC2 Next.js page (App Router route, layout nav, i18n, API wiring)
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Steps

### 1. Create page

**File:** `frontend/src/app/your-page/page.tsx`

```tsx
"use client";

import { apiGet } from "@/lib/api";
import { useI18n } from "@/lib/i18n";

export default function YourPage() {
  const { t } = useI18n();
  // fetch via apiGet("/your-api-path")
  return (
    <div>
      <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">
        {t("yourpage.title")}
      </h1>
    </div>
  );
}
```

### 2. Optional layout wrapper

If the page needs the sidebar, ensure it's under a route group using `AppLayout` (most pages inherit from parent `layout.tsx` in `frontend/src/app/(main)/` or similar — check existing pages).

### 3. Navigation

**File:** `frontend/src/components/Sidebar.tsx` — add to appropriate `navSections` entry:

```ts
{ href: "/your-page", labelKey: "nav.yourpage", icon: "fa-solid fa-star" },
```

### 4. i18n

**File:** `frontend/src/lib/i18n.tsx` — add keys for en, zh, ja (minimum):

```ts
"nav.yourpage": "Your Page",
"yourpage.title": "Your Page",
```

See `add-i18n` skill for full locale coverage.

### 5. Go API (if new endpoint)

If the page needs a new backend route, use `add-api-endpoint` skill. Existing pages call Go via:

```
/api/go?p=/your-path&format=json
```

Handlers should call `s.renderPageOrJSON()` — it returns JSON for the Next.js proxy.

### 6. Build & test

```powershell
cd frontend
npm run dev
# or npm run build && npx next start -p 3000
```

> All new pages are Next.js-only. Go backend no longer serves HTML templates or static assets.

## Verify

- Page loads at `http://localhost:3000/your-page`
- Sidebar highlights active nav
- API calls succeed (Network tab → `/api/go?p=...`)
- i18n keys render (not raw key names)