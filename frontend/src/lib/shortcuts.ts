export interface ShortcutDef {
  key: string;
  ctrl?: boolean;
  shift?: boolean;
  alt?: boolean;
  label: string;
  description: string;
}

export const DEFAULT_SHORTCUTS: Record<string, ShortcutDef> = {
  new_item: { key: "n", ctrl: true, label: "New", description: "Create new item on current page" },
  save: { key: "s", ctrl: true, label: "Save", description: "Save current content" },
  show_shortcuts: { key: "/", ctrl: true, label: "Shortcuts", description: "Show keyboard shortcuts" },
  close_modal: { key: "Escape", label: "Close", description: "Close modal or dropdown" },
  refresh: { key: "F5", label: "Refresh", description: "Refresh current page" },
  toggle_lock: { key: "l", ctrl: true, shift: true, label: "Lock Agent", description: "Lock or unlock current agent" },
  global_search: { key: "k", ctrl: true, label: "Search", description: "Focus global search" },
};

const STORAGE_KEY = "forgec2_shortcuts";

export function loadShortcuts(): Record<string, ShortcutDef> {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      const custom = JSON.parse(saved) as Record<string, Partial<ShortcutDef>>;
      const result: Record<string, ShortcutDef> = {};
      Object.keys(DEFAULT_SHORTCUTS).forEach((key) => {
        result[key] = { ...DEFAULT_SHORTCUTS[key], ...(custom[key] || {}) };
      });
      return result;
    }
  } catch {
    // corrupt localStorage — fall back to defaults silently
  }
  return { ...DEFAULT_SHORTCUTS };
}

export function formatShortcut(s: ShortcutDef, isMac = false): string {
  const parts: string[] = [];
  if (s.ctrl) parts.push(isMac ? "⌘" : "Ctrl");
  if (s.shift) parts.push(isMac ? "⇧" : "Shift");
  if (s.alt) parts.push(isMac ? "⌥" : "Alt");
  parts.push(s.key.length === 1 ? s.key.toUpperCase() : s.key);
  return parts.join(isMac ? "" : "+");
}

export function matchShortcut(e: KeyboardEvent, s: ShortcutDef): boolean {
  const key = s.key.length === 1 ? s.key.toLowerCase() : s.key;
  const eventKey = e.key.length === 1 ? e.key.toLowerCase() : e.key;
  return (
    eventKey === key &&
    !!s.ctrl === (e.ctrlKey || e.metaKey) &&
    !!s.shift === e.shiftKey &&
    !!s.alt === e.altKey
  );
}

export const DASHBOARD_RANGE_KEY = "forgec2_dashboard_range";