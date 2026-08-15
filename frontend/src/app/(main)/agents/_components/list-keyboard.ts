import type { FlatBeaconRow } from "./groupBeaconsByHost";

export const HOST_EXPAND_KEY = "forgec2_host_expand";

export type ListKeyAction =
  | { type: "move"; delta: 1 | -1 }
  | { type: "interact" }
  | { type: "toggleSelect" }
  | { type: "toggleExpand" }
  | { type: "focusSearch" }
  | { type: "dismissMenu" };

export function listKeyAction(key: string, hasMenu = false): ListKeyAction | null {
  if (key === "Escape" && hasMenu) return { type: "dismissMenu" };
  if (key === "j") return { type: "move", delta: 1 };
  if (key === "k") return { type: "move", delta: -1 };
  if (key === "Enter") return { type: "interact" };
  if (key === "x") return { type: "toggleSelect" };
  if (key === " ") return { type: "toggleExpand" };
  if (key === "/") return { type: "focusSearch" };
  return null;
}

export function hostKeyForFocus(rows: FlatBeaconRow[], focusedId: string | null): string | null {
  if (!focusedId) return null;
  for (const row of rows) {
    if (row.kind === "host" && (row.group.sessions[0]?.id || "") === focusedId) return row.group.key;
  }
  return null;
}

export function readExpandedHosts(storage?: Pick<Storage, "getItem"> | null): Set<string> {
  try {
    const raw = (storage ?? (typeof localStorage !== "undefined" ? localStorage : null))?.getItem(HOST_EXPAND_KEY);
    if (!raw) return new Set();
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return new Set();
    return new Set(parsed.filter((k): k is string => typeof k === "string" && k.length > 0));
  } catch {
    return new Set();
  }
}

export function writeExpandedHosts(keys: Set<string>, storage?: Pick<Storage, "setItem"> | null): void {
  try {
    (storage ?? (typeof localStorage !== "undefined" ? localStorage : null))?.setItem(
      HOST_EXPAND_KEY,
      JSON.stringify([...keys]),
    );
  } catch {
    /* private mode */
  }
}
