# ForgeC2 Component Library

> Auto-generated from `frontend/src/components/`

## Core Components (`UI.tsx`)

### StatusBadge
- **Props**: `status: string`, `pulse?: boolean`
- **Statuses**: online (green), offline (grey), stale (amber), locked (red), completed (green), failed (red), pending (grey), running (blue)
- **Structure**: `[dot] [label]` — inline flex, rounded-full, text-xs
- **Usage**: Agent status, task status, listener status

### PageHeader
- **Props**: `title: ReactNode`, `subtitle?: string`, `children?: ReactNode`
- **Structure**: Flex row (title left, actions right), responsive to column on mobile
- **CSS**: `.page-title` (xl, semibold), `.page-subtitle` (sm, secondary text)
- **Usage**: Every page top section

### SearchInput
- **Props**: `value: string`, `onChange: fn`, `placeholder?: string`, `className?: string`
- **Structure**: Relative container with magnifying glass icon + input
- **Usage**: Filter/search bars in list pages

### TableCard
- **Props**: `header?: ReactNode`, `children: ReactNode`, `responsive?: boolean`
- **Structure**: `.ui-card` wrapper, optional header bar with border-bottom
- **Usage**: Data tables, log views

### Pagination
- **Props**: `page: number`, `pageSize: number`, `total: number`, `onPageChange: fn`
- **Structure**: Bottom bar with range text, prev/next chevron buttons
- **Usage**: Paginated lists (agents, tasks, users, tokens, etc.)

### ConfirmModal
- **Props**: `open: boolean`, `title: string`, `message: string`, `confirmText?: string`, `cancelText?: string`, `danger?: boolean`, `onConfirm: fn`, `onCancel: fn`
- **Structure**: Fixed overlay (black/40 + backdrop-blur), centered card, cancel + confirm buttons
- **Variants**: Default (indigo confirm), Danger (red confirm)
- **Usage**: Delete confirmations, dangerous action confirmations

## Layout Components

### Sidebar (`Sidebar.tsx`)
- **Sections**: Operations, Build & Deploy, Post-Exploitation, Intel & Analysis, System
- **States**: Expanded (224px), Collapsed (64px, icons only)
- **Features**: Active page highlight (`.nav-active`), collapsible sections, agent/listener count badges, keyboard shortcut (Ctrl+B)
- **Navigation items**: 25+ routes across 5 sections

### TopBar (`TopBar.tsx`)
- **Features**: Global search, notification bell with badge, theme toggle (light/dark/system), locale switcher, user menu with logout
- **Notification types**: info (blue), warning (amber), error (red), success (green)
- **Search**: Global quick-navigate with keyboard shortcut (Ctrl+K)

### AppLayout (`AppLayout.tsx`)
- **Structure**: Sidebar (left) + main content area + TopBar (top)
- **Responsive**: Sidebar auto-collapses on mobile, hamburger menu toggle

## Feature Components

### ChartCard (`ChartCard.tsx`)
- **Props**: `title`, `icon`, `iconColor`, `children`, `onRefresh?`, `loading?`, `error?`, `exportFilename?`
- **States**: Loaded, Loading (spinning refresh icon), Error (warning icon + retry button)
- **Features**: PNG export, refresh button, error state with retry
- **Usage**: Dashboard stat cards, chart containers

### ShellTerminal (`ShellTerminal.tsx`)
- **Dependency**: xterm.js (loaded via CDN)
- **Features**: WebSocket-based terminal, resize handling, dark theme

### DropdownMenu (`DropdownMenu.tsx`)
- **Sub-components**: `DropdownItem`, `DropdownDivider`, `DropdownHeader`
- **Features**: Click-outside-to-close, keyboard navigation, nested submenus
- **Usage**: User menu, action dropdowns, context menus

### ErrorBoundary (`ErrorBoundary.tsx`)
- **Features**: Catches React rendering errors, shows fallback UI with retry button, logs error details
- **Usage**: Wraps page content in main layout

### UpdateBanner (`UpdateBanner.tsx`)
- **Features**: Checks `/api/update` for new version, shows dismissible banner with changelog link
- **States**: Hidden, Visible (new version available)

### ShortcutsHelp (`ShortcutsHelp.tsx`)
- **Keyboard shortcuts**: Ctrl+K (search), Ctrl+B (sidebar), Escape (close modal), G then D/G A/G T (quick nav)

## Component Conventions

- All components use CSS variables (`var(--xxx)`) for theming
- Dark mode via `.dark` class on `<html>`
- Icons via FontAwesome 6.5.1 CDN (`fa-solid`, `fa-regular`)
- Animations: `animate-fade-in` (0.2s), `animate-slide-in` (0.2s), `animate-pulse-dot`
- Spacing uses CSS variable scale: `var(--space-1)` through `var(--space-12)`
- Font: Inter (body) / JetBrains Mono (code)
