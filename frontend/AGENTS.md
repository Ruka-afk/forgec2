<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.
<!-- END:nextjs-agent-rules -->

# ForgeC2 Frontend Architecture

## Tailwind CSS — PostCSS (shadcn/ui)

This project uses **Tailwind CSS via PostCSS** with **shadcn/ui** (base-nova style, `@base-ui/react` primitives).

- `postcss.config.mjs` uses `@tailwindcss/postcss` plugin
- `globals.css` uses `@import "tailwindcss"` + `@import "shadcn/tailwind.css"` + `@import "tw-animate-css"`
- `components.json` defines shadcn config: aliases `@/components/ui` for components, `@/lib/utils` for `cn()`
- Utility classes are compiled at build time by PostCSS

## CSS Architecture

- `globals.css` contains: shadcn imports, CSS custom properties (`:root`/`.dark` for both shadcn oklch vars and legacy hex vars), custom component classes (`.nav-active`, `.page-title`, etc.), and animations
- shadcn/ui components live in `src/components/ui/` (button, card, input, select, dialog, badge, etc.)
- All components use shadcn/ui primitives and Tailwind CSS utility classes — no legacy custom CSS classes remain
- Font Awesome 6.5.1 via CDN, Google Fonts (Inter + JetBrains Mono) via CDN
- Dark mode via `.dark` class on `<html>`, toggled by theme provider

## Utility Helpers

- `cn()` from `@/lib/utils` — shadcn's class name merging helper (clsx + tailwind-merge)
- `formatTime()`, `timeAgo()`, `debounce()` also exported from `@/lib/utils`

## Reference Repository

No external reference repository is used for layout or styling. Styling is based entirely on the shadcn design system and Tailwind CSS utility classes.

## Forbidden Patterns

- Using CDN-based Tailwind (`public/js/tailwind.min.js`) — we migrated to PostCSS
- `@tailwind base;` / `@tailwind components;` / `@tailwind utilities;` in any CSS file
- Using Next.js built-in font optimization (`next/font`) — we use Google Fonts CDN links instead
