---
name: add-ui-page
description: Add a new ForgeC2 page (Vite + React Router route, layout nav, i18n, API wiring)
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Steps

### 1. Create page

**File:** `frontend/src/pages/your-page/YourPage.tsx`

```tsx
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";

export default function YourPage() {
  const { t } = useI18n();
  // fetch via api.get(paths.your.endpoint)
  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">
        {t("yourpage.title")}
      </h1>
    </div>
  );
}
```

Then register it in `frontend/src/router.tsx` (pages are lazy-loaded there)
and add the path to `paths` in `frontend/src/lib/api-paths.ts`.

### 2. Layout wrapper

The app shell is global: `AppLayout` wraps the router outlet, so every page
already has the sidebar and top bar. No per-page layout file is needed.

### 3. Navigation

**File:** `frontend/src/components/Sidebar.tsx` — add to appropriate `navSections` entry:

```ts
{ href: "/your-page", labelKey: "nav.yourpage", icon: "fa-solid fa-star" },
```

### 4. i18n

**File:** `frontend/src/lib/i18n/en.ts` and `frontend/src/lib/i18n/zh.ts` — add
the key to BOTH locale blocks (en and zh; `check:i18n` fails otherwise):

```ts
"nav.yourpage": "Your Page",
"yourpage.title": "Your Page",
```

### 5. Go API (if new endpoint)

Pages call Go directly on the same origin through the typed client — there is no
Next.js-style proxy:

```ts
await api.get(paths.your.endpoint);
await api.postJson(paths.your.action, { ... });
```

`check:api-paths` rejects bare string paths, so add every new path to
`frontend/src/lib/api-paths.ts`. Handlers answer a `{success, data}` envelope;
`api.ts` unwraps `data` for you.

### 6. Build & test

```powershell
cd frontend
npm run dev      # Vite dev server, /api proxied to the Go server on :8000
npm run verify   # typecheck + lint + vitest + all check:* gates
```

> This is a Vite SPA, not Next.js. The Go server embeds the production build
> (`internal/webdist/dist`); sync it with `scripts/build-embedded.ps1` before
> pushing, or the `check:webdist` pre-push hook fails.

## Verify

- Page loads in the running app and the sidebar highlights the active nav item
- API calls succeed (Network tab → `/api/...`)
- i18n keys render in both locales (not raw key names)
- `npm run verify` is green